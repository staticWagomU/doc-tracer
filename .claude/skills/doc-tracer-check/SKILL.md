---
name: doc-tracer-check
description: ドキュメントとコードのトレーサビリティ整合性をチェックする。リンク切れ、孤立ドキュメント、構造の問題を検出して修正案を提示。
allowed-tools: Bash(doc-tracer:*), Read, Edit, Glob, Grep
---

# トレーサビリティ整合性チェック

ドキュメントとコードの整合性をチェックし、問題があれば修正します。

## チェック実行

```bash
doc-tracer check
```

## 検出される問題と対処法

### 1. リンク切れ（Broken Links）

**症状**: エッジの参照先ノードが存在しない

```
⚠️  リンク切れ (3件):

  - doc:docs/03_基本設計/認証設計書.md -> component:LoginForm (uses)
    定義元: docs/03_基本設計/認証設計書.md
```

**原因**:
- コンポーネント名が変更された
- ファイルが削除された
- IDのタイプミス

**対処**:
1. 参照先の正しいIDを確認

```bash
# コンポーネントを探す
doc-tracer serve  # Web UIで検索
# または
grep -r "LoginForm" --include="*.vue"
```

2. フロントマターを修正

```yaml
uses:
  - component:LoginFormNew  # 正しい名前に修正
```

### 2. 孤立ドキュメント（Orphaned Documents）

**症状**: どこからも参照されていないドキュメント

```
⚠️  孤立ドキュメント（参照なし）(2件):

  - 新機能設計書.md
    パス: docs/03_基本設計/新機能設計書.md
```

**原因**:
- 新規作成したがparentを設定していない
- 親ドキュメントのchildrenに追加していない

**対処**:
1. 適切な親ドキュメントを特定

```bash
# 同じ機能領域の上位層を探す
ls docs/02_仕様書/
```

2. フロントマターにparentを追加

```yaml
---
trace:
  parent:
    - doc:docs/02_仕様書/該当仕様書.md
---
```

### 3. 循環参照（設計上は許可しているが注意）

**症状**: A → B → C → A のような循環

**対処**: 通常は問題ないが、意図しない場合は関係を見直す

## 統計情報の確認

checkコマンドは統計情報も表示:

```
📊 統計情報:

  ノード:
    - document: 150
    - component: 80
    - controller: 30
    - model: 25

  エッジ:
    - contains: 200
    - uses: 150
    - calls: 50
```

**確認ポイント**:
- documentの数がdocsフォルダのmd数と一致するか
- componentの数が.vueファイル数と一致するか
- エッジの数が妥当か（孤立が多すぎないか）

## 問題の一括修正

### 全ドキュメントにparentを追加するパターン

```bash
# 03_基本設計のドキュメント一覧
find docs/03_基本設計 -name "*.md" -type f

# 各ファイルのフロントマターを確認
for f in docs/03_基本設計/*.md; do
  echo "=== $f ==="
  head -20 "$f"
done
```

### リンク切れの一括検出

```bash
# uses/implementsの参照先を抽出
grep -r "uses:" docs --include="*.md" -A 10 | grep "component:"
```

## チェック後のアクション

1. **問題なしの場合**:
   ```
   ✅ 整合性チェック完了: 問題なし
   ```
   → そのままコミット可能

2. **問題ありの場合**:
   ```
   整合性チェックで問題が見つかりました
   ```
   → 上記の対処法で修正後、再チェック

## CI/CD連携

```yaml
# GitHub Actions例
- name: Check documentation consistency
  run: |
    doc-tracer scan .
    doc-tracer check
  # 問題があればCI失敗
```
