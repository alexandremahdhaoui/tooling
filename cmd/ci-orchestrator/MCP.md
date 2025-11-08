# ci-orchestrator MCP Server

MCP server for orchestrating CI/CD pipelines (not yet implemented).

## Purpose

**Status: PLANNED - NOT YET IMPLEMENTED**

This is a placeholder for future CI/CD pipeline orchestration functionality. The tool structure is in place, but all operations currently return "not yet implemented" errors.

## Invocation

```bash
ci-orchestrator --mcp
```

The server will start but all tool calls will return errors indicating the functionality is not yet implemented.

## Planned Tools

### `run` (Not Implemented)

Execute a CI pipeline.

**Planned Input Schema:**
```json
{
  "pipeline": "string (required)"       // Pipeline name or configuration
}
```

**Current Behavior:**
Returns error: "ci-orchestrator: not yet implemented"

**Planned Output:**
```json
{
  "status": "success|failed",
  "duration": 123.45,
  "steps": [
    {
      "name": "step name",
      "status": "success",
      "duration": 12.34
    }
  ]
}
```

**Example (will fail with not implemented error):**
```json
{
  "method": "tools/call",
  "params": {
    "name": "run",
    "arguments": {
      "pipeline": "build-and-test"
    }
  }
}
```

## Planned Features

The ci-orchestrator is planned to provide:

- **Pipeline Execution**: Run multi-stage CI/CD pipelines
- **Stage Orchestration**: Coordinate build, test, and deployment stages
- **Artifact Management**: Track artifacts across pipeline stages
- **Integration with Forge**: Leverage forge's build and test infrastructure
- **MCP-Native**: Full MCP protocol support for AI agent integration

## Current Status

- ✅ Binary scaffold created
- ✅ MCP server framework in place
- ✅ Version command working
- ❌ Pipeline execution not implemented
- ❌ Configuration schema not defined
- ❌ Integration with forge not implemented

## Development

To implement ci-orchestrator functionality, see:
- [ARCHITECTURE.md](../../ARCHITECTURE.md) for design patterns
- [docs/prompts/](../../docs/prompts/) for engine creation guides
- Existing implementations in `cmd/testenv/` for orchestration patterns

## See Also

- [forge MCP Server](../forge/MCP.md) - Main orchestrator
- [testenv MCP Server](../testenv/MCP.md) - Test environment orchestration example
- [Forge Architecture](../../ARCHITECTURE.md)
