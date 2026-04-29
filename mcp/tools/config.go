package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/joho/godotenv"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/velocitykode/velocity"
)

// HandleConfig reads configuration values from .env and config files.
func HandleConfig(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	key := request.GetString("key", "")

	config := velocity.ConfigFromEnv()

	if key != "" {
		return handleSpecificKey(key, config)
	}

	return handleAllConfig(config)
}

func handleSpecificKey(key string, config velocity.Config) (*mcp.CallToolResult, error) {
	if isSecretKey(key) {
		return mcp.NewToolResultText(fmt.Sprintf("`%s` = [REDACTED] (secret value)", key)), nil
	}

	// Read from .env for the raw value
	dir, _ := os.Getwd()
	env, err := godotenv.Read(filepath.Join(dir, ".env"))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("reading .env: %v", err)), nil
	}

	val, ok := env[key]
	if !ok {
		val = os.Getenv(key)
		if val == "" {
			return mcp.NewToolResultText(fmt.Sprintf("Key `%s` not found in .env or environment.", key)), nil
		}
	}

	return mcp.NewToolResultText(fmt.Sprintf("`%s` = `%s`", key, val)), nil
}

func handleAllConfig(config velocity.Config) (*mcp.CallToolResult, error) {
	dir, _ := os.Getwd()
	env, err := godotenv.Read(filepath.Join(dir, ".env"))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("reading .env: %v", err)), nil
	}

	var b strings.Builder
	b.WriteString("# Configuration\n\n")

	// Show parsed config summary
	b.WriteString("## Application\n")
	b.WriteString(fmt.Sprintf("- Name: %s\n", os.Getenv("APP_NAME")))
	b.WriteString(fmt.Sprintf("- Environment: %s\n", config.Env))
	b.WriteString(fmt.Sprintf("- Debug: %t\n", config.Debug))
	b.WriteString(fmt.Sprintf("- Port: %s\n", config.Port))
	b.WriteString("\n")

	b.WriteString("## Database\n")
	b.WriteString(fmt.Sprintf("- Driver: %s\n", config.DB.Connection))
	b.WriteString(fmt.Sprintf("- Host: %s\n", config.DB.Host))
	b.WriteString(fmt.Sprintf("- Database: %s\n", config.DB.Database))
	b.WriteString("\n")

	// Show all raw .env values (non-secret)
	b.WriteString("## Raw .env Values\n\n")
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	currentGroup := ""
	for _, k := range keys {
		parts := strings.SplitN(k, "_", 2)
		group := parts[0]
		if group != currentGroup {
			if currentGroup != "" {
				b.WriteString("\n")
			}
			currentGroup = group
		}

		v := env[k]
		if isSecretKey(k) {
			v = "[REDACTED]"
		}
		b.WriteString(fmt.Sprintf("- `%s` = `%s`\n", k, v))
	}

	return mcp.NewToolResultText(b.String()), nil
}
