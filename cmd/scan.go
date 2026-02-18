package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tomohiro-owada/doc-tracer/internal/db"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var scanCmd = &cobra.Command{
	Use:   "scan [path]",
	Short: "コードベースとドキュメントをスキャン",
	Long: `指定されたパスをスキャンして、ノードとエッジを構築します。

例:
  doc-tracer scan .                              # カレントディレクトリをスキャン
  doc-tracer scan ./docs --docs-only             # ドキュメントのみスキャン
  doc-tracer scan ./src --code-only              # コードのみスキャン
  doc-tracer scan . --include-node-modules       # node_modules も含めてスキャン`,
	Args: cobra.MaximumNArgs(1),
	RunE: runScan,
}

var (
	docsOnly           bool
	codeOnly           bool
	includeNodeModules bool
)

func init() {
	scanCmd.Flags().BoolVar(&docsOnly, "docs-only", false, "ドキュメントのみスキャン")
	scanCmd.Flags().BoolVar(&codeOnly, "code-only", false, "コードのみスキャン")
	scanCmd.Flags().BoolVar(&includeNodeModules, "include-node-modules", false, "node_modules ディレクトリも含めてスキャン")
	rootCmd.AddCommand(scanCmd)
}

func runScan(cmd *cobra.Command, args []string) error {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("DB接続エラー: %w", err)
	}
	defer database.Close()

	var docCount, codeCount int

	if !codeOnly {
		docCount, err = scanDocs(database, path)
		if err != nil {
			return fmt.Errorf("ドキュメントスキャンエラー: %w", err)
		}
	}

	if !docsOnly {
		codeCount, err = scanCode(database, path)
		if err != nil {
			return fmt.Errorf("コードスキャンエラー: %w", err)
		}
	}

	fmt.Printf("スキャン完了: ドキュメント %d件, コード要素 %d件\n", docCount, codeCount)
	return nil
}

// TraceInfo はfront matterのtrace情報
type TraceInfo struct {
	ID         string   `yaml:"id"`
	Implements []string `yaml:"implements"`
	Uses       []string `yaml:"uses"`
	Parent     []string `yaml:"parent"`
}

// FrontMatter はMarkdownのfront matter
type FrontMatter struct {
	Trace TraceInfo `yaml:"trace"`
}

func scanDocs(database *db.DB, rootPath string) (int, error) {
	count := 0

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// docsディレクトリ内の.mdファイルのみ
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		// docs/ ディレクトリ内のみ対象
		if !strings.Contains(path, "/docs/") && !strings.HasPrefix(path, "docs/") {
			return nil
		}

		// アーカイブと作業ログは除外
		if strings.Contains(path, "99_アーカイブ") || strings.Contains(path, "90_作業ログ") {
			return nil
		}

		hash, err := fileHash(path)
		if err != nil {
			return err
		}

		// ファイル内容を読み込み
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// パスを正規化（docs/ からの相対パスに変換）
		normalizedPath := path
		if idx := strings.Index(path, "/docs/"); idx != -1 {
			normalizedPath = path[idx+1:] // "/docs/" の "/" を除く
		} else if strings.HasPrefix(path, "docs/") {
			normalizedPath = path
		}

		// ドキュメントノードを作成
		docID := fmt.Sprintf("doc:%s", normalizedPath)
		node := &db.Node{
			ID:       docID,
			Type:     "document",
			Name:     filepath.Base(path),
			FilePath: path,
			FileHash: hash,
		}

		if err := database.UpsertNode(node); err != nil {
			return err
		}

		// front matterを解析
		fm, err := parseFrontMatter(string(content))
		if err == nil && fm != nil {
			// implements関係を作成
			for _, impl := range fm.Trace.Implements {
				edge := &db.Edge{
					FromID:    docID,
					ToID:      impl,
					Relation:  "implements",
					Source:    "manual",
					DefinedIn: path,
				}
				database.UpsertEdge(edge)
			}

			// uses関係を作成
			for _, use := range fm.Trace.Uses {
				edge := &db.Edge{
					FromID:    docID,
					ToID:      use,
					Relation:  "uses",
					Source:    "manual",
					DefinedIn: path,
				}
				database.UpsertEdge(edge)
			}

			// parent関係を作成
			for _, parent := range fm.Trace.Parent {
				edge := &db.Edge{
					FromID:    parent,
					ToID:      docID,
					Relation:  "contains",
					Source:    "manual",
					DefinedIn: path,
				}
				database.UpsertEdge(edge)
			}
		}

		count++
		return nil
	})

	return count, err
}

