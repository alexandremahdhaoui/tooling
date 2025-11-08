package orchestrate

// Package orchestrate provides utilities for orchestrating multiple MCP engines
// in sequence. This is used to support multi-engine builder and test-runner aliases.

// MCPCaller is a function type for calling MCP engines.
// It abstracts the actual MCP communication so orchestrators can be tested
// with mock implementations.
type MCPCaller func(binaryPath string, toolName string, params interface{}) (interface{}, error)

// EngineResolver is a function type for resolving engine URIs to binary paths.
// It handles both direct URIs (e.g., "go://build-go") and alias URIs (e.g., "alias://my-builder").
type EngineResolver func(engineURI string) (string, error)

// EngineCall represents a single MCP engine invocation.
type EngineCall struct {
	// EngineName is the human-readable name of the engine (for logging/errors)
	EngineName string

	// BinaryPath is the resolved path to the MCP server binary
	BinaryPath string

	// ToolName is the MCP tool to invoke (e.g., "build", "run")
	ToolName string

	// Params are the parameters to pass to the MCP tool
	Params map[string]any
}

// EngineResult represents the result of an MCP engine call.
type EngineResult struct {
	// EngineName is the engine that produced this result
	EngineName string

	// Result is the raw result from the MCP call
	Result interface{}

	// Error is any error that occurred during the call
	Error error
}

// Orchestrator defines the interface for multi-engine orchestration.
type Orchestrator interface {
	// Orchestrate executes multiple engines in sequence and aggregates results.
	// The exact behavior depends on the implementation (builder vs test-runner).
	Orchestrate(engines []EngineSpec) (interface{}, error)
}

// EngineSpec defines a generic engine specification for orchestration.
// This is intentionally generic to support different engine types.
type EngineSpec struct {
	// EngineName is a human-readable identifier for this engine (for errors/logging)
	EngineName string

	// EngineURI is the engine reference (e.g., "go://build-go", "alias://my-builder")
	EngineURI string

	// Spec contains engine-specific configuration (command, args, env, etc.)
	Spec map[string]interface{}
}
