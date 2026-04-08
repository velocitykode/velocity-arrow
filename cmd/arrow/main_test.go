package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func newTestRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "arrow",
		Short: "Velocity-focused MCP server and AI assistant installer",
	}
	root.AddCommand(mcpCmd())
	root.AddCommand(installCmd())
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
	if !strings.Contains(output, "install") {
		t.Error("help should list install subcommand")
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

func TestInstallCmd_Builds(t *testing.T) {
	cmd := installCmd()
	if cmd.Use != "install" {
		t.Errorf("Use = %q, want install", cmd.Use)
	}
	if cmd.RunE == nil {
		t.Error("RunE should be set")
	}
}

func TestInstallCmd_NoGoMod(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(old)

	root := newTestRoot()
	root.SetArgs([]string{"install"})

	err := root.Execute()
	if err == nil {
		t.Error("install in empty dir should error")
	}
}

func TestInstallCmd_VelocityProject(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(`module testapp

go 1.26

require github.com/velocitykode/velocity v0.20.3
`), 0644)
	os.MkdirAll(filepath.Join(dir, ".claude"), 0755)

	old, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(old)

	root := newTestRoot()
	root.SetArgs([]string{"install"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("install should succeed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); err != nil {
		t.Error("CLAUDE.md should be created")
	}
	if _, err := os.Stat(filepath.Join(dir, ".mcp.json")); err != nil {
		t.Error(".mcp.json should be created")
	}
}