func scanCode(database *db.DB, rootPath string) (int, error) {
	count := 0

	// スキャン対象の拡張子
	codeExts := map[string]bool{
		".ts": true, ".tsx": true, ".vue": true, ".js": true,
		".php": true, ".go": true,
		".py": true,                         // Python
		".java": true,                       // Java
		".rs": true,                         // Rust
		".rb": true,                         // Ruby
		".cs": true,                         // C#
		".kt": true, ".kts": true,           // Kotlin
		".swift": true,                      // Swift
		".dart": true,                       // Dart/Flutter
	}

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			// 除外ディレクトリ
			name := info.Name()

			// 常に除外するディレクトリ
			if name == ".git" {
				return filepath.SkipDir
			}

			// --include-node-modules フラグが指定されていない場合のみ除外
			if !includeNodeModules {
				if name == "node_modules" || name == "vendor" || name == "dist" || name == "build" {
					return filepath.SkipDir
				}
			}

			return nil
		}

		ext := filepath.Ext(path)
		if !codeExts[ext] {
			return nil
		}

		hash, err := fileHash(path)
		if err != nil {
			return err
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// ファイルタイプに応じて解析
		switch ext {
		case ".vue":
			c, _ := parseVueFile(database, path, hash, string(content))
			count += c
		case ".ts", ".tsx", ".js":
			c, _ := parseTypeScriptFile(database, path, hash, string(content))
			count += c
		case ".php":
			c, _ := parsePhpFile(database, path, hash, string(content))
			count += c
		case ".go":
			c, _ := parseGoFile(database, path, hash, string(content))
			count += c
		case ".py":
			c, _ := parsePythonFile(database, path, hash, string(content))
			count += c
		case ".java":
			c, _ := parseJavaFile(database, path, hash, string(content))
			count += c
		case ".rs":
			c, _ := parseRustFile(database, path, hash, string(content))
			count += c
		case ".rb":
			c, _ := parseRubyFile(database, path, hash, string(content))
			count += c
		case ".cs":
			c, _ := parseCSharpFile(database, path, hash, string(content))
			count += c
		case ".kt", ".kts":
			c, _ := parseKotlinFile(database, path, hash, string(content))
			count += c
		case ".swift":
			c, _ := parseSwiftFile(database, path, hash, string(content))
			count += c
		case ".dart":
			c, _ := parseDartFile(database, path, hash, string(content))
			count += c
		}

		return nil
	})

	return count, err
}

func parseFrontMatter(content string) (*FrontMatter, error) {
	// front matterを抽出
	if !strings.HasPrefix(content, "---") {
		return nil, nil
	}

	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, nil
	}

	var fm FrontMatter
	if err := yaml.Unmarshal([]byte(parts[1]), &fm); err != nil {
		return nil, err
	}

	return &fm, nil
}

func parseVueFile(database *db.DB, path, hash, content string) (int, error) {
	count := 0

	// コンポーネント名を取得（ファイル名から）
	name := strings.TrimSuffix(filepath.Base(path), ".vue")
	nodeID := fmt.Sprintf("component:%s", name)

	node := &db.Node{
		ID:       nodeID,
		Type:     "component",
		Name:     name,
		FilePath: path,
		FileHash: hash,
	}

	if err := database.UpsertNode(node); err != nil {
		return count, err
	}
	count++

	// import文を解析してインポートされたコンポーネントを追跡
	importRe := regexp.MustCompile(`import\s+(?:\{[^}]+\}|[^{}\s]+)\s+from\s+['"]([^'"]+)['"]`)
	matches := importRe.FindAllStringSubmatch(content, -1)
	importedComponents := make(map[string]bool) // インポートされたコンポーネント名を追跡

	for _, match := range matches {
		importPath := match[1]
		// 相対パスのコンポーネントへの参照を検出（親子関係）
		if strings.HasSuffix(importPath, ".vue") || strings.Contains(importPath, "components/") {
			targetName := filepath.Base(importPath)
			targetName = strings.TrimSuffix(targetName, ".vue")
			importedComponents[targetName] = true // インポート済みとして記録
			edge := &db.Edge{
				FromID:    nodeID,
				ToID:      fmt.Sprintf("component:%s", targetName),
				Relation:  "contains",
				Source:    "auto",
				DefinedIn: path,
			}
			database.UpsertEdge(edge)
		}
	}

	// テンプレート内のコンポーネント使用を検出（Nuxt3 auto-import対応）
	// <template>セクションを抽出
	templateRe := regexp.MustCompile(`(?s)<template[^>]*>(.*?)</template>`)
	templateMatch := templateRe.FindStringSubmatch(content)
	if len(templateMatch) > 1 {
		templateContent := templateMatch[1]
		// PascalCase/kebab-caseのコンポーネントタグを検出
		// 例: <InputNumber>, <input-number>, <ArticleShow />
		componentTagRe := regexp.MustCompile(`<([A-Z][a-zA-Z0-9]*|[a-z]+-[a-z0-9-]+)[\s/>]`)
		tagMatches := componentTagRe.FindAllStringSubmatch(templateContent, -1)
		seenComponents := make(map[string]bool)
		for _, match := range tagMatches {
			tagName := match[1]
			// HTMLタグを除外
			htmlTags := map[string]bool{
				"div": true, "span": true, "p": true, "a": true, "img": true,
				"ul": true, "ol": true, "li": true, "h1": true, "h2": true,
				"h3": true, "h4": true, "h5": true, "h6": true, "table": true,
				"tr": true, "td": true, "th": true, "thead": true, "tbody": true,
				"form": true, "input": true, "button": true, "label": true,
				"select": true, "option": true, "textarea": true, "section": true,
				"header": true, "footer": true, "nav": true, "main": true,
				"article": true, "aside": true, "figure": true, "figcaption": true,
				"template": true, "slot": true, "component": true, "transition": true,
				"svg": true, "path": true, "rect": true, "circle": true, "line": true,
				"use": true, "defs": true, "g": true, "text": true, "tspan": true,
			}
			lowerTag := strings.ToLower(tagName)
			if htmlTags[lowerTag] {
				continue
			}
			// kebab-case を PascalCase に変換
			componentName := tagName
			if strings.Contains(tagName, "-") {
				parts := strings.Split(tagName, "-")
				componentName = ""
				for _, part := range parts {
					if len(part) > 0 {
						componentName += strings.ToUpper(part[:1]) + part[1:]
					}
				}
			}
			// 自分自身への参照は除外
			if componentName == name {
				continue
			}
			// 重複を避ける
			if seenComponents[componentName] {
				continue
			}
			seenComponents[componentName] = true

			// インポートされているコンポーネントは component: を使用
			// インポートされていない（グローバルコンポーネント）は module: を使用
			var targetID string
			if importedComponents[componentName] {
				targetID = fmt.Sprintf("component:%s", componentName)
			} else {
				targetID = fmt.Sprintf("module:%s", componentName)
			}

			// テンプレート内のコンポーネント使用は親子関係（contains）
			edge := &db.Edge{
				FromID:    nodeID,
				ToID:      targetID,
				Relation:  "contains",
				Source:    "auto",
				DefinedIn: path,
			}
			database.UpsertEdge(edge)
		}
	}

	// API呼び出しを検出
	apiRe := regexp.MustCompile(`\$fetch\(['"]([^'"]+)['"]\)|useFetch\(['"]([^'"]+)['"]\)|fetch\(['"]([^'"]+)['"]`)
	apiMatches := apiRe.FindAllStringSubmatch(content, -1)
	for _, match := range apiMatches {
		apiPath := match[1]
		if apiPath == "" {
			apiPath = match[2]
		}
		if apiPath == "" {
			apiPath = match[3]
		}
		if apiPath != "" && strings.HasPrefix(apiPath, "/api") {
			edge := &db.Edge{
				FromID:    nodeID,
				ToID:      fmt.Sprintf("api:%s", apiPath),
				Relation:  "calls",
				Source:    "auto",
				DefinedIn: path,
			}
			database.UpsertEdge(edge)
		}
	}

	return count, nil
}

