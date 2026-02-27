package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/stevehiehn/declaragent/internal/mcp"
)

var (
	mcpPlansDir  string
	mcpTransport string
	mcpPort      int
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server (stdio or SSE transport)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		wd, _ := os.Getwd()
		switch mcpTransport {
		case "stdio":
			return mcp.ServeStdio(wd, mcpPlansDir)
		case "sse":
			return mcp.ServeSSE(mcpPort, wd, mcpPlansDir)
		default:
			return fmt.Errorf("unknown transport %q (must be stdio or sse)", mcpTransport)
		}
	},
}

func init() {
	mcpCmd.Flags().StringVar(&mcpPlansDir, "plans", "", "Directory containing plan YAML files to expose as tools")
	mcpCmd.Flags().StringVar(&mcpTransport, "transport", "stdio", "Transport mode: stdio or sse")
	mcpCmd.Flags().IntVar(&mcpPort, "port", 19100, "Port for SSE transport (default 19100)")
	rootCmd.AddCommand(mcpCmd)
}