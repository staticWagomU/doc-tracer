---
trace:
  id: doc:docs/05_プログラム設計/serveコマンド設計書.md
  parent:
    - doc:docs/04_詳細設計/コマンド詳細設計書.md
  uses:
    - module:serve
---

# serve コマンド プログラム設計書

## 1. ファイル構成

```
cmd/serve.go
cmd/web/index.html
```

---

## 2. 主要関数

### 2.1 runServe

```go
func runServe(cmd *cobra.Command, args []string) error
```

**責務**: Webサーバー起動

**処理**:
1. DB接続
2. APIハンドラ登録
3. 静的ファイルハンドラ登録
4. サーバー起動

### 2.2 handleGraph

```go
func handleGraph(w http.ResponseWriter, r *http.Request, database *db.DB)
```

**責務**: `/api/graph` エンドポイント

**処理**:
1. `GetAllNodes()` でノード取得
2. `GetAllEdges()` でエッジ取得
3. JSON返却

### 2.3 handleImpact

```go
func handleImpact(w http.ResponseWriter, r *http.Request, database *db.DB)
```

**責務**: `/api/impact` エンドポイント

**処理**:
1. クエリパラメータ `file` を取得
2. `GetImpactedDocs()` で影響ドキュメント取得
3. JSON返却

---

## 3. 静的ファイル埋め込み

```go
//go:embed web/*
var webFS embed.FS
```

`cmd/web/` 配下のファイルをバイナリに埋め込み。

---

## 4. Web UI (index.html)

### 4.1 データ取得

```javascript
async function loadData() {
    const response = await fetch('/api/graph');
    graphData = await response.json();
    renderGraph();
}
```

### 4.2 グラフ描画

```javascript
simulation = d3.forceSimulation(graphData.nodes)
    .force('link', d3.forceLink(links).id(d => d.id).distance(100))
    .force('charge', d3.forceManyBody().strength(-200))
    .force('center', d3.forceCenter(width / 2, height / 2))
    .force('collision', d3.forceCollide().radius(30));
```

### 4.3 ノード選択時の上流ハイライト

```javascript
async function selectNode(event, d) {
    // BFSで上流方向のみ探索
    const upstreamMap = new Map();
    // to_id → from_id の方向でマップ構築

    const queue = [d.id];
    while (queue.length > 0) {
        const currentId = queue.shift();
        const upstreams = upstreamMap.get(currentId) || [];
        for (const { node: upstreamId, link } of upstreams) {
            connectedLinks.add(link);
            if (!connectedIds.has(upstreamId)) {
                connectedIds.add(upstreamId);
                queue.push(upstreamId);
            }
        }
    }
}
```

### 4.4 検索機能

```javascript
document.getElementById('search').addEventListener('input', (e) => {
    const query = e.target.value.toLowerCase();
    d3.selectAll('.node').each(function(d) {
        const matches = d.name.toLowerCase().includes(query) ||
                       (d.file_path && d.file_path.toLowerCase().includes(query));
        d3.select(this)
            .classed('dimmed', query && !matches)
            .classed('highlighted', query && matches);
    });
});
```