func parseTypeScriptFile(database *db.DB, path, hash, content string) (int, error) {
	count := 0

	// composableかstoreかを判定
	var nodeType, nodeID string
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	if strings.Contains(path, "composables") || strings.HasPrefix(name, "use") {
		nodeType = "composable"
		nodeID = fmt.Sprintf("composable:%s", name)
	} else if strings.Contains(path, "store") {
		nodeType = "store"
		nodeID = fmt.Sprintf("store:%s", name)
	} else {
		nodeType = "module"
		nodeID = fmt.Sprintf("module:%s", name)
	}

	node := &db.Node{
		ID:       nodeID,
		Type:     nodeType,
		FilePath: path,
		FileHash: hash,
		Name:     name,
	}

	if err := database.UpsertNode(node); err != nil {
		return count, err
	}
	count++

	return count, nil
}

func parsePhpFile(database *db.DB, path, hash, content string) (int, error) {
	count := 0

	// コントローラーを検出
	if strings.Contains(path, "Controller") {
		name := strings.TrimSuffix(filepath.Base(path), ".php")
		nodeID := fmt.Sprintf("controller:%s", name)

		node := &db.Node{
			ID:       nodeID,
			Type:     "controller",
			Name:     name,
			FilePath: path,
			FileHash: hash,
		}

		if err := database.UpsertNode(node); err != nil {
			return count, err
		}
		count++

		// ルート定義からAPIパスを抽出（簡易版）
		routeRe := regexp.MustCompile(`Route::(get|post|put|delete|patch)\(['"]([^'"]+)['"]`)
		matches := routeRe.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			method := strings.ToUpper(match[1])
			apiPath := match[2]
			apiID := fmt.Sprintf("api:%s %s", method, apiPath)

			apiNode := &db.Node{
				ID:       apiID,
				Type:     "api",
				Name:     fmt.Sprintf("%s %s", method, apiPath),
				FilePath: path,
				FileHash: hash,
			}
			database.UpsertNode(apiNode)

			edge := &db.Edge{
				FromID:    nodeID,
				ToID:      apiID,
				Relation:  "implements",
				Source:    "auto",
				DefinedIn: path,
			}
			database.UpsertEdge(edge)
		}
	}

	// Modelを検出
	if strings.Contains(path, "Models") || strings.Contains(path, "Model.php") {
		name := strings.TrimSuffix(filepath.Base(path), ".php")
		nodeID := fmt.Sprintf("model:%s", name)

		node := &db.Node{
			ID:       nodeID,
			Type:     "model",
			Name:     name,
			FilePath: path,
			FileHash: hash,
		}

		if err := database.UpsertNode(node); err != nil {
			return count, err
		}
		count++

		// テーブル名を抽出
		tableRe := regexp.MustCompile(`\$table\s*=\s*['"]([^'"]+)['"]`)
		match := tableRe.FindStringSubmatch(content)
		if len(match) > 1 {
			tableName := match[1]
			tableID := fmt.Sprintf("db:%s", tableName)

			tableNode := &db.Node{
				ID:   tableID,
				Type: "db_table",
				Name: tableName,
			}
			database.UpsertNode(tableNode)

			edge := &db.Edge{
				FromID:    nodeID,
				ToID:      tableID,
				Relation:  "uses",
				Source:    "auto",
				DefinedIn: path,
			}
			database.UpsertEdge(edge)
		}
	}

	return count, nil
}

