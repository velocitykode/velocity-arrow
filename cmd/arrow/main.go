package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/velocitykode/velocity-arrow/install"
	mcpserver "github.com/velocitykode/velocity-arrow/mcp"
)

func main() {
	root := &cobra.Command{
		Use:   "arrow",
		Short: "Velocity-focused MCP server and AI assistant installer",
	}

	root.AddCommand(mcpCmd())
	root.AddCommand(installCmd())

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

func installCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install Arrow into a Velocity project",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting working directory: %w", err)
			}
			return install.Run(dir)
		},
	}
	return cmd
}
