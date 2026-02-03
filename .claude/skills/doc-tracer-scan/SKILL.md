---
name: doc-tracer-scan
description: プロジェクトのドキュメントとコードをスキャンしてトレーサビリティグラフを構築する。新しいプロジェクトで最初に実行するか、大きな変更後に再スキャンする際に使用。
allowed-tools: Bash(doc-tracer:*), Read, Glob
---

# doc-tracer スキャン

プロジェクトのドキュメントとコードをスキャンして、トレーサビリティグラフを構築します。

## 実行手順

### 1. doc-tracerがインストールされているか確認

```bash
which doc-tracer || echo "doc-tracer not found"
```

インストールされていない場合:
```bash
cd /path/to/doc-tracer && go build -o doc-tracer . && sudo mv doc-tracer /usr/local/bin/
```

### 2. スキャン実行

引数 `$ARGUMENTS` で指定されたパスをスキャンします。

```bash
# ドキュメントとコードの両方をスキャン
doc-tracer scan $ARGUMENTS

# または個別にスキャン
doc-tracer scan $ARGUMENTS/docs --docs-only
doc-tracer scan $ARGUMENTS/src --code-only
```

### 3. 結果確認

```bash
doc-tracer check
```

### 4. 可視化（オプション）

```bash
doc-tracer serve --port 8080
```

ブラウザで http://localhost:8080 を開いてグラフを確認。

## 複数リポジトリのスキャン

モノレポや複数リポジトリ構成の場合:

```bash
# ドキュメントリポジトリ
doc-tracer scan /path/to/docs-repo --docs-only

# フロントエンドリポジトリ（シンボリックリンクの場合はrealpathで解決）
FRONT_PATH=$(realpath /path/to/frontend)
doc-tracer scan "$FRONT_PATH" --code-only

# バックエンドリポジトリ
BACK_PATH=$(realpath /path/to/backend)
doc-tracer scan "$BACK_PATH" --code-only
```

## 出力

- `tracer.db`: SQLiteデータベース（ノードとエッジを格納）
- スキャン完了メッセージ: `スキャン完了: ドキュメント X件, コード要素 Y件`

---

## ⚠️ 重要: スキャン後の確認は必須

### スキャン結果が0件の場合

```
スキャン完了: ドキュメント 0件, コード要素 0件
```

**原因**: `docs/` ディレクトリが存在しない

**解決策**:
```bash
# ディレクトリ構造を確認
ls -la docs/

# なければ作成
mkdir -p docs/{01_要件定義,02_仕様書,03_基本設計,04_詳細設計,05_プログラム設計}
```

### 必須: スキャン後はすぐにチェック

```bash
# この2コマンドは常にセットで実行
./doc-tracer scan . && ./doc-tracer check
```

チェックで問題が出たら**必ず解決してから次の作業に進む**。

### 必須: ビューワーで目視確認

```bash
./doc-tracer serve --port 8888
```

確認ポイント:
- [ ] 全ノードが接続されているか
- [ ] 孤立ノードがないか
- [ ] ドキュメント→コードの線が見えるか

### 完了条件

以下を全て満たすまでスキャン作業は「完了」ではない:

1. `./doc-tracer scan .` でドキュメント件数 > 0
2. `./doc-tracer scan .` でコード要素件数 > 0（コードがある場合）
3. `./doc-tracer check` で「問題なし」
4. ビューワーで孤立ノードがないことを確認
