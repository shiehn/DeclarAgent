package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/stevehiehn/declaragent/internal/mcp"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP stdio server",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		wd, _ := os.Getwd()
		return mcp.Serve(wd)
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
