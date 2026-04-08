package agents

// Agent is the interface for AI agent integrations.
type Agent interface {
	// Name returns the display name of the agent.
	Name() string

	// DetectOnSystem returns true if the agent is installed on the system.
	DetectOnSystem() bool

	// DetectInProject returns true if the agent has project-level configuration.
	DetectInProject(dir string) bool
}

// GuidelinesAgent is an agent that supports guidelines installation.
type GuidelinesAgent interface {
	Agent

	// GuidelinesPath returns the path to the guidelines file relative to the project root.
	GuidelinesPath() string

	// GuidelinesTag returns the XML tag name used to wrap guidelines.
	GuidelinesTag() string
}

// SkillsAgent is an agent that supports skill installation.
type SkillsAgent interface {
	Agent

	// SkillsDir returns the path to the skills directory relative to the project root.
	SkillsDir() string
}

// MCPAgent is an agent that supports MCP server registration.
type MCPAgent interface {
	Agent

	// MCPConfigPath returns the path to the MCP config file relative to the project root.
	MCPConfigPath() string

	// MCPConfigKey returns the JSON key under which servers are registered.
	MCPConfigKey() string
}
