# doc-tracer

ドキュメントとコード間のトレーサビリティを可視化・管理するCLIツール。

コード変更時に影響を受けるドキュメントを特定し、ドキュメント更新の抜け漏れを防ぎます。

## 特徴

- **5層構造対応**: 要件定義 → 仕様書 → 基本設計 → 詳細設計 → プログラム設計 → コード
- **双方向トレース**: ドキュメントからコードへ、コードからドキュメントへ
- **自動検出**: コードの依存関係を静的解析で自動抽出
- **Web UI**: D3.jsによるインタラクティブなグラフ可視化
- **整合性チェック**: リンク切れ・孤立ドキュメントの検出

---

## インストール

```bash
# リポジトリをクローン
git clone https://github.com/tomohiro-owada/doc-tracer.git
cd doc-tracer

# ビルド
go build -o doc-tracer .

# パスに追加（オプション）
sudo mv doc-tracer /usr/local/bin/
```

### 動作要件

- Go 1.21以上

---

## クイックスタート

```bash
# 1. プロジェクトをスキャン
doc-tracer scan /path/to/your/project

# 2. Web UIで可視化
doc-tracer serve --port 8080

# 3. ブラウザで http://localhost:8080 にアクセス
```

---

## コマンド一覧

### `scan` - スキャン

コードベースとドキュメントをスキャンして、ノードとエッジをDBに格納します。

```bash
# カレントディレクトリをスキャン
doc-tracer scan

# 特定ディレクトリをスキャン
doc-tracer scan /path/to/project

# ドキュメントのみスキャン
doc-tracer scan --docs-only

# コードのみスキャン
doc-tracer scan --code-only

# DBファイルを指定
doc-tracer scan --db custom.db
```

### `serve` - Web UI起動

トレーサビリティグラフを可視化するWeb UIを起動します。

```bash
# デフォルトポート（8080）で起動
doc-tracer serve

# ポート指定
doc-tracer serve --port 3000
```

**Web UI機能**:
- ノードのドラッグ＆ドロップ
- ズーム・パン操作
- ノード選択で上流方向（影響を受ける側）をハイライト
- ファイル名・ノード名での検索
- 統計情報表示

### `check` - 整合性チェック

トレーサビリティDBの整合性をチェックします。

```bash
doc-tracer check
```

**チェック項目**:
- リンク切れ（存在しないノードへの参照）
- 孤立ドキュメント（どこからも参照されていない）
- 統計情報（ノード/エッジ数、タイプ別集計）

### `impact` - 影響範囲検索

指定ファイルを変更した場合に影響を受けるドキュメントを表示します。

```bash
# 影響を受けるドキュメントを表示
doc-tracer impact src/components/LoanForm.vue

# JSON形式で出力
doc-tracer impact --json src/api/loan.ts
```

---

## YAMLフロントマター仕様

Markdownドキュメントの先頭に YAML フロントマターを記述することで、トレーサビリティ関係を定義します。

### 基本構造

```yaml
---
trace:
  id: doc:docs/03_基本設計/認証設計書.md
  parent:
    - doc:docs/02_仕様書/認証仕様書.md
  children:
    - doc:docs/04_詳細設計/ログイン詳細設計.md
    - doc:docs/04_詳細設計/認証API詳細設計.md
  implements:
    - doc:docs/01_要件定義/機能要件定義書.md
  uses:
    - component:LoginForm
    - api:/api/v1/auth/login
---

# 認証設計書

本文...
```

### フィールド詳細

| フィールド | 説明 | 生成されるエッジ |
|-----------|------|-----------------|
| `id` | このドキュメントの一意識別子 | - |
| `parent` | 親ドキュメント（上位層）のID | 親 → 自分 (`contains`) |
| `children` | 子ドキュメント（下位層）のID | 自分 → 子 (`contains`) |
| `implements` | 実装元（要件など）のID | 自分 → 対象 (`implements`) |
| `uses` | 使用するコード要素のID | 自分 → 対象 (`uses`) |

### ID命名規則

| プレフィックス | 対象 | 例 |
|--------------|------|-----|
| `doc:` | ドキュメント | `doc:docs/03_基本設計/認証設計書.md` |
| `component:` | Vueコンポーネント | `component:LoginForm` |
| `api:` | APIエンドポイント | `api:/api/v1/users` |
| `controller:` | PHPコントローラー | `controller:UserController` |
| `model:` | Eloquentモデル | `model:User` |
| `store:` | Piniaストア | `store:useAuthStore` |
| `composable:` | Vue Composable | `composable:useAuth` |
| `module:` | TypeScriptモジュール | `module:utils` |
| `db:` | DBテーブル | `db:users` |

### 階層構造の例（5層モデル）

```
docs/
├── 01_要件定義/
│   └── 機能要件定義書.md          # 最上位層
├── 02_仕様書/
│   └── 認証仕様書.md              # parent: 01_要件定義
├── 03_基本設計/
│   └── 認証基本設計書.md          # parent: 02_仕様書
├── 04_詳細設計/
│   └── ログイン詳細設計書.md      # parent: 03_基本設計
└── 05_プログラム設計/
    └── ログインフォーム設計書.md  # parent: 04_詳細設計, uses: コード
```

---

## データベース構造

SQLiteを使用してトレーサビリティ情報を永続化します。

### nodes テーブル

コード要素やドキュメントを表すノード。