func parseGoFile(database *db.DB, path, hash, content string) (int, error) {
	count := 0

	// ファイル名からモジュール名を取得
	name := strings.TrimSuffix(filepath.Base(path), ".go")

	// パスからノードタイプとIDプレフィックスを決定
	var nodeType, nodeID string

	if strings.Contains(path, "/cmd/") || strings.HasPrefix(path, "cmd/") {
		// cmdディレクトリ内 → コマンドモジュール
		nodeType = "module"
		nodeID = fmt.Sprintf("module:%s", name)
	} else if strings.Contains(path, "/internal/db/") || strings.Contains(path, "internal/db/") {
		// internal/db → DBモジュール
		nodeType = "module"
		nodeID = fmt.Sprintf("module:db/%s", name)
	} else if strings.Contains(path, "/internal/") || strings.HasPrefix(path, "internal/") {
		// その他のinternal → 内部モジュール
		nodeType = "module"
		// パスからサブディレクトリを抽出
		parts := strings.Split(path, "/internal/")
		if len(parts) > 1 {
			subPath := strings.TrimSuffix(parts[1], ".go")
			nodeID = fmt.Sprintf("module:%s", subPath)
		} else {
			nodeID = fmt.Sprintf("module:%s", name)
		}
	} else {
		// その他 → 汎用モジュール
		nodeType = "module"
		nodeID = fmt.Sprintf("module:%s", name)
	}

	node := &db.Node{
		ID:       nodeID,
		Type:     nodeType,
		Name:     name,
		FilePath: path,
		FileHash: hash,
	}

	if err := database.UpsertNode(node); err != nil {
		return count, err
	}
	count++

	// 関数定義を検出
	funcRe := regexp.MustCompile(`func\s+(\w+)\s*\(`)
	funcMatches := funcRe.FindAllStringSubmatch(content, -1)
	for _, match := range funcMatches {
		funcName := match[1]
		// エクスポートされた関数（大文字始まり）のみ記録
		if len(funcName) > 0 && funcName[0] >= 'A' && funcName[0] <= 'Z' {
			funcID := fmt.Sprintf("func:%s.%s", name, funcName)
			funcNode := &db.Node{
				ID:       funcID,
				Type:     "function",
				Name:     funcName,
				FilePath: path,
				FileHash: hash,
			}
			database.UpsertNode(funcNode)

			// 関数はモジュールに属する
			edge := &db.Edge{
				FromID:    nodeID,
				ToID:      funcID,
				Relation:  "contains",
				Source:    "auto",
				DefinedIn: path,
			}
			database.UpsertEdge(edge)
		}
	}

	// メソッド定義を検出 (func (receiver Type) MethodName)
	methodRe := regexp.MustCompile(`func\s+\([^)]+\)\s+(\w+)\s*\(`)
	methodMatches := methodRe.FindAllStringSubmatch(content, -1)
	for _, match := range methodMatches {
		methodName := match[1]
		// エクスポートされたメソッド（大文字始まり）のみ記録
		if len(methodName) > 0 && methodName[0] >= 'A' && methodName[0] <= 'Z' {
			methodID := fmt.Sprintf("func:%s.%s", name, methodName)
			methodNode := &db.Node{
				ID:       methodID,
				Type:     "function",
				Name:     methodName,
				FilePath: path,
				FileHash: hash,
			}
			database.UpsertNode(methodNode)

			edge := &db.Edge{
				FromID:    nodeID,
				ToID:      methodID,
				Relation:  "contains",
				Source:    "auto",
				DefinedIn: path,
			}
			database.UpsertEdge(edge)
		}
	}

	return count, nil
}

