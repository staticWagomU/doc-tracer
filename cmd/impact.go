package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/tomohiro-owada/doc-tracer/internal/db"

	"github.com/spf13/cobra"
)

var impactCmd = &cobra.Command{
	Use:   "impact <file>",
	Short: "ファイル変更の影響範囲を検索",
	Long: `指定されたファイルを変更した場合に影響を受けるドキュメントを表示します。

例:
  doc-tracer impact src/components/LoanForm.vue
  doc-tracer impact --json src/api/loan.ts`,
	Args: cobra.ExactArgs(1),
	RunE: runImpact,
}

var outputJSON bool

func init() {
	impactCmd.Flags().BoolVar(&outputJSON, "json", false, "JSON形式で出力")
	rootCmd.AddCommand(impactCmd)
}

func runImpact(cmd *cobra.Command, args []string) error {
	filePath := args[0]

	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("DB接続エラー: %w", err)
	}
	defer database.Close()

	docs, err := database.GetImpactedDocs(filePath)
	if err != nil {
		return fmt.Errorf("影響範囲検索エラー: %w", err)
	}

	if len(docs) == 0 {
		fmt.Println("影響を受けるドキュメントはありません")
		return nil
	}

	if outputJSON {
		output, err := json.MarshalIndent(docs, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(output))
	} else {
		fmt.Printf("影響を受けるドキュメント (%d件):\n\n", len(docs))
		for _, doc := range docs {
			fmt.Printf("  - %s\n    パス: %s\n\n", doc.Name, doc.FilePath)
		}
	}

	return nil
}
