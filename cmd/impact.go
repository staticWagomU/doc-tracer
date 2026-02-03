package cmd

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/tomohiro-owada/doc-tracer/internal/db"

	"github.com/spf13/cobra"
)

var impactCmd = &cobra.Command{
	Use:   "impact [file]",
	Short: "ファイル変更の影響範囲を検索",
	Long: `指定されたファイルを変更した場合に影響を受けるドキュメントを表示します。

例:
  doc-tracer impact src/components/LoanForm.vue
  doc-tracer impact --json src/api/loan.ts
  doc-tracer impact --staged  # git stagingされたファイルの影響範囲を表示`,
	Args: cobra.MaximumNArgs(1),
	RunE: runImpact,
}

var outputJSON bool
var useStaged bool

func init() {
	impactCmd.Flags().BoolVar(&outputJSON, "json", false, "JSON形式で出力")
	impactCmd.Flags().BoolVar(&useStaged, "staged", false, "git stagingされたファイルの影響範囲を検索")
	rootCmd.AddCommand(impactCmd)
}

func runImpact(cmd *cobra.Command, args []string) error {
	var filePaths []string

	if useStaged {
		// git diff --staged --name-only を実行
		gitCmd := exec.Command("git", "diff", "--staged", "--name-only")
		output, err := gitCmd.Output()
		if err != nil {
			return fmt.Errorf("git diff実行エラー: %w", err)
		}
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			if line != "" {
				filePaths = append(filePaths, line)
			}
		}
		if len(filePaths) == 0 {
			fmt.Println("ステージングされたファイルがありません")
			return nil
		}
		fmt.Printf("ステージングされたファイル (%d件):\n", len(filePaths))
		for _, fp := range filePaths {
			fmt.Printf("  - %s\n", fp)
		}
		fmt.Println()
	} else {
		if len(args) == 0 {
			return fmt.Errorf("ファイルパスを指定するか --staged フラグを使用してください")
		}
		filePaths = []string{args[0]}
	}

	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("DB接続エラー: %w", err)
	}
	defer database.Close()

	// 全ファイルの影響を集約（重複除去）
	docMap := make(map[string]db.Node)
	for _, filePath := range filePaths {
		docs, err := database.GetImpactedDocs(filePath)
		if err != nil {
			return fmt.Errorf("影響範囲検索エラー (%s): %w", filePath, err)
		}
		for _, doc := range docs {
			docMap[doc.ID] = doc
		}
	}

	// マップからスライスに変換
	var allDocs []db.Node
	for _, doc := range docMap {
		allDocs = append(allDocs, doc)
	}

	if len(allDocs) == 0 {
		fmt.Println("影響を受けるドキュメントはありません")
		return nil
	}

	if outputJSON {
		output, err := json.MarshalIndent(allDocs, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(output))
	} else {
		fmt.Printf("影響を受けるドキュメント (%d件):\n\n", len(allDocs))
		for _, doc := range allDocs {
			fmt.Printf("  - %s\n    パス: %s\n\n", doc.Name, doc.FilePath)
		}
	}

	return nil
}