// ============================================================
// Python パーサー
// ============================================================
func parsePythonFile(database *db.DB, path, hash, content string) (int, error) {
	count := 0
	name := strings.TrimSuffix(filepath.Base(path), ".py")

	// ノードタイプ判定
	var nodeType, nodeID string
	if strings.Contains(path, "/views/") || strings.Contains(path, "/api/") {
		nodeType = "controller"
		nodeID = fmt.Sprintf("controller:%s", name)
	} else if strings.Contains(path, "/models/") {
		nodeType = "model"
		nodeID = fmt.Sprintf("model:%s", name)
	} else {
		nodeType = "module"
		nodeID = fmt.Sprintf("module:%s", name)
	}

	node := &db.Node{
		ID:       nodeID,
		Type:     nodeType,
		Name:     name,
		FilePath: path,
		FileHash: hash,
	}
	if err := database.UpsertNode(node); err != nil {
		return count, err
	}
	count++

	// クラス定義を検出
	classRe := regexp.MustCompile(`class\s+(\w+)\s*[:\(]`)
	for _, match := range classRe.FindAllStringSubmatch(content, -1) {
		className := match[1]
		classID := fmt.Sprintf("class:%s.%s", name, className)
		classNode := &db.Node{
			ID:       classID,
			Type:     "class",
			Name:     className,
			FilePath: path,
			FileHash: hash,
		}
		database.UpsertNode(classNode)
		database.UpsertEdge(&db.Edge{
			FromID: nodeID, ToID: classID, Relation: "contains", Source: "auto", DefinedIn: path,
		})
	}

	// 関数定義を検出（トップレベル）
	funcRe := regexp.MustCompile(`(?m)^def\s+(\w+)\s*\(`)
	for _, match := range funcRe.FindAllStringSubmatch(content, -1) {
		funcName := match[1]
		if !strings.HasPrefix(funcName, "_") { // プライベート関数は除外
			funcID := fmt.Sprintf("func:%s.%s", name, funcName)
			funcNode := &db.Node{
				ID:       funcID,
				Type:     "function",
				Name:     funcName,
				FilePath: path,
				FileHash: hash,
			}
			database.UpsertNode(funcNode)
			database.UpsertEdge(&db.Edge{
				FromID: nodeID, ToID: funcID, Relation: "contains", Source: "auto", DefinedIn: path,
			})
		}
	}

	// Flask/FastAPIルート検出
	routeRe := regexp.MustCompile(`@(?:app|router)\.(get|post|put|delete|patch)\s*\(\s*['"](/[^'"]*)['"]\s*\)`)
	for _, match := range routeRe.FindAllStringSubmatch(content, -1) {
		method := strings.ToUpper(match[1])
		apiPath := match[2]
		apiID := fmt.Sprintf("api:%s %s", method, apiPath)
		apiNode := &db.Node{
			ID:       apiID,
			Type:     "api",
			Name:     fmt.Sprintf("%s %s", method, apiPath),
			FilePath: path,
			FileHash: hash,
		}
		database.UpsertNode(apiNode)
		database.UpsertEdge(&db.Edge{
			FromID: nodeID, ToID: apiID, Relation: "implements", Source: "auto", DefinedIn: path,
		})
	}

	return count, nil
}

// ============================================================
// Java パーサー
// ============================================================
func parseJavaFile(database *db.DB, path, hash, content string) (int, error) {
	count := 0
	name := strings.TrimSuffix(filepath.Base(path), ".java")

	// ノードタイプ判定
	var nodeType, nodeID string
	if strings.Contains(path, "/controller/") || strings.HasSuffix(name, "Controller") {
		nodeType = "controller"
		nodeID = fmt.Sprintf("controller:%s", name)
	} else if strings.Contains(path, "/model/") || strings.Contains(path, "/entity/") {
		nodeType = "model"
		nodeID = fmt.Sprintf("model:%s", name)
	} else if strings.Contains(path, "/service/") {
		nodeType = "service"
		nodeID = fmt.Sprintf("service:%s", name)
	} else if strings.Contains(path, "/repository/") {
		nodeType = "repository"
		nodeID = fmt.Sprintf("repository:%s", name)
	} else {
		nodeType = "class"
		nodeID = fmt.Sprintf("class:%s", name)
	}

	node := &db.Node{
		ID:       nodeID,
		Type:     nodeType,
		Name:     name,
		FilePath: path,
		FileHash: hash,
	}
	if err := database.UpsertNode(node); err != nil {
		return count, err
	}
	count++

	// publicメソッドを検出
	methodRe := regexp.MustCompile(`public\s+(?:static\s+)?[\w<>\[\]]+\s+(\w+)\s*\(`)
	for _, match := range methodRe.FindAllStringSubmatch(content, -1) {
		methodName := match[1]
		if methodName != name { // コンストラクタは除外
			methodID := fmt.Sprintf("method:%s.%s", name, methodName)
			methodNode := &db.Node{
				ID:       methodID,
				Type:     "method",
				Name:     methodName,
				FilePath: path,
				FileHash: hash,
			}
			database.UpsertNode(methodNode)
			database.UpsertEdge(&db.Edge{
				FromID: nodeID, ToID: methodID, Relation: "contains", Source: "auto", DefinedIn: path,
			})
		}
	}

	// Spring Boot APIエンドポイント検出
	mappingRe := regexp.MustCompile(`@(Get|Post|Put|Delete|Patch)Mapping\s*\(\s*(?:value\s*=\s*)?['"](/[^'"]*)['"]\s*\)`)
	for _, match := range mappingRe.FindAllStringSubmatch(content, -1) {
		method := strings.ToUpper(match[1])
		apiPath := match[2]
		apiID := fmt.Sprintf("api:%s %s", method, apiPath)
		apiNode := &db.Node{
			ID:       apiID,
			Type:     "api",
			Name:     fmt.Sprintf("%s %s", method, apiPath),
			FilePath: path,
			FileHash: hash,
		}
		database.UpsertNode(apiNode)
		database.UpsertEdge(&db.Edge{
			FromID: nodeID, ToID: apiID, Relation: "implements", Source: "auto", DefinedIn: path,
		})
	}

	return count, nil
}

