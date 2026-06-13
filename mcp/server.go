package mcp

import (
	"context"

	"github.com/velocitykode/velocity-arrow/mcp/tools"
	"github.com/velocitykode/velocity-mcp/schema"
	"github.com/velocitykode/velocity-mcp/server"
	"github.com/velocitykode/velocity-mcp/transport"
)

// Serve starts the MCP server on stdio transport.
func Serve() error {
	return transport.ServeStdio(context.Background(), newServer())
}

func newServer() *server.Server {
	return server.New(
		"velocity-arrow",
		"0.1.0",
		server.WithInstructions("Velocity framework MCP server. Provides tools for app introspection, database access, route listing, documentation search, log reading, and configuration inspection."),
		server.WithTools(registeredTools()...),
	)
}

func registeredTools() []server.Tool {
	return []server.Tool{
		appInfoTool().HandleFunc(tools.HandleAppInfo),
		dbSchemaTool().HandleFunc(tools.HandleDBSchema),
		dbQueryTool().HandleFunc(tools.HandleDBQuery),
		routesTool().HandleFunc(tools.HandleRoutes),
		searchDocsTool().HandleFunc(tools.HandleSearchDocs),
		lastErrorTool().HandleFunc(tools.HandleLastError),
		logEntriesTool().HandleFunc(tools.HandleLogEntries),
		configTool().HandleFunc(tools.HandleConfig),
	}
}

func appInfoTool() *server.ToolBuilder {
	return server.NewTool("velocity_app_info",
		"Get Velocity application info: Go version, Velocity version, dependencies, and registered providers.")
}

func dbSchemaTool() *server.ToolBuilder {
	return server.NewTool("velocity_db_schema",
		"Explore the database schema. Use summary mode first, then request specific tables.").
		WithSchema(func(s *schema.Object) {
			s.Boolean("summary").
				Description("When true, returns only table names and column types. Default: true.")
			s.String("filter").
				Description("Filter tables by name (substring match).")
			s.String("database").
				Description("Database name override. Defaults to DB_DATABASE from .env.")
		})
}

func dbQueryTool() *server.ToolBuilder {
	return server.NewTool("velocity_db_query",
		"Run a read-only SQL query against the application database. Only SELECT, SHOW, EXPLAIN, DESCRIBE, and WITH...SELECT are allowed.").
		WithSchema(func(s *schema.Object) {
			s.String("query").
				Required().
				Description("The SQL query to execute.")
			s.String("database").
				Description("Database name override. Defaults to DB_DATABASE from .env.")
		})
}

func routesTool() *server.ToolBuilder {
	return server.NewTool("velocity_routes",
		"List registered routes by parsing route registration files. Returns method, path, handler, and middleware.")
}

func searchDocsTool() *server.ToolBuilder {
	return server.NewTool("velocity_search_docs",
		"Search the embedded Velocity documentation.").
		WithSchema(func(s *schema.Object) {
			s.Array("queries").
				Required().
				Description("Search queries to run against the docs.").
				Items("string")
			s.Array("packages").
				Description("Filter by package names (e.g., orm, cache, queue).").
				Items("string")
			s.Number("token_limit").
				Description("Maximum tokens in the response. Default: 3000.")
		})
}

func lastErrorTool() *server.ToolBuilder {
	return server.NewTool("velocity_last_error",
		"Get the last ERROR entry from the application log file.")
}

func logEntriesTool() *server.ToolBuilder {
	return server.NewTool("velocity_log_entries",
		"Read the last N log entries from the application log file.").
		WithSchema(func(s *schema.Object) {
			s.Number("entries").
				Description("Number of entries to return. Default: 10.")
		})
}

func configTool() *server.ToolBuilder {
	return server.NewTool("velocity_config",
		"Read configuration values from .env and config files.").
		WithSchema(func(s *schema.Object) {
			s.String("key").
				Description("Specific config key to read (e.g., DB_CONNECTION, APP_ENV). Omit to get all non-secret values.")
		})
}
