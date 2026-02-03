---
trace:
  id: doc:docs/05_プログラム設計/scanコマンド設計書.md
  parent:
    - doc:docs/04_詳細設計/スキャン処理詳細設計書.md
  uses:
    - module:scan
---

# scan コマンド プログラム設計書

## 1. ファイル構成

```
cmd/scan.go
```

---

## 2. 主要関数

### 2.1 runScan

```go
func runScan(cmd *cobra.Command, args []string) error
```

**責務**: スキャンコマンドのエントリポイント

**処理**:
1. パス引数の取得（デフォルト: `.`）
2. DB接続
3. `scanDocs()` 実行（`--code-only` でなければ）
4. `scanCode()` 実行（`--docs-only` でなければ）
5. 結果表示

### 2.2 scanDocs

```go
func scanDocs(database *db.DB, rootPath string) (int, error)
```

**責務**: ドキュメントのスキャン

**処理**:
1. `filepath.Walk()` で再帰走査
2. `.md` ファイルをフィルタ
3. `docs/` 配下のみ対象
4. `99_アーカイブ`, `90_作業ログ` を除外
5. `parseFrontMatter()` でYAML解析
6. ノード・エッジをDB保存

### 2.3 scanCode

```go
func scanCode(database *db.DB, rootPath string) (int, error)
```

**責務**: コードのスキャン

**処理**:
1. `filepath.Walk()` で再帰走査
2. 除外ディレクトリをスキップ
3. 拡張子に応じてパーサーを呼び出し

### 2.4 parseFrontMatter

```go
func parseFrontMatter(content string) (*FrontMatter, error)
```

**責務**: YAMLフロントマターの解析

**処理**:
1. `---` で囲まれた部分を抽出
2. YAML unmarshal
3. `TraceInfo` 構造体を返却

### 2.5 parseVueFile

```go
func parseVueFile(database *db.DB, path, hash, content string) (int, error)
```

**責務**: Vueファイルの解析

**検出パターン**:
- `import ... from '...vue'`
- `<ComponentName>` タグ
- `$fetch()`, `useFetch()`, `fetch()` 呼び出し

### 2.6 parseTypeScriptFile

```go
func parseTypeScriptFile(database *db.DB, path, hash, content string) (int, error)
```

**責務**: TypeScriptファイルの解析

**ノードタイプ判定**:
- パスに `composables` or 名前が `use` 始まり → `composable`
- パスに `store` → `store`
- その他 → `module`

### 2.7 parsePhpFile

```go
func parsePhpFile(database *db.DB, path, hash, content string) (int, error)
```

**責務**: PHPファイルの解析

**検出対象**:
- コントローラー: `Route::xxx()` からAPIパス抽出
- モデル: `$table` プロパティからテーブル名抽出

---

## 3. 正規表現パターン

### 3.1 import文

```go
importRe := regexp.MustCompile(`import\s+(?:\{[^}]+\}|[^{}\s]+)\s+from\s+['"]([^'"]+)['"]`)
```

### 3.2 テンプレートコンポーネント

```go
templateRe := regexp.MustCompile(`(?s)<template[^>]*>(.*?)</template>`)
componentTagRe := regexp.MustCompile(`<([A-Z][a-zA-Z0-9]*|[a-z]+-[a-z0-9-]+)[\s/>]`)
```

### 3.3 API呼び出し

```go
apiRe := regexp.MustCompile(`\$fetch\(['"]([^'"]+)['"]\)|useFetch\(['"]([^'"]+)['"]\)|fetch\(['"]([^'"]+)['"]`)
```

### 3.4 PHPルート定義

```go
routeRe := regexp.MustCompile(`Route::(get|post|put|delete|patch)\(['"]([^'"]+)['"]`)
```

### 3.5 PHPテーブル定義

```go
tableRe := regexp.MustCompile(`\$table\s*=\s*['"]([^'"]+)['"]`)
```

---

## 4. ファイルハッシュ

```go
func fileHash(path string) (string, error)
```

SHA256ハッシュを計算。変更検知に使用。