// ============================================================
// Rust パーサー
// ============================================================
func parseRustFile(database *db.DB, path, hash, content string) (int, error) {
	count := 0
	name := strings.TrimSuffix(filepath.Base(path), ".rs")

	nodeType := "module"
	nodeID := fmt.Sprintf("module:%s", name)

	node := &db.Node{
		ID:       nodeID,
		Type:     nodeType,
		Name:     name,
		FilePath: path,
		FileHash: hash,
	}
	if err := database.UpsertNode(node); err != nil {
		return count, err
	}
	count++

	// pub struct検出
	structRe := regexp.MustCompile(`pub\s+struct\s+(\w+)`)
	for _, match := range structRe.FindAllStringSubmatch(content, -1) {
		structName := match[1]
		structID := fmt.Sprintf("struct:%s.%s", name, structName)
		structNode := &db.Node{
			ID:       structID,
			Type:     "struct",
			Name:     structName,
			FilePath: path,
			FileHash: hash,
		}
		database.UpsertNode(structNode)
		database.UpsertEdge(&db.Edge{
			FromID: nodeID, ToID: structID, Relation: "contains", Source: "auto", DefinedIn: path,
		})
	}

	// pub fn検出
	funcRe := regexp.MustCompile(`pub\s+(?:async\s+)?fn\s+(\w+)`)
	for _, match := range funcRe.FindAllStringSubmatch(content, -1) {
		funcName := match[1]
		funcID := fmt.Sprintf("func:%s.%s", name, funcName)
		funcNode := &db.Node{
			ID:       funcID,
			Type:     "function",
			Name:     funcName,
			FilePath: path,
			FileHash: hash,
		}
		database.UpsertNode(funcNode)
		database.UpsertEdge(&db.Edge{
			FromID: nodeID, ToID: funcID, Relation: "contains", Source: "auto", DefinedIn: path,
		})
	}

	// Actix-web / Axumルート検出
	routeRe := regexp.MustCompile(`#\[(get|post|put|delete|patch)\s*\(\s*"(/[^"]*)"\s*\)\]`)
	for _, match := range routeRe.FindAllStringSubmatch(content, -1) {
		method := strings.ToUpper(match[1])
		apiPath := match[2]
		apiID := fmt.Sprintf("api:%s %s", method, apiPath)
		apiNode := &db.Node{
			ID:       apiID,
			Type:     "api",
			Name:     fmt.Sprintf("%s %s", method, apiPath),
			FilePath: path,
			FileHash: hash,
		}
		database.UpsertNode(apiNode)
		database.UpsertEdge(&db.Edge{
			FromID: nodeID, ToID: apiID, Relation: "implements", Source: "auto", DefinedIn: path,
		})
	}

	return count, nil
}

// ============================================================
// Ruby パーサー
// ============================================================
func parseRubyFile(database *db.DB, path, hash, content string) (int, error) {
	count := 0
	name := strings.TrimSuffix(filepath.Base(path), ".rb")

	// ノードタイプ判定
	var nodeType, nodeID string
	if strings.Contains(path, "/controllers/") || strings.HasSuffix(name, "_controller") {
		nodeType = "controller"
		nodeID = fmt.Sprintf("controller:%s", name)
	} else if strings.Contains(path, "/models/") {
		nodeType = "model"
		nodeID = fmt.Sprintf("model:%s", name)
	} else {
		nodeType = "module"
		nodeID = fmt.Sprintf("module:%s", name)
	}

	node := &db.Node{
		ID:       nodeID,
		Type:     nodeType,
		Name:     name,
		FilePath: path,
		FileHash: hash,
	}
	if err := database.UpsertNode(node); err != nil {
		return count, err
	}
	count++

	// クラス定義を検出
	classRe := regexp.MustCompile(`class\s+(\w+)`)
	for _, match := range classRe.FindAllStringSubmatch(content, -1) {
		className := match[1]
		classID := fmt.Sprintf("class:%s.%s", name, className)
		classNode := &db.Node{
			ID:       classID,
			Type:     "class",
			Name:     className,
			FilePath: path,
			FileHash: hash,
		}
		database.UpsertNode(classNode)
		database.UpsertEdge(&db.Edge{
			FromID: nodeID, ToID: classID, Relation: "contains", Source: "auto", DefinedIn: path,
		})
	}

	// publicメソッド検出
	methodRe := regexp.MustCompile(`(?m)^\s*def\s+(\w+)`)
	for _, match := range methodRe.FindAllStringSubmatch(content, -1) {
		methodName := match[1]
		if !strings.HasPrefix(methodName, "_") {
			methodID := fmt.Sprintf("method:%s.%s", name, methodName)
			methodNode := &db.Node{
				ID:       methodID,
				Type:     "method",
				Name:     methodName,
				FilePath: path,
				FileHash: hash,
			}
			database.UpsertNode(methodNode)
			database.UpsertEdge(&db.Edge{
				FromID: nodeID, ToID: methodID, Relation: "contains", Source: "auto", DefinedIn: path,
			})
		}
	}

	return count, nil
}

