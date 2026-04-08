package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/velocitykode/velocity-arrow/mcp/tools"
)

// Serve starts the MCP server on stdio transport.
func Serve() error {
	s := server.NewMCPServer(
		"velocity-arrow",
		"0.1.0",
		server.WithToolCapabilities(false),
		server.WithInstructions("Velocity framework MCP server. Provides tools for app introspection, database access, route listing, documentation search, log reading, and configuration inspection."),
	)

	registerTools(s)

	return server.ServeStdio(s)
}

func registerTools(s *server.MCPServer) {
	s.AddTool(appInfoTool(), tools.HandleAppInfo)
	s.AddTool(dbSchemaTool(), tools.HandleDBSchema)
	s.AddTool(dbQueryTool(), tools.HandleDBQuery)
	s.AddTool(routesTool(), tools.HandleRoutes)
	s.AddTool(searchDocsTool(), tools.HandleSearchDocs)
	s.AddTool(lastErrorTool(), tools.HandleLastError)
	s.AddTool(logEntriesTool(), tools.HandleLogEntries)
	s.AddTool(configTool(), tools.HandleConfig)
}

func appInfoTool() mcp.Tool {
	return mcp.NewTool("velocity_app_info",
		mcp.WithDescription("Get Velocity application info: Go version, Velocity version, dependencies, and registered providers."),
		mcp.WithReadOnlyHintAnnotation(true),
	)
}

func dbSchemaTool() mcp.Tool {
	return mcp.NewTool("velocity_db_schema",
		mcp.WithDescription("Explore the database schema. Use summary mode first, then request specific tables."),
		mcp.WithBoolean("summary",
			mcp.Description("When true, returns only table names and column types. Default: true."),
		),
		mcp.WithString("filter",
			mcp.Description("Filter tables by name (substring match)."),
		),
		mcp.WithString("database",
			mcp.Description("Database name override. Defaults to DB_DATABASE from .env."),
		),
		mcp.WithReadOnlyHintAnnotation(true),
	)
}

func dbQueryTool() mcp.Tool {
	return mcp.NewTool("velocity_db_query",
		mcp.WithDescription("Run a read-only SQL query against the application database. Only SELECT, SHOW, EXPLAIN, DESCRIBE, and WITH...SELECT are allowed."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The SQL query to execute."),
		),
		mcp.WithString("database",
			mcp.Description("Database name override. Defaults to DB_DATABASE from .env."),
		),
		mcp.WithReadOnlyHintAnnotation(true),
	)
}

func routesTool() mcp.Tool {
	return mcp.NewTool("velocity_routes",
		mcp.WithDescription("List registered routes by parsing route registration files. Returns method, path, handler, and middleware."),
		mcp.WithReadOnlyHintAnnotation(true),
	)
}

func searchDocsTool() mcp.Tool {
	return mcp.NewTool("velocity_search_docs",
		mcp.WithDescription("Search the embedded Velocity documentation."),
		mcp.WithArray("queries",
			mcp.Required(),
			mcp.Description("Search queries to run against the docs."),
			mcp.Items(map[string]any{"type": "string"}),
		),
		mcp.WithArray("packages",
			mcp.Description("Filter by package names (e.g., orm, cache, queue)."),
			mcp.Items(map[string]any{"type": "string"}),
		),
		mcp.WithNumber("token_limit",
			mcp.Description("Maximum tokens in the response. Default: 3000."),
		),
		mcp.WithReadOnlyHintAnnotation(true),
	)
}

func lastErrorTool() mcp.Tool {
	return mcp.NewTool("velocity_last_error",
		mcp.WithDescription("Get the last ERROR entry from the application log file."),
		mcp.WithReadOnlyHintAnnotation(true),
	)
}

func logEntriesTool() mcp.Tool {
	return mcp.NewTool("velocity_log_entries",
		mcp.WithDescription("Read the last N log entries from the application log file."),
		mcp.WithNumber("entries",
			mcp.Description("Number of entries to return. Default: 10."),
		),
		mcp.WithReadOnlyHintAnnotation(true),
	)
}

func configTool() mcp.Tool {
	return mcp.NewTool("velocity_config",
		mcp.WithDescription("Read configuration values from .env and config files."),
		mcp.WithString("key",
			mcp.Description("Specific config key to read (e.g., DB_CONNECTION, APP_ENV). Omit to get all non-secret values."),
		),
		mcp.WithReadOnlyHintAnnotation(true),
	)
}
