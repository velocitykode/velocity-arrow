package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseGoMod_Full(t *testing.T) {
	dir := t.TempDir()
	gomod := `module myapp

go 1.26

require (
	github.com/velocitykode/velocity v0.20.3
	github.com/joho/godotenv v1.5.1
)

require (
	github.com/lib/pq v1.10.9 // indirect
)
`
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0644)

	info, err := parseGoMod(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.module != "myapp" {
		t.Errorf("module = %q, want %q", info.module, "myapp")
	}
	if info.goVersion != "1.26" {
		t.Errorf("goVersion = %q, want %q", info.goVersion, "1.26")
	}
	if len(info.deps) != 3 {
		t.Errorf("deps count = %d, want 3", len(info.deps))
	}
	if info.deps[0].path != "github.com/velocitykode/velocity" {
		t.Errorf("deps[0].path = %q, want velocity", info.deps[0].path)
	}
	if info.deps[0].version != "v0.20.3" {
		t.Errorf("deps[0].version = %q, want v0.20.3", info.deps[0].version)
	}
}

func TestParseGoMod_SingleLineRequire(t *testing.T) {
	dir := t.TempDir()
	gomod := `module singleapp

go 1.25

require github.com/velocitykode/velocity v0.18.0
`
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0644)

	info, err := parseGoMod(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.module != "singleapp" {
		t.Errorf("module = %q, want %q", info.module, "singleapp")
	}
	if len(info.deps) != 1 {
		t.Errorf("deps count = %d, want 1", len(info.deps))
	}
}

func TestParseGoMod_NoFile(t *testing.T) {
	dir := t.TempDir()
	_, err := parseGoMod(dir)
	if err == nil {
		t.Fatal("expected error for missing go.mod")
	}
}

func TestParseGoMod_SkipsComments(t *testing.T) {
	dir := t.TempDir()
	gomod := `module commentapp

go 1.26

require (
	// this is a comment
	github.com/velocitykode/velocity v0.20.3
)
`
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0644)

	info, err := parseGoMod(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(info.deps) != 1 {
		t.Errorf("deps count = %d, want 1 (should skip comment)", len(info.deps))
	}
}

func TestScanProviders_WithProviders(t *testing.T) {
	dir := t.TempDir()
	appDir := filepath.Join(dir, "app")
	os.MkdirAll(appDir, 0755)

	code := `package app

import "myapp/providers"

func registerProviders(reg *ProviderRegistry) {
	reg.Add(&providers.AuthProvider{}, &providers.MailProvider{})
}
`
	os.WriteFile(filepath.Join(appDir, "providers.go"), []byte(code), 0644)

	providers := scanProviders(dir)
	if len(providers) != 2 {
		t.Fatalf("providers count = %d, want 2", len(providers))
	}
	if providers[0] != "providers.AuthProvider" {
		t.Errorf("providers[0] = %q, want providers.AuthProvider", providers[0])
	}
	if providers[1] != "providers.MailProvider" {
		t.Errorf("providers[1] = %q, want providers.MailProvider", providers[1])
	}
}

func TestScanProviders_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	providers := scanProviders(dir)
	if len(providers) != 0 {
		t.Errorf("expected no providers, got %d", len(providers))
	}
}
