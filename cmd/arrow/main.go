package main

import (
	"os"

	"github.com/spf13/cobra"
	mcpserver "github.com/velocitykode/velocity-arrow/mcp"
)

func main() {
	root := &cobra.Command{
		Use:   "arrow",
		Short: "Velocity-focused MCP server",
	}

	root.AddCommand(mcpCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func mcpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Start the MCP server (stdio transport)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return mcpserver.Serve()
		},
	}
}
