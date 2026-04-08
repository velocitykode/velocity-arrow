package install

import (
	"embed"
	"strings"
	"text/template"
)

//go:embed guidelines
var guidelinesFS embed.FS

// composeGuidelines builds the layered guidelines from embedded templates.
func composeGuidelines(goMod *projectGoMod) string {
	var sections []string

	// Layer 1: Core - always included
	sections = append(sections, renderTemplate("guidelines/foundation.md", goMod))
	sections = append(sections, renderTemplate("guidelines/velocity/core.md", goMod))
	sections = append(sections, renderTemplate("guidelines/go/core.md", goMod))

	// Layer 2: Package-conditional - when dep in go.mod
	packageGuidelines := map[string]string{
		"github.com/velocitykode/velocity/orm":     "guidelines/packages/orm.md",
		"github.com/velocitykode/velocity/cache":   "guidelines/packages/cache.md",
		"github.com/velocitykode/velocity/queue":   "guidelines/packages/queue.md",
		"github.com/velocitykode/velocity/auth":    "guidelines/packages/auth.md",
		"github.com/velocitykode/velocity/mail":    "guidelines/packages/mail.md",
		"github.com/velocitykode/velocity/storage": "guidelines/packages/storage.md",
	}

	// Also check if the main velocity module includes these sub-packages
	// (they may be part of the main module, not separate deps)
	hasVelocity := hasDep(goMod, "github.com/velocitykode/velocity")

	for dep, tmpl := range packageGuidelines {
		if hasDep(goMod, dep) || hasVelocity {
			content := renderTemplate(tmpl, goMod)
			if content != "" {
				sections = append(sections, content)
			}
		}
	}

	return strings.Join(sections, "\n\n")
}

func renderTemplate(path string, goMod *projectGoMod) string {
	data, err := guidelinesFS.ReadFile(path)
	if err != nil {
		return ""
	}

	tmpl, err := template.New(path).Parse(string(data))
	if err != nil {
		return string(data)
	}

	vars := map[string]any{
		"Module":    goMod.module,
		"GoVersion": goMod.goVersion,
		"Deps":      goMod.deps,
	}

	var b strings.Builder
	if err := tmpl.Execute(&b, vars); err != nil {
		return string(data)
	}

	return b.String()
}
