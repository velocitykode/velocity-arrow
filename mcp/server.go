package mcp

import (
	"context"
	"fmt"

	"github.com/velocitykode/velocity-arrow/internal/embed"
	"github.com/velocitykode/velocity-arrow/internal/kb"
	"github.com/velocitykode/velocity-arrow/internal/store"
	"github.com/velocitykode/velocity-arrow/mcp/tools"
	"github.com/velocitykode/velocity-mcp/schema"
	"github.com/velocitykode/velocity-mcp/server"
	"github.com/velocitykode/velocity-mcp/transport"
)

// instructions is the server-level guidance handed to an MCP client: what the
// project-introspection tools cover, plus how to use the knowledge-base tools
// and what a knowledge-base miss does (and does not) mean.
const instructions = "Velocity framework MCP server. Provides tools for app introspection, database " +
	"access, route listing, documentation search, log reading, and configuration inspection. " +
	"It also guards the velocity framework knowledge base: call kb-guard before writing code in an " +
	"unfamiliar area, kb-symbol to verify an exact signature, and kb-search for intent lookups. " +
	"A miss means 'not in this KB', not 'not in the framework'. The knowledge base is a baked, " +
	"version-stamped snapshot of velocity core; read the kb://manifest resource for its coverage boundary."

// Serve starts the MCP server on stdio transport. When allowWrites is true
// the velocity_db_query tool accepts non-read-only SQL (INSERT/UPDATE/DELETE
// and DDL); otherwise it stays read-only.
func Serve(allowWrites bool) error {
	ctx := context.Background()

	kbStore, err := store.Open(ctx, kb.SnapshotDB, embed.New())
	if err != nil {
		return fmt.Errorf("opening knowledge-base snapshot: %w", err)
	}
	defer kbStore.Close()

	return transport.ServeStdio(ctx, newServer(allowWrites, kbStore))
}

func newServer(allowWrites bool, kbStore *store.Store) *server.Server {
	return server.New(
		"velocity-arrow",
		"0.1.0",
		server.WithInstructions(instructions),
		server.WithTools(registeredTools(allowWrites, kbStore)...),
		server.WithResources(tools.NewKBManifestResource(kbStore)),
	)
}

func registeredTools(allowWrites bool, kbStore *store.Store) []server.Tool {
	return []server.Tool{
		appInfoTool().HandleFunc(tools.HandleAppInfo),
		dbSchemaTool().HandleFunc(tools.HandleDBSchema),
		dbQueryTool(allowWrites).HandleFunc(tools.NewDBQueryHandler(allowWrites)),
		routesTool().HandleFunc(tools.HandleRoutes),
		searchDocsTool().HandleFunc(tools.HandleSearchDocs),
		lastErrorTool().HandleFunc(tools.HandleLastError),
		logEntriesTool().HandleFunc(tools.HandleLogEntries),
		configTool().HandleFunc(tools.HandleConfig),
		kbSearchTool().HandleFunc(tools.NewKBSearchHandler(kbStore)),
		kbSymbolTool().HandleFunc(tools.NewKBSymbolHandler(kbStore)),
		kbGuardTool().HandleFunc(tools.NewKBGuardHandler(kbStore)),
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

func dbQueryTool(allowWrites bool) *server.ToolBuilder {
	description := "Run a read-only SQL query against the application database. Only SELECT, SHOW, EXPLAIN, DESCRIBE, and WITH...SELECT are allowed."
	if allowWrites {
		description = "Run a SQL query against the application database. Writes are ENABLED: INSERT, UPDATE, DELETE, and DDL are permitted in addition to read-only queries. Use with care - statements are executed directly against the live database."
	}
	return server.NewTool("velocity_db_query", description).
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

func kbSearchTool() *server.ToolBuilder {
	return server.NewTool("kb-search",
		"Search the velocity framework knowledge base by intent (e.g. \"hash a password\", "+
			"\"rate-limit http\"). Returns ranked answer cards: the helper or rule, its signature, "+
			"a one-line how, and provenance. Call this BEFORE reaching for the standard library.").
		WithReadOnlyHint(true).
		WithSchema(func(s *schema.Object) {
			s.String("query").Required().Description("Intent or keywords to search for.").Min(1)
			s.Enum("kind", string(kb.KindRule), string(kb.KindSymbol), string(kb.KindHelper),
				string(kb.KindRecipe), string(kb.KindConcept)).
				Description("Optional: restrict to one kind of entry.")
			s.Integer("limit").Description("Max results (default 5).").Min(1).Max(25)
		})
}

func kbSymbolTool() *server.ToolBuilder {
	return server.NewTool("kb-symbol",
		"Look up the exact velocity API surface for a name (function, method, or type). "+
			"Returns the real signature, owning package, provenance, and deprecation status. "+
			"Use to verify a signature instead of guessing it.").
		WithReadOnlyHint(true).
		WithSchema(func(s *schema.Object) {
			s.String("name").Required().Description("Symbol name, optionally package-qualified.").Min(1)
		})
}

func kbGuardTool() *server.ToolBuilder {
	return server.NewTool("kb-guard",
		"Return curated guard rules and gotchas for a topic or velocity package "+
			"(e.g. \"logging\", \"validation\", \"http\"). These encode use-velocity-not-stdlib "+
			"mappings and known traps. Call this BEFORE writing code in an unfamiliar area.").
		WithReadOnlyHint(true).
		WithSchema(func(s *schema.Object) {
			s.String("topic").Description("Topic or package to guard (empty = highest-signal guards).")
		})
}
