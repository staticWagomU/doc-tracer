# doc-tracer

**ドキュメントとコードの双方向トレーサビリティを実現するツール**

[English](README.md) | [日本語](README.ja.md)

<img width="1263" height="738" alt="Screenshot 2026-02-03 at 2 50 43 PM" src="https://github.com/user-attachments/assets/160b1347-790c-49a4-b1b7-f0a1e7ee6b27" />


---

## このツールが解決する問題

### 問題1: ドキュメントとコードの乖離

ソフトウェア開発において、ドキュメントとコードは常に乖離する傾向にあります。

- コードを変更したが、関連するドキュメントを更新し忘れた
- どのドキュメントを更新すべきか分からない
- ドキュメントが古くなり、誰も信用しなくなる

### 問題2: 影響範囲が見えない

コードを変更するとき、その変更がどこに影響するか把握するのは困難です。

- このAPIを変更したら、どの画面に影響する？
- このコンポーネントを修正したら、どの仕様書を更新すべき？
- 要件変更があったとき、どのコードを修正する必要がある？

### 問題3: AIによるドキュメント生成の限界

Claude CodeのようなAIツールがコードを生成・修正する際、関連ドキュメントの更新は難しい課題です。

- AIはコードの文脈は理解できるが、ドキュメント体系全体は把握していない
- どのドキュメントが「このコード」に関連するか判断できない
- ドキュメント更新の漏れが発生しやすい

---

## doc-tracerのアプローチ

### 核心的なアイデア: グラフによる関係性の明示

doc-tracerは、ドキュメントとコードの関係を**有向グラフ**として表現します。

<img width="1024" height="559" alt="image" src="https://github.com/user-attachments/assets/2cd74431-015d-4096-bd66-b63dca21ac69" />


**ノード**: ドキュメント、コンポーネント、API、DBテーブルなど
**エッジ**: contains（親子）、implements（実装）、uses（使用）、calls（呼び出し）

### 2つの情報源

1. **YAMLフロントマター（手動）**: ドキュメント間の階層関係、コードへの参照を明示的に記述
2. **静的解析（自動）**: コード内のimport文、API呼び出し、テンプレート使用を自動検出

この組み合わせにより、**ドキュメント → コード → ドキュメント** の双方向トレースが可能になります。

---

## 5層ドキュメント構造

doc-tracerは、ウォーターフォール型の5層ドキュメント構造を前提としています。

```
01_要件定義/        # WHY - なぜ作るのか
    └── 機能要件定義書.md
    └── 非機能要件定義書.md

02_仕様書/          # WHAT - 何を作るのか
    └── 認証仕様書.md
    └── モゲレコ借入仕様書.md

03_基本設計/        # HOW（概要）- どう作るか（全体像）
    └── 認証基本設計書.md
    └── API基本設計書.md

04_詳細設計/        # HOW（詳細）- どう作るか（詳細）
    └── ログイン詳細設計書.md

05_プログラム設計/  # HOW（実装）- 実際のコード構造
    └── LoginForm設計書.md
    └── 認証API設計書.md

src/                # 実装コード
    └── components/LoginForm.vue
    └── api/auth.ts
```

### 層間の関係

| 関係 | 意味 | 例 |
|------|------|-----|
| `contains` | 親 → 子（階層構造） | 基本設計 → 詳細設計 |
| `implements` | 下位 → 上位（実装関係） | プログラム設計 → 仕様書 |
| `uses` | 参照関係 | 設計書 → コンポーネント |

---

## YAMLフロントマターの設計思想

### なぜYAMLフロントマターなのか

1. **ドキュメントと一体化**: 関係性定義がドキュメント内にあるため、同期が取りやすい
2. **Git管理可能**: プレーンテキストなのでバージョン管理・差分確認が容易
3. **標準的な形式**: Jekyll、Hugo、Obsidianなど多くのツールで採用されている形式

### フロントマターの構造

```yaml
---
trace:
  # このドキュメントのID（ファイルパスから自動生成も可）
  id: doc:docs/03_基本設計/認証基本設計書.md

  # 親ドキュメント（このドキュメントを包含する上位層）
  parent:
    - doc:docs/02_仕様書/認証仕様書.md

  # 子ドキュメント（このドキュメントから派生する下位層）
  children:
    - doc:docs/04_詳細設計/ログイン詳細設計書.md
    - doc:docs/04_詳細設計/パスワードリセット詳細設計書.md

  # 実装元（このドキュメントが実装する上位要件）
  implements:
    - doc:docs/01_要件定義/機能要件定義書.md

  # 使用するコード要素
  uses:
    - component:LoginForm
    - api:/api/v1/auth/login
---
```

### parent vs children

- **parent**: 子ドキュメント側に書く（「私の親はこれ」）
- **children**: 親ドキュメント側に書く（「私の子はこれら」）

どちらを使っても同じエッジ（contains）が生成されます。プロジェクトの運用に合わせて選択してください。

推奨: 子ドキュメント側に `parent` を書く方が、ファイル追加時の更新箇所が少なくて済みます。

---

## ノードIDの命名規則

| プレフィックス | 対象 | 自動/手動 |
|--------------|------|----------|
| `doc:` | ドキュメント | 自動（スキャン時にパスから生成） |
| `component:` | Vueコンポーネント | 自動（.vueファイル名から生成） |
| `store:` | Piniaストア | 自動 |
| `composable:` | Vue Composable | 自動 |
| `api:` | APIエンドポイント | 自動（コード解析）/ 手動 |
| `controller:` | PHPコントローラー | 自動 |
| `model:` | Eloquentモデル | 自動 |
| `db:` | DBテーブル | 自動（モデルから抽出） |

