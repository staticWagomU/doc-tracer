package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "doc-tracer",
	Short: "ドキュメントトレーサビリティ管理ツール",
	Long: `doc-tracer はコードとドキュメント間のトレーサビリティを管理するCLIツールです。

コード変更時に影響を受けるドキュメントを特定し、
ドキュメント更新の抜け漏れを防ぎます。

使用例:
  doc-tracer scan              # コードベースをスキャン
  doc-tracer impact <file>     # 影響範囲を検索
  doc-tracer check             # 整合性チェック
  doc-tracer serve             # Web UI起動`,
}

var dbPath string

func init() {
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "tracer.db", "SQLiteデータベースのパス")
}

func Execute() error {
	return rootCmd.Execute()
}
