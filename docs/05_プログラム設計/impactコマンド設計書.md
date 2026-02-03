---
trace:
  id: doc:docs/05_プログラム設計/impactコマンド設計書.md
  parent:
    - doc:docs/04_詳細設計/コマンド詳細設計書.md
  uses:
    - module:impact
---

# impact コマンド プログラム設計書

## 1. ファイル構成

```
cmd/impact.go
```

---

## 2. 主要関数

### 2.1 runImpact

```go
func runImpact(cmd *cobra.Command, args []string) error
```

**責務**: 影響範囲検索の実行

**処理**:
1. ファイルパス引数を取得
2. DB接続
3. `GetImpactedDocs()` 呼び出し
4. 結果表示（テキスト or JSON）

---

## 3. 出力フォーマット

### 3.1 テキスト形式（デフォルト）

```
影響を受けるドキュメント (N件):

  - name
    パス: file_path

```

### 3.2 JSON形式（`--json`）

```json
[
  {
    "id": "doc:docs/03_基本設計/xxx.md",
    "type": "document",
    "name": "xxx.md",
    "file_path": "docs/03_基本設計/xxx.md"
  }
]
```

### 3.3 影響なし

```
影響を受けるドキュメントはありません
```

---

## 4. DBクエリ（再帰CTE）

```sql
WITH RECURSIVE impact_chain AS (
    -- 起点: 指定ファイルに含まれるノード
    SELECT n.id, n.type, n.name, n.file_path, 0 as depth
    FROM nodes n
    WHERE n.file_path = ?

    UNION

    -- 再帰: このノードを参照しているノードを辿る
    SELECT n.id, n.type, n.name, n.file_path, ic.depth + 1
    FROM nodes n
    JOIN edges e ON e.from_id = n.id
    JOIN impact_chain ic ON e.to_id = ic.id
    WHERE ic.depth < 10
)
SELECT DISTINCT id, type, name, file_path
FROM impact_chain
WHERE type = 'document'
ORDER BY depth
```

**ポイント**:
- `e.from_id = n.id` と `e.to_id = ic.id` で**逆方向**に辿る
- `depth < 10` で無限ループ防止
- `type = 'document'` でドキュメントのみ抽出
