package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/velocitykode/velocity-arrow/install/agents"
)

// Run executes the install command in the given project directory.
func Run(dir string) error {
	// Validate this is a Velocity project
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		return fmt.Errorf("no go.mod found - run 'arrow install' from a Velocity project root")
	}

	goMod, err := parseProjectGoMod(dir)
	if err != nil {
		return fmt.Errorf("parsing go.mod: %w", err)
	}

	if !isVelocityProject(goMod) {
		return fmt.Errorf("not a Velocity project - github.com/velocitykode/velocity not found in go.mod")
	}

	fmt.Println("Arrow - Installing into Velocity project")
	fmt.Printf("  Module: %s\n", goMod.module)
	fmt.Printf("  Go: %s\n", goMod.goVersion)
	fmt.Println()

	// Detect agents
	detected := detectAgents(dir)
	if len(detected) == 0 {
		fmt.Println("No AI agents detected. Install Claude Code, Cursor, or Codex first.")
		return nil
	}

	fmt.Printf("Detected agents: %s\n\n", formatAgentNames(detected))

	// Install to each agent
	for _, agent := range detected {
		fmt.Printf("Installing for %s...\n", agent.Name())

		// 1. Write guidelines
		if gAgent, ok := agent.(agents.GuidelinesAgent); ok {
			guidelines := composeGuidelines(goMod)
			status := writeGuidelines(dir, gAgent, guidelines)
			fmt.Printf("  Guidelines: %s\n", status)
		}

		// 2. Install skills
		if sAgent, ok := agent.(agents.SkillsAgent); ok {
			skills := matchSkills(goMod)
			status := writeSkills(dir, sAgent, skills)
			fmt.Printf("  Skills: %s (%d installed)\n", status, len(skills))
		}

		// 3. Register MCP
		if mAgent, ok := agent.(agents.MCPAgent); ok {
			status := writeMCPConfig(dir, mAgent)
			fmt.Printf("  MCP: %s\n", status)
		}

		fmt.Println()
	}

	fmt.Println("Done.")
	return nil
}

type projectGoMod struct {
	module    string
	goVersion string
	deps      []projectDep
}

type projectDep struct {
	path    string
	version string
}

func parseProjectGoMod(dir string) (*projectGoMod, error) {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return nil, err
	}

	info := &projectGoMod{}
	lines := strings.Split(string(data), "\n")
	inRequire := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "module ") {
			info.module = strings.TrimPrefix(line, "module ")
			continue
		}

		if strings.HasPrefix(line, "go ") {
			info.goVersion = strings.TrimPrefix(line, "go ")
			continue
		}

		if line == "require (" {
			inRequire = true
			continue
		}

		if line == ")" {
			inRequire = false
			continue
		}

		if inRequire {
			parts := strings.Fields(line)
			if len(parts) >= 2 && !strings.HasPrefix(parts[0], "//") {
				info.deps = append(info.deps, projectDep{path: parts[0], version: parts[1]})
			}
		}

		if strings.HasPrefix(line, "require ") && !strings.HasSuffix(line, "(") {
			parts := strings.Fields(strings.TrimPrefix(line, "require "))
			if len(parts) >= 2 {
				info.deps = append(info.deps, projectDep{path: parts[0], version: parts[1]})
			}
		}
	}

	return info, nil
}

func isVelocityProject(goMod *projectGoMod) bool {
	for _, dep := range goMod.deps {
		if dep.path == "github.com/velocitykode/velocity" ||
			strings.HasPrefix(dep.path, "github.com/velocitykode/velocity/") {
			return true
		}
	}
	return false
}

func hasDep(goMod *projectGoMod, path string) bool {
	for _, dep := range goMod.deps {
		if dep.path == path || strings.HasPrefix(dep.path, path+"/") {
			return true
		}
	}
	return false
}

func detectAgents(dir string) []agents.Agent {
	registry := []agents.Agent{
		agents.NewClaudeCode(),
		agents.NewCursor(),
		agents.NewCodex(),
	}

	var detected []agents.Agent
	for _, agent := range registry {
		if agent.DetectInProject(dir) || agent.DetectOnSystem() {
			detected = append(detected, agent)
		}
	}
	return detected
}

func formatAgentNames(agentList []agents.Agent) string {
	names := make([]string, len(agentList))
	for i, a := range agentList {
		names[i] = a.Name()
	}
	return strings.Join(names, ", ")
}
