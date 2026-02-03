---
title: "AIがコードを書く時代、人はドキュメントだけレビューすればいい？ → ドキュメント更新も漏れるじゃん"
emoji: "📝"
type: "tech"
topics: ["claude", "documentation", "go", "cli", "開発効率化"]
published: true
---

## TL;DR

- Claude Codeにコードを書かせてレビューは人がやる、という運用をしている
- でも「ドキュメント更新」をAIに任せると**抜け漏れが発生する**
- コード変更時に「どのドキュメントを更新すべきか」を**自動で特定するツール**を作った
- https://github.com/tomohiro-owada/doc-tracer

---

## 問題: AIでもドキュメント更新は完璧にできない

最近、Claude Codeを使った開発フローを試している。

```
1. 要件を伝える
2. Claude Codeがコードを書く
3. 人がレビューする
4. マージ
```

「コードを書くのはAI、レビューするのは人」という分業。効率的に見える。

### でも、ドキュメントはどうする？

コードを変更したら、関連するドキュメントも更新が必要。

- 仕様書
- 設計書
- API仕様

Claude Codeに「ドキュメントも更新して」と頼むと、一応やってくれる。でも**抜け漏れが発生する**。

なぜか？

**AIはコードの文脈は理解できるが、ドキュメント体系全体を把握していない。**

「このコードを変更したら、どのドキュメントに影響するか」を判断するには、ドキュメント間の関係性を知っている必要がある。でもAIは毎回それを推測するしかない。

---

## 解決策: コードとドキュメントの関係をグラフで管理する

doc-tracerを作った。

```bash
go install github.com/tomohiro-owada/doc-tracer@latest
```

### やること

1. **ドキュメントにメタデータを埋め込む**（YAMLフロントマター）
2. **コードを自動スキャンして関係を抽出**
3. **グラフとして可視化・検索可能にする**

### ドキュメントのメタデータ

```yaml
---
trace:
  id: doc:docs/05_プログラム設計/LoginForm設計書.md
  parent:
    - doc:docs/04_詳細設計/ログイン詳細設計書.md
  uses:
    - component:LoginForm
    - api:/api/v1/auth/login
---

# LoginForm プログラム設計書

...
```

これで「このドキュメントは何を参照しているか」が明示される。

### コードの自動スキャン

```bash
doc-tracer scan .
```

Vue、TypeScript、Go、Python、Java、Rust など13言語に対応。

- コンポーネントの親子関係
- API呼び出し
- import文
- ルート定義（Flask、Spring Boot、Actix-web等）

これらを自動で抽出してグラフに追加。

---

## 核心機能: 影響範囲の逆引き

```bash
doc-tracer impact src/components/LoginForm.vue
```

出力:

```
影響を受けるドキュメント (5件):

  - LoginForm設計書.md
    パス: docs/05_プログラム設計/LoginForm設計書.md

  - ログイン詳細設計書.md
    パス: docs/04_詳細設計/ログイン詳細設計書.md

  - 認証基本設計書.md
    パス: docs/03_基本設計/認証基本設計書.md

  - 認証仕様書.md
    パス: docs/02_仕様書/認証仕様書.md

  - 機能要件定義書.md
    パス: docs/01_要件定義/機能要件定義書.md
```

**コードを変更したら、どのドキュメントを更新すべきか一発で分かる。**

これをClaude Codeに渡せば、更新すべきドキュメントを漏れなく更新できる。

---

## 可視化

```bash
doc-tracer serve
```

![doc-tracer visualization](https://github.com/user-attachments/assets/160b1347-790c-49a4-b1b7-f0a1e7ee6b27)

D3.jsでグラフを可視化。ノードをクリックすると上流のドキュメントがハイライトされる。

---

## 整合性チェック

```bash
doc-tracer check
```

```
📊 統計情報:

  ノード:
    - document: 12
    - module: 7
    - function: 10

  エッジ:
    - contains: 21
    - uses: 7

✅ 整合性チェック完了: 問題なし
```

- **リンク切れ検出**: 存在しないノードへの参照
- **孤立ドキュメント検出**: どこからも参照されていないドキュメント

---

## 5層ドキュメント構造

doc-tracerはウォーターフォール型の5層構造を想定している。

```
01_要件定義/     # WHY - なぜ作るのか
02_仕様書/       # WHAT - 何を作るのか
03_基本設計/     # HOW（概要）
04_詳細設計/     # HOW（詳細）
05_プログラム設計/ # 実装設計 → コードへの参照
```

下位層から上位層への参照を辿ることで、コード変更の影響範囲を特定できる。

---

## 対応言語

| 言語 | 検出対象 |
|------|----------|
| JavaScript/TypeScript | モジュール、composable、store |
| Vue | コンポーネント、API呼び出し |
| Go | モジュール、関数、メソッド |
| Python | クラス、関数、Flask/FastAPIルート |
| Java | クラス、メソッド、Spring Bootエンドポイント |
| Rust | struct、関数、Actix-web/Axumルート |
| Ruby | クラス、メソッド |
| C# | クラス、メソッド、ASP.NETエンドポイント |
| Kotlin | クラス、関数、Spring Bootエンドポイント |
| Swift | クラス、struct、関数 |
| Dart | クラス、Widget、Provider/Bloc |
| PHP | コントローラー、モデル、Laravelルート |

---

## 使い方

### 1. インストール

```bash
go install github.com/tomohiro-owada/doc-tracer@latest
```

または[Releases](https://github.com/tomohiro-owada/doc-tracer/releases)からバイナリをダウンロード。

### 2. ドキュメントにフロントマターを追加

```yaml
---
trace:
  parent:
    - doc:docs/上位層/親ドキュメント.md
  uses:
    - module:xxx
---
```

### 3. スキャン

```bash
doc-tracer scan .
```

### 4. 影響範囲を確認

```bash
doc-tracer impact path/to/changed/file.py
```

### 5. 整合性チェック

```bash
doc-tracer check
```

---

## Claude Codeとの連携（スキル化済み）

Claude Code向けのスキルとして統合済み。コミット前に自動で影響範囲をチェックできる。

```bash
# ステージングしたファイルの影響範囲を一発確認
git add src/components/LoginForm.vue
doc-tracer impact --staged

# 出力例:
# ステージングされたファイル (1件):
#   - src/components/LoginForm.vue
#
# 影響を受けるドキュメント (5件):
#   - LoginForm設計書.md
#   - ログイン詳細設計書.md
#   - 認証基本設計書.md
#   ...

# 整合性チェック
doc-tracer check
```

Claude Codeに「このドキュメントを更新して」と依頼すれば、漏れなく更新できる。

---

## 今後の改良予定

- ~~**git diff連携**: `doc-tracer impact --staged`~~ → 完了！
- ~~**AIとの統合**: Claude Code skill化~~ → 完了！

全部できた！

---

## まとめ

「AIにコードを書かせてレビューは人」という運用は効率的。でもドキュメント更新はAIでも漏れる。

doc-tracerは:

1. **コードとドキュメントの関係を明示化**
2. **変更時の影響範囲を自動特定**
3. **整合性を強制チェック**

これで「ドキュメント更新忘れた」を防ぐ。

リポジトリ: https://github.com/tomohiro-owada/doc-tracer

フィードバック歓迎！
