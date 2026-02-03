package cmd

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"

	"doc-tracer/internal/db"

	"github.com/spf13/cobra"
)

//go:embed web/*
var webFS embed.FS

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "可視化Web UIを起動",
	Long: `トレーサビリティグラフを可視化するWeb UIを起動します。

例:
  doc-tracer serve              # デフォルトポート8080
  doc-tracer serve --port 3000  # ポート指定`,
	RunE: runServe,
}

var port int

func init() {
	serveCmd.Flags().IntVar(&port, "port", 8080, "ポート番号")
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("DB接続エラー: %w", err)
	}
	defer database.Close()

	// API エンドポイント
	http.HandleFunc("/api/graph", func(w http.ResponseWriter, r *http.Request) {
		handleGraph(w, r, database)
	})

	http.HandleFunc("/api/impact", func(w http.ResponseWriter, r *http.Request) {
		handleImpact(w, r, database)
	})

	// 静的ファイル（埋め込み）
	webContent, err := fs.Sub(webFS, "web")
	if err != nil {
		return err
	}
	http.Handle("/", http.FileServer(http.FS(webContent)))

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("🌐 Web UI起動: http://localhost%s\n", addr)
	fmt.Println("終了するには Ctrl+C を押してください")

	return http.ListenAndServe(addr, nil)
}

type GraphResponse struct {
	Nodes []db.Node `json:"nodes"`
	Edges []db.Edge `json:"edges"`
}

func handleGraph(w http.ResponseWriter, r *http.Request, database *db.DB) {
	w.Header().Set("Content-Type", "application/json")

	nodes, err := database.GetAllNodes()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	edges, err := database.GetAllEdges()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := GraphResponse{
		Nodes: nodes,
		Edges: edges,
	}

	json.NewEncoder(w).Encode(resp)
}

func handleImpact(w http.ResponseWriter, r *http.Request, database *db.DB) {
	w.Header().Set("Content-Type", "application/json")

	filePath := r.URL.Query().Get("file")
	if filePath == "" {
		http.Error(w, "file parameter required", http.StatusBadRequest)
		return
	}

	docs, err := database.GetImpactedDocs(filePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(docs)
}
