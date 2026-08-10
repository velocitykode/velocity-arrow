package main

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	mcpserver "github.com/velocitykode/velocity-arrow/mcp"

	// Register all ORM drivers (postgres, mysql, sqlite) so db_query/db_schema
	// work against any project, not just sqlite. Without this only sqlite is
	// registered and querying a postgres/mysql project fails with
	// `driver "postgres" not registered`.
	_ "github.com/velocitykode/velocity/orm/standard"
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
	var allowWrites bool
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start the MCP server (stdio transport)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !allowWrites && envTrue(os.Getenv("ARROW_ALLOW_WRITES")) {
				allowWrites = true
			}
			return mcpserver.Serve(allowWrites)
		},
	}
	cmd.Flags().BoolVar(&allowWrites, "allow-writes", false,
		"Allow non-read-only SQL (INSERT/UPDATE/DELETE/DDL) via velocity_db_query. DANGEROUS: enables writes to the live application database. Can also be set with ARROW_ALLOW_WRITES=1.")
	return cmd
}

func envTrue(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
