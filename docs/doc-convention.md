# Documentation Conventions

This document defines the 4 documentation types used in this repository and their conventions.

## The 4 Document Types

### 1. Normal READMEs (User-Facing)

**Purpose**: Help users understand and use tools/components

**Structure**: WHY → HOW → WHAT
- **WHY Section**: Problem being solved (doesn't need literal "Why" title)
- **HOW Section**: How the tool solves the problem (approach/methodology)
- **WHAT Section**: Largest section - what it is, how to use it, installation, examples

**Style**: Concise, not verbose, action-oriented

**Examples**: README.md, cmd/*/README.md

### 2. Prompts (AI Assistant Instructions)

**Purpose**: Guide AI assistants like Claude Code on performing specific tasks

**Style**: Token-efficient but extremely detailed with step-by-step instructions

**Examples**: docs/prompts/*.md

**Requirements**:
- Detailed task instructions for AI
- Clear expected inputs/outputs
- Examples of usage
- Token-efficient (no unnecessary prose)

### 3. MCP Scripts (MCP Server Documentation)

**Purpose**: Document MCP servers and available tools for AI coding assistants

**Style**: Concise but detailed, explains tools/commands available via MCP

**Location**: cmd/*/MCP.md (co-located with MCP servers)

**Required Sections**:
- Purpose
- Invocation (how to start MCP server)
- Available Tools (each with input/output schemas)
- Integration examples (forge.yaml)
- See Also references

**Examples**: cmd/go-build/MCP.md, cmd/testenv/MCP.md

### 4. ARCHITECTURE.md (Single Architecture Document)

**Purpose**: Comprehensive system architecture documentation

**Style**: Detailed but token-efficient (not unnecessarily verbose)

**Requirements**:
- Heavy use of diagrams (ASCII art, Mermaid)
- Current statistics and structure
- All components documented
- Design patterns explained
- Token-efficient prose (bullet points over paragraphs)

**File**: ARCHITECTURE.md (single file at root)

## General Conventions

### README.md Files

- Every cmd/* and major pkg/* should have README.md
- Use WHY→HOW→WHAT structure
- Include Table of Contents for longer docs
- Cross-reference related documents in "See Also" section
- Keep concise - aim for 30%+ shorter than typical docs

### Cross-Referencing

Use relative paths for internal links:
```markdown
See [MCP Server Documentation](./MCP.md)
See [Architecture](../../ARCHITECTURE.md)
```

### Code Examples

All code examples must be:
- Syntactically correct
- Use current tool/binary names
- Include comments explaining key parts

### GoDoc Comments

Public functions, methods, and types should have GoDoc comments explaining:
- Purpose
- Parameters
- Return values
- Example usage (where helpful)

## Maintenance

When updating documentation:
1. Verify tool names are current (no test-integration, kindenv, local-container-registry, generic-engine)
2. Update statistics to match reality
3. Test all code examples
4. Maintain WHY→HOW→WHAT structure for READMEs
5. Keep token count low (prefer bullets over prose)
