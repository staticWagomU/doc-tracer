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

	"doc-tracer/internal/db"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var scanCmd = &cobra.Command{
	Use:   "scan [path]",
	Short: "コードベースとドキュメントをスキャン",
	Long: `指定されたパスをスキャンして、ノードとエッジを構築します。

例:
  doc-tracer scan .                    # カレントディレクトリをスキャン
  doc-tracer scan ./docs --docs-only   # ドキュメントのみスキャン
  doc-tracer scan ./src --code-only    # コードのみスキャン`,
	Args: cobra.MaximumNArgs(1),
	RunE: runScan,
}

var (
	docsOnly bool
	codeOnly bool
)

func init() {
	scanCmd.Flags().BoolVar(&docsOnly, "docs-only", false, "ドキュメントのみスキャン")
	scanCmd.Flags().BoolVar(&codeOnly, "code-only", false, "コードのみスキャン")
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
	}

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			// 除外ディレクトリ
			name := info.Name()
			if name == "node_modules" || name == "vendor" || name == ".git" || name == "dist" || name == "build" {
				return filepath.SkipDir
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

	// import文を解析
	importRe := regexp.MustCompile(`import\s+(?:\{[^}]+\}|[^{}\s]+)\s+from\s+['"]([^'"]+)['"]`)
	matches := importRe.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		importPath := match[1]
		// 相対パスのコンポーネントへの参照を検出（親子関係）
		if strings.HasSuffix(importPath, ".vue") || strings.Contains(importPath, "components/") {
			targetName := filepath.Base(importPath)
			targetName = strings.TrimSuffix(targetName, ".vue")
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
			// テンプレート内のコンポーネント使用は親子関係（contains）
			edge := &db.Edge{
				FromID:    nodeID,
				ToID:      fmt.Sprintf("component:%s", componentName),
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
