package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func newTestRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "arrow",
		Short: "Velocity-focused MCP server",
	}
	root.AddCommand(mcpCmd())
	return root
}

func TestRootCmd_Help(t *testing.T) {
	root := newTestRoot()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"--help"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("root --help errored: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "mcp") {
		t.Error("help should list mcp subcommand")
	}
}

func TestMcpCmd_Builds(t *testing.T) {
	cmd := mcpCmd()
	if cmd.Use != "mcp" {
		t.Errorf("Use = %q, want mcp", cmd.Use)
	}
	if cmd.RunE == nil {
		t.Error("RunE should be set")
	}
}