// ============================================================
// C# パーサー
// ============================================================
func parseCSharpFile(database *db.DB, path, hash, content string) (int, error) {
	count := 0
	name := strings.TrimSuffix(filepath.Base(path), ".cs")

	// ノードタイプ判定
	var nodeType, nodeID string
	if strings.Contains(path, "/Controllers/") || strings.HasSuffix(name, "Controller") {
		nodeType = "controller"
		nodeID = fmt.Sprintf("controller:%s", name)
	} else if strings.Contains(path, "/Models/") {
		nodeType = "model"
		nodeID = fmt.Sprintf("model:%s", name)
	} else if strings.Contains(path, "/Services/") {
		nodeType = "service"
		nodeID = fmt.Sprintf("service:%s", name)
	} else {
		nodeType = "class"
		nodeID = fmt.Sprintf("class:%s", name)
	}

	node := &db.Node{
		ID:       nodeID,
		Type:     nodeType,
		Name:     name,
		FilePath: path,
		FileHash: hash,
	}
	if err := database.UpsertNode(node); err != nil {
		return count, err
	}
	count++

	// publicメソッド検出
	methodRe := regexp.MustCompile(`public\s+(?:async\s+)?(?:static\s+)?[\w<>\[\]]+\s+(\w+)\s*\(`)
	for _, match := range methodRe.FindAllStringSubmatch(content, -1) {
		methodName := match[1]
		if methodName != name { // コンストラクタは除外
			methodID := fmt.Sprintf("method:%s.%s", name, methodName)
			methodNode := &db.Node{
				ID:       methodID,
				Type:     "method",
				Name:     methodName,
				FilePath: path,
				FileHash: hash,
			}
			database.UpsertNode(methodNode)
			database.UpsertEdge(&db.Edge{
				FromID: nodeID, ToID: methodID, Relation: "contains", Source: "auto", DefinedIn: path,
			})
		}
	}

	// ASP.NET Core APIエンドポイント検出
	routeRe := regexp.MustCompile(`\[Http(Get|Post|Put|Delete|Patch)\s*\(\s*"([^"]*)"\s*\)\]`)
	for _, match := range routeRe.FindAllStringSubmatch(content, -1) {
		method := strings.ToUpper(match[1])
		apiPath := match[2]
		if !strings.HasPrefix(apiPath, "/") {
			apiPath = "/" + apiPath
		}
		apiID := fmt.Sprintf("api:%s %s", method, apiPath)
		apiNode := &db.Node{
			ID:       apiID,
			Type:     "api",
			Name:     fmt.Sprintf("%s %s", method, apiPath),
			FilePath: path,
			FileHash: hash,
		}
		database.UpsertNode(apiNode)
		database.UpsertEdge(&db.Edge{
			FromID: nodeID, ToID: apiID, Relation: "implements", Source: "auto", DefinedIn: path,
		})
	}

	return count, nil
}

// ============================================================
// Kotlin パーサー
// ============================================================
func parseKotlinFile(database *db.DB, path, hash, content string) (int, error) {
	count := 0
	ext := filepath.Ext(path)
	name := strings.TrimSuffix(filepath.Base(path), ext)

	// ノードタイプ判定
	var nodeType, nodeID string
	if strings.Contains(path, "/controller/") || strings.HasSuffix(name, "Controller") {
		nodeType = "controller"
		nodeID = fmt.Sprintf("controller:%s", name)
	} else if strings.Contains(path, "/model/") || strings.Contains(path, "/entity/") {
		nodeType = "model"
		nodeID = fmt.Sprintf("model:%s", name)
	} else if strings.Contains(path, "/service/") {
		nodeType = "service"
		nodeID = fmt.Sprintf("service:%s", name)
	} else {
		nodeType = "class"
		nodeID = fmt.Sprintf("class:%s", name)
	}

	node := &db.Node{
		ID:       nodeID,
		Type:     nodeType,
		Name:     name,
		FilePath: path,
		FileHash: hash,
	}
	if err := database.UpsertNode(node); err != nil {
		return count, err
	}
	count++

	// fun検出
	funcRe := regexp.MustCompile(`fun\s+(\w+)\s*\(`)
	for _, match := range funcRe.FindAllStringSubmatch(content, -1) {
		funcName := match[1]
		funcID := fmt.Sprintf("func:%s.%s", name, funcName)
		funcNode := &db.Node{
			ID:       funcID,
			Type:     "function",
			Name:     funcName,
			FilePath: path,
			FileHash: hash,
		}
		database.UpsertNode(funcNode)
		database.UpsertEdge(&db.Edge{
			FromID: nodeID, ToID: funcID, Relation: "contains", Source: "auto", DefinedIn: path,
		})
	}

	// Spring Boot APIエンドポイント検出（Javaと同じ）
	mappingRe := regexp.MustCompile(`@(Get|Post|Put|Delete|Patch)Mapping\s*\(\s*(?:value\s*=\s*)?['"](/[^'"]*)['"]\s*\)`)
	for _, match := range mappingRe.FindAllStringSubmatch(content, -1) {
		method := strings.ToUpper(match[1])
		apiPath := match[2]
		apiID := fmt.Sprintf("api:%s %s", method, apiPath)
		apiNode := &db.Node{
			ID:       apiID,
			Type:     "api",
			Name:     fmt.Sprintf("%s %s", method, apiPath),
			FilePath: path,
			FileHash: hash,
		}
		database.UpsertNode(apiNode)
		database.UpsertEdge(&db.Edge{
			FromID: nodeID, ToID: apiID, Relation: "implements", Source: "auto", DefinedIn: path,
		})
	}

	return count, nil
}

