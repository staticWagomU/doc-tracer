package cmd

import (
	"fmt"

	"github.com/tomohiro-owada/doc-tracer/internal/db"

	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "ドキュメントとコードの整合性をチェック",
	Long: `トレーサビリティDBの整合性をチェックします。

チェック項目:
  - リンク切れ（存在しないノードへの参照）
  - 孤立ドキュメント（どこからも参照されていない）

例:
  doc-tracer check`,
	RunE: runCheck,
}

func init() {
	rootCmd.AddCommand(checkCmd)
}

func runCheck(cmd *cobra.Command, args []string) error {
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("DB接続エラー: %w", err)
	}
	defer database.Close()

	hasIssues := false

	// リンク切れチェック
	brokenLinks, err := database.CheckBrokenLinks()
	if err != nil {
		return fmt.Errorf("リンク切れチェックエラー: %w", err)
	}

	if len(brokenLinks) > 0 {
		hasIssues = true
		fmt.Printf("⚠️  リンク切れ (%d件):\n\n", len(brokenLinks))
		for _, edge := range brokenLinks {
			fmt.Printf("  - %s -> %s (%s)\n", edge.FromID, edge.ToID, edge.Relation)
			fmt.Printf("    定義元: %s\n\n", edge.DefinedIn)
		}
	}

	// 孤立ドキュメントチェック
	orphans, err := database.GetOrphanedDocs()
	if err != nil {
		return fmt.Errorf("孤立ドキュメントチェックエラー: %w", err)
	}

	if len(orphans) > 0 {
		hasIssues = true
		fmt.Printf("⚠️  孤立ドキュメント（参照なし）(%d件):\n\n", len(orphans))
		for _, doc := range orphans {
			fmt.Printf("  - %s\n    パス: %s\n\n", doc.Name, doc.FilePath)
		}
	}

	// 統計情報
	nodes, err := database.GetAllNodes()
	if err != nil {
		return err
	}

	edges, err := database.GetAllEdges()
	if err != nil {
		return err
	}

	// タイプ別集計
	typeCounts := make(map[string]int)
	for _, node := range nodes {
		typeCounts[node.Type]++
	}

	relationCounts := make(map[string]int)
	for _, edge := range edges {
		relationCounts[edge.Relation]++
	}

	fmt.Println("📊 統計情報:")
	fmt.Println()
	fmt.Println("  ノード:")
	for t, count := range typeCounts {
		fmt.Printf("    - %s: %d\n", t, count)
	}
	fmt.Println()
	fmt.Println("  エッジ:")
	for r, count := range relationCounts {
		fmt.Printf("    - %s: %d\n", r, count)
	}
	fmt.Println()

	if hasIssues {
		return fmt.Errorf("整合性チェックで問題が見つかりました")
	}

	fmt.Println("✅ 整合性チェック完了: 問題なし")
	return nil
}
