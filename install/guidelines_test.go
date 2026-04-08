package install

import (
	"strings"
	"testing"
)

func TestComposeGuidelines_LayerOrder(t *testing.T) {
	goMod := &projectGoMod{
		module:    "testapp",
		goVersion: "1.26",
		deps: []projectDep{
			{path: "github.com/velocitykode/velocity", version: "v0.20.3"},
		},
	}

	result := composeGuidelines(goMod)

	// Verify layer order: foundation < velocity/core < go/core < packages
	foundationIdx := strings.Index(result, "Velocity Project")
	velCoreIdx := strings.Index(result, "Velocity Framework Patterns")
	goCoreIdx := strings.Index(result, "Go Conventions")
	ormIdx := strings.Index(result, "Velocity ORM")

	if foundationIdx < 0 {
		t.Fatal("missing foundation section")
	}
	if velCoreIdx < 0 {
		t.Fatal("missing velocity core section")
	}
	if goCoreIdx < 0 {
		t.Fatal("missing go core section")
	}
	if ormIdx < 0 {
		t.Fatal("missing ORM section")
	}

	if foundationIdx >= velCoreIdx {
		t.Errorf("foundation (idx=%d) should come before velocity/core (idx=%d)", foundationIdx, velCoreIdx)
	}
	if velCoreIdx >= goCoreIdx {
		t.Errorf("velocity/core (idx=%d) should come before go/core (idx=%d)", velCoreIdx, goCoreIdx)
	}
	if goCoreIdx >= ormIdx {
		t.Errorf("go/core (idx=%d) should come before package guidelines like ORM (idx=%d)", goCoreIdx, ormIdx)
	}
}

func TestComposeGuidelines_TemplatesModuleName(t *testing.T) {
	goMod := &projectGoMod{
		module:    "github.com/myorg/myproject",
		goVersion: "1.25",
		deps: []projectDep{
			{path: "github.com/velocitykode/velocity", version: "v0.20.3"},
		},
	}

	result := composeGuidelines(goMod)

	if !strings.Contains(result, "github.com/myorg/myproject") {
		t.Error("module name not templated into output")
	}
	if !strings.Contains(result, "1.25") {
		t.Error("go version not templated into output")
	}
	// Should NOT contain template syntax
	if strings.Contains(result, "{{.Module}}") {
		t.Error("raw template variable leaked into output")
	}
	if strings.Contains(result, "{{.GoVersion}}") {
		t.Error("raw template variable leaked into output")
	}
}

func TestComposeGuidelines_AllPackagesIncludedWithVelocity(t *testing.T) {
	goMod := &projectGoMod{
		module:    "testapp",
		goVersion: "1.26",
		deps: []projectDep{
			{path: "github.com/velocitykode/velocity", version: "v0.20.3"},
		},
	}

	result := composeGuidelines(goMod)

	packages := []struct {
		name    string
		marker  string
	}{
		{"ORM", "Velocity ORM"},
		{"Cache", "Velocity Cache"},
		{"Queue", "Velocity Queue"},
		{"Auth", "Velocity Auth"},
		{"Mail", "Velocity Mail"},
		{"Storage", "Velocity Storage"},
	}

	for _, pkg := range packages {
		if !strings.Contains(result, pkg.marker) {
			t.Errorf("missing %s package guidelines (looked for %q)", pkg.name, pkg.marker)
		}
	}
}

func TestComposeGuidelines_NoVelocity_ExcludesPackages(t *testing.T) {
	goMod := &projectGoMod{
		module:    "testapp",
		goVersion: "1.26",
		deps: []projectDep{
			{path: "github.com/gin-gonic/gin", version: "v1.9.0"},
		},
	}

	result := composeGuidelines(goMod)

	// Core is always included
	if !strings.Contains(result, "Velocity Project") {
		t.Error("foundation should always be included")
	}

	// Package guidelines must NOT be included
	excluded := []string{"Velocity ORM", "Velocity Cache", "Velocity Queue", "Velocity Auth", "Velocity Mail", "Velocity Storage"}
	for _, marker := range excluded {
		if strings.Contains(result, marker) {
			t.Errorf("%q should NOT be present without velocity dep", marker)
		}
	}
}

func TestRenderTemplate_InterpolatesValues(t *testing.T) {
	goMod := &projectGoMod{
		module:    "my-unique-module-name",
		goVersion: "1.99",
	}

	result := renderTemplate("guidelines/foundation.md", goMod)
	if !strings.Contains(result, "my-unique-module-name") {
		t.Error("module name not interpolated")
	}
	if !strings.Contains(result, "1.99") {
		t.Error("go version not interpolated")
	}
}

func TestRenderTemplate_NonExistent(t *testing.T) {
	result := renderTemplate("guidelines/nonexistent.md", &projectGoMod{})
	if result != "" {
		t.Error("should return empty for nonexistent template")
	}
}
