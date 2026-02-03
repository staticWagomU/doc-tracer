---
trace:
  id: doc:docs/03_基本設計/CLI設計書.md
  parent:
    - doc:docs/02_仕様書/機能仕様書.md
  children:
    - doc:docs/04_詳細設計/コマンド詳細設計書.md
---

# doc-tracer CLI設計書

## 1. コマンド体系

```
doc-tracer
├── scan [path]       # スキャン
├── serve            # Web UI起動
├── check            # 整合性チェック
└── impact <file>    # 影響範囲検索
```

---

## 2. グローバルオプション

| オプション | デフォルト | 説明 |
|-----------|----------|------|
| `--db` | `tracer.db` | SQLiteデータベースのパス |

---

## 3. コマンド詳細

### 3.1 scan

```bash
doc-tracer scan [path] [flags]
```

| フラグ | 説明 |
|-------|------|
| `--docs-only` | ドキュメントのみスキャン |
| `--code-only` | コードのみスキャン |

**処理**:
1. 指定パスを再帰的に走査
2. `.md` ファイルのYAMLフロントマター解析
3. `.vue`, `.ts`, `.php` の静的解析
4. ノードとエッジをDBに保存

**出力**:
```
スキャン完了: ドキュメント X件, コード要素 Y件
```

### 3.2 serve

```bash
doc-tracer serve [flags]
```

| フラグ | デフォルト | 説明 |
|-------|----------|------|
| `--port` | `8080` | ポート番号 |

**処理**:
1. HTTPサーバー起動
2. `/api/graph` エンドポイント提供
3. `/api/impact` エンドポイント提供
4. 静的ファイル（埋め込みHTML）配信

**出力**:
```
🌐 Web UI起動: http://localhost:8080
終了するには Ctrl+C を押してください
```

### 3.3 check

```bash
doc-tracer check
```

**処理**:
1. リンク切れチェック
2. 孤立ドキュメントチェック
3. 統計情報表示

**出力**:
```
⚠️  リンク切れ (X件):
  - from_id -> to_id (relation)

⚠️  孤立ドキュメント (Y件):
  - name
    パス: file_path

📊 統計情報:
  ノード:
    - document: N
    - component: M
  エッジ:
    - contains: N
    - uses: M

✅ 整合性チェック完了: 問題なし
```

### 3.4 impact

```bash
doc-tracer impact <file> [flags]
```

| フラグ | 説明 |
|-------|------|
| `--json` | JSON形式で出力 |

**処理**:
1. 指定ファイルのノードを特定
2. 再帰CTEで上方向トレース
3. 影響を受けるドキュメントを表示

**出力**:
```
影響を受けるドキュメント (X件):
  - name
    パス: file_path
```

---

## 4. 終了コード

| コード | 意味 |
|-------|------|
| 0 | 正常終了 |
| 1 | エラー（check失敗含む） |