---

## エッジ（関係）の種類

### contains（包含）

親子関係。階層構造を表現します。

```
親ドキュメント --contains--> 子ドキュメント
親コンポーネント --contains--> 子コンポーネント
```

生成条件:
- ドキュメントの `parent` / `children` 定義
- Vueテンプレート内のコンポーネント使用

### implements（実装）

下位層が上位層を実装する関係。

```
プログラム設計書 --implements--> 仕様書
```

生成条件:
- ドキュメントの `implements` 定義

### uses（使用）

参照・依存関係。

```
設計書 --uses--> コンポーネント
モデル --uses--> DBテーブル
```

生成条件:
- ドキュメントの `uses` 定義
- コード内のimport文

### calls（呼び出し）

API呼び出し関係。

```
コンポーネント --calls--> API
```

生成条件:
- コード内の `$fetch()`, `useFetch()`, `fetch()` 呼び出し

---

## 影響範囲の計算

doc-tracerの最も重要な機能は、**コード変更時の影響範囲計算**です。

### 上方向トレース

あるファイルを変更したとき、影響を受ける（更新が必要な）ドキュメントを特定します。

```
LoginForm.vue を変更
    ↓ (contains の逆方向)
LoginPage.vue
    ↓ (uses の逆方向)
ログイン詳細設計書.md
    ↓ (contains の逆方向)
認証基本設計書.md
    ↓ (contains の逆方向)
認証仕様書.md
```

この情報があれば、コード変更後に更新すべきドキュメントが明確になります。

---

## Claude Codeとの連携

### ドキュメント更新の自動化

doc-tracerは、Claude Codeがドキュメント更新を自動化するための基盤を提供します。

1. **影響範囲の特定**: `doc-tracer impact <file>` で更新すべきドキュメントを特定
2. **整合性チェック**: `doc-tracer check` でリンク切れ・孤立ドキュメントを検出
3. **可視化**: `doc-tracer serve` でグラフを確認し、構造を理解

### Claude Code向けのワークフロー

```bash
# 1. コード変更後、影響範囲を確認
doc-tracer impact src/components/LoginForm.vue

# 2. 影響を受けるドキュメントを更新（Claude Codeに依頼）
# → 詳細設計書、プログラム設計書を更新

# 3. 整合性チェック
doc-tracer check

# 4. コミット
git add . && git commit -m "feat: ログイン機能改善"
```

---

## データベース

SQLiteを使用。スキャン結果を永続化し、高速なクエリを実現します。

### テーブル構造

**nodes**: ノード（ドキュメント、コード要素）
```sql
CREATE TABLE nodes (
    id TEXT PRIMARY KEY,          -- 例: doc:docs/03_基本設計/認証設計書.md
    type TEXT NOT NULL,           -- document, component, api, etc.
    name TEXT NOT NULL,           -- 表示名
    file_path TEXT,               -- ファイルパス
    file_hash TEXT,               -- SHA256（変更検知用）
    updated_at DATETIME
);
```

**edges**: エッジ（関係）
```sql
CREATE TABLE edges (
    from_id TEXT NOT NULL,        -- 起点ノード
    to_id TEXT NOT NULL,          -- 終点ノード
    relation TEXT NOT NULL,       -- contains, implements, uses, calls
    source TEXT DEFAULT 'auto',   -- auto（自動検出）or manual（手動定義）
    defined_in TEXT,              -- 定義元ファイル
    UNIQUE(from_id, to_id, relation)
);
```

---

## コマンドリファレンス

### scan

```bash
doc-tracer scan [path] [--docs-only] [--code-only] [--db tracer.db]
```

### serve

```bash
doc-tracer serve [--port 8080] [--db tracer.db]
```

### check

```bash
doc-tracer check [--db tracer.db]
```

### impact

```bash
doc-tracer impact <file> [--json] [--db tracer.db]
```

---

## インストール

### go install（推奨）

```bash
go install github.com/tomohiro-owada/doc-tracer@latest
```

### ソースからビルド

```bash
git clone https://github.com/tomohiro-owada/doc-tracer.git
cd doc-tracer
go build -o doc-tracer .
```

---

## 対応言語

doc-tracerは以下の言語を自動スキャンします：

| 言語 | 拡張子 | 検出対象 |
|------|--------|----------|
| **JavaScript/TypeScript** | .js, .ts, .tsx | モジュール、composable、store |
| **Vue** | .vue | コンポーネント、API呼び出し |
| **PHP** | .php | コントローラー、モデル、APIルート |
| **Go** | .go | モジュール、関数、メソッド |
| **Python** | .py | クラス、関数、Flask/FastAPIルート |
| **Java** | .java | クラス、メソッド、Spring Bootエンドポイント |
| **Rust** | .rs | struct、関数、Actix-web/Axumルート |
| **Ruby** | .rb | クラス、メソッド |
| **C#** | .cs | クラス、メソッド、ASP.NETエンドポイント |
| **Kotlin** | .kt, .kts | クラス、関数、Spring Bootエンドポイント |
| **Swift** | .swift | クラス、struct、関数 |
| **Dart** | .dart | クラス、Widget、Screen、Provider/Bloc |

各パーサーはディレクトリ構造から適切なノードタイプ（controller, model, service, view等）を自動判定します。

---

## ライセンス

MIT License