// ============================================================
// Swift パーサー
// ============================================================
func parseSwiftFile(database *db.DB, path, hash, content string) (int, error) {
	count := 0
	name := strings.TrimSuffix(filepath.Base(path), ".swift")

	// ノードタイプ判定
	var nodeType, nodeID string
	if strings.Contains(path, "/Controllers/") || strings.HasSuffix(name, "Controller") {
		nodeType = "controller"
		nodeID = fmt.Sprintf("controller:%s", name)
	} else if strings.Contains(path, "/Models/") {
		nodeType = "model"
		nodeID = fmt.Sprintf("model:%s", name)
	} else if strings.Contains(path, "/Views/") || strings.HasSuffix(name, "View") {
		nodeType = "view"
		nodeID = fmt.Sprintf("view:%s", name)
	} else {
		nodeType = "class"
		nodeID = fmt.Sprintf("class:%s", name)
	}

	node := &db.Node{
		ID:       nodeID,
		Type:     nodeType,
		Name:     name,
		FilePath: path,
		FileHash: hash,
	}
	if err := database.UpsertNode(node); err != nil {
		return count, err
	}
	count++

	// class/struct検出
	typeRe := regexp.MustCompile(`(?:class|struct)\s+(\w+)`)
	for _, match := range typeRe.FindAllStringSubmatch(content, -1) {
		typeName := match[1]
		typeID := fmt.Sprintf("class:%s.%s", name, typeName)
		typeNode := &db.Node{
			ID:       typeID,
			Type:     "class",
			Name:     typeName,
			FilePath: path,
			FileHash: hash,
		}
		database.UpsertNode(typeNode)
		database.UpsertEdge(&db.Edge{
			FromID: nodeID, ToID: typeID, Relation: "contains", Source: "auto", DefinedIn: path,
		})
	}

	// func検出
	funcRe := regexp.MustCompile(`func\s+(\w+)\s*\(`)
	for _, match := range funcRe.FindAllStringSubmatch(content, -1) {
		funcName := match[1]
		funcID := fmt.Sprintf("func:%s.%s", name, funcName)
		funcNode := &db.Node{
			ID:       funcID,
			Type:     "function",
			Name:     funcName,
			FilePath: path,
			FileHash: hash,
		}
		database.UpsertNode(funcNode)
		database.UpsertEdge(&db.Edge{
			FromID: nodeID, ToID: funcID, Relation: "contains", Source: "auto", DefinedIn: path,
		})
	}

	return count, nil
}

// ============================================================
// Dart パーサー
// ============================================================
func parseDartFile(database *db.DB, path, hash, content string) (int, error) {
	count := 0
	name := strings.TrimSuffix(filepath.Base(path), ".dart")

	// ノードタイプ判定
	var nodeType, nodeID string
	if strings.Contains(path, "/screens/") || strings.Contains(path, "/pages/") || strings.HasSuffix(name, "_screen") || strings.HasSuffix(name, "_page") {
		nodeType = "view"
		nodeID = fmt.Sprintf("view:%s", name)
	} else if strings.Contains(path, "/widgets/") || strings.HasSuffix(name, "_widget") {
		nodeType = "component"
		nodeID = fmt.Sprintf("component:%s", name)
	} else if strings.Contains(path, "/models/") {
		nodeType = "model"
		nodeID = fmt.Sprintf("model:%s", name)
	} else if strings.Contains(path, "/services/") || strings.Contains(path, "/api/") {
		nodeType = "service"
		nodeID = fmt.Sprintf("service:%s", name)
	} else if strings.Contains(path, "/providers/") || strings.Contains(path, "/bloc/") || strings.Contains(path, "/cubit/") {
		nodeType = "store"
		nodeID = fmt.Sprintf("store:%s", name)
	} else {
		nodeType = "module"
		nodeID = fmt.Sprintf("module:%s", name)
	}

	node := &db.Node{
		ID:       nodeID,
		Type:     nodeType,
		Name:     name,
		FilePath: path,
		FileHash: hash,
	}
	if err := database.UpsertNode(node); err != nil {
		return count, err
	}
	count++

	// class検出
	classRe := regexp.MustCompile(`class\s+(\w+)`)
	for _, match := range classRe.FindAllStringSubmatch(content, -1) {
		className := match[1]
		classID := fmt.Sprintf("class:%s.%s", name, className)
		classNode := &db.Node{
			ID:       classID,
			Type:     "class",
			Name:     className,
			FilePath: path,
			FileHash: hash,
		}
		database.UpsertNode(classNode)
		database.UpsertEdge(&db.Edge{
			FromID: nodeID, ToID: classID, Relation: "contains", Source: "auto", DefinedIn: path,
		})
	}

	// トップレベル関数検出
	funcRe := regexp.MustCompile(`(?m)^[\w<>\[\]?]+\s+(\w+)\s*\(`)
	for _, match := range funcRe.FindAllStringSubmatch(content, -1) {
		funcName := match[1]
		// build, initState などのFlutter標準メソッドは除外
		if funcName != "build" && funcName != "initState" && funcName != "dispose" && !strings.HasPrefix(funcName, "_") {
			funcID := fmt.Sprintf("func:%s.%s", name, funcName)
			funcNode := &db.Node{
				ID:       funcID,
				Type:     "function",
				Name:     funcName,
				FilePath: path,
				FileHash: hash,
			}
			database.UpsertNode(funcNode)
			database.UpsertEdge(&db.Edge{
				FromID: nodeID, ToID: funcID, Relation: "contains", Source: "auto", DefinedIn: path,
			})
		}
	}

	return count, nil
}

func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