| カラム | 型 | 説明 |
|--------|-----|------|
| `id` | TEXT (PK) | 一意識別子（例: `doc:docs/xxx.md`） |
| `type` | TEXT | ノードタイプ（下記参照） |
| `name` | TEXT | 表示名 |
| `file_path` | TEXT | ファイルパス |
| `file_hash` | TEXT | ファイルのSHA256ハッシュ（変更検知用） |
| `line_start` | INTEGER | 開始行（コード要素の場合） |
| `line_end` | INTEGER | 終了行 |
| `metadata` | TEXT | 追加メタデータ（JSON） |
| `updated_at` | DATETIME | 最終更新日時 |

**ノードタイプ**:
- `document` - Markdownドキュメント
- `component` - Vueコンポーネント
- `api` - APIエンドポイント
- `controller` - PHPコントローラー
- `model` - Eloquentモデル
- `store` - Piniaストア
- `composable` - Vue Composable
- `module` - TypeScriptモジュール
- `db_table` - データベーステーブル

### edges テーブル

ノード間の関係（エッジ）。

| カラム | 型 | 説明 |
|--------|-----|------|
| `id` | INTEGER (PK) | 自動採番ID |
| `from_id` | TEXT | 起点ノードID |
| `to_id` | TEXT | 終点ノードID |
| `relation` | TEXT | 関係タイプ（下記参照） |
| `source` | TEXT | `auto`（自動検出）または `manual`（手動定義） |
| `defined_in` | TEXT | 定義元ファイルパス |
| `created_at` | DATETIME | 作成日時 |

**関係タイプ**:

| 関係 | 意味 | 方向 |
|------|------|------|
| `contains` | 親子関係（階層構造） | 親 → 子 |
| `implements` | 実装関係 | 実装 → 要件 |
| `uses` | 使用関係 | 使用側 → 被使用側 |
| `calls` | API呼び出し | コンポーネント → API |

### scan_history テーブル

スキャン履歴（増分スキャン用）。

| カラム | 型 | 説明 |
|--------|-----|------|
| `id` | INTEGER (PK) | 自動採番ID |
| `file_path` | TEXT | ファイルパス |
| `file_hash` | TEXT | スキャン時のハッシュ |
| `scanned_at` | DATETIME | スキャン日時 |

---

## 自動検出機能

`scan` コマンドは以下のコード要素を自動的に検出します。

### 対象ファイル

| 拡張子 | 解析内容 |
|--------|---------|
| `.vue` | コンポーネント、import、テンプレート内コンポーネント使用、API呼び出し |
| `.ts`, `.tsx`, `.js` | composable、store、module |
| `.php` | コントローラー、モデル、ルート定義、テーブル定義 |
| `.go` | 将来対応予定 |

### 除外ディレクトリ

- `node_modules/`
- `vendor/`
- `.git/`
- `dist/`
- `build/`

### ドキュメント除外パターン

- `99_アーカイブ/` - アーカイブドキュメント
- `90_作業ログ/` - 作業ログ

### Vueコンポーネント解析

```vue
<template>
  <!-- 自動検出: contains関係 -->
  <LoginForm />
  <input-number />  <!-- kebab-case も対応 -->
</template>

<script setup>
// 自動検出: contains関係
import LoginForm from './LoginForm.vue'

// 自動検出: calls関係
const { data } = await useFetch('/api/v1/users')
await $fetch('/api/v1/auth/login')
</script>
```

### PHPコントローラー解析

```php
class UserController extends Controller
{
    // 自動検出: implements関係（api ノード生成）
    Route::get('/api/v1/users', ...);
    Route::post('/api/v1/users', ...);
}
```

### PHPモデル解析

```php
class User extends Model
{
    // 自動検出: uses関係（db_table ノード生成）
    protected $table = 'users';
}
```

---

## 使用例

### 複数リポジトリのスキャン

```bash
#!/bin/bash

# ドキュメントリポジトリをスキャン
doc-tracer scan /path/to/docs --docs-only

# フロントエンドリポジトリをスキャン（コードのみ）
doc-tracer scan /path/to/frontend --code-only

# バックエンドリポジトリをスキャン（コードのみ）
doc-tracer scan /path/to/backend --code-only

# 整合性チェック
doc-tracer check

# Web UIで確認
doc-tracer serve
```

### CI/CD連携

```yaml
# GitHub Actions例
- name: Check documentation consistency
  run: |
    doc-tracer scan .
    doc-tracer check
```

### 影響範囲の確認（コードレビュー時）

```bash
# 変更ファイルの影響を確認
git diff --name-only HEAD~1 | while read file; do
  echo "=== $file ==="
  doc-tracer impact "$file"
done
```

---

## 設定ファイル

`tracer.yaml`（オプション）でスキャン設定をカスタマイズできます。

```yaml
# スキャン対象パス
scan_paths:
  - ./docs
  - ./src
  - ./app

# 除外パターン
exclude_patterns:
  - "**/test/**"
  - "**/fixtures/**"

# ドキュメントルート
docs_root: ./docs
```

---

## アーキテクチャ

```
doc-tracer/
├── main.go              # エントリポイント
├── cmd/
│   ├── root.go          # ルートコマンド、グローバルフラグ
│   ├── scan.go          # スキャンコマンド、ファイル解析
│   ├── serve.go         # Webサーバー、API
│   ├── check.go         # 整合性チェック
│   ├── impact.go        # 影響範囲検索
│   └── web/
│       └── index.html   # Web UI（D3.js）
├── internal/
│   └── db/
│       └── schema.go    # DBスキーマ、CRUD操作
├── go.mod
└── go.sum
```

---

## ライセンス

MIT License

---

## 関連プロジェクト

- [Cobra](https://github.com/spf13/cobra) - CLIフレームワーク
- [D3.js](https://d3js.org/) - グラフ可視化
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) - Pure Go SQLite
