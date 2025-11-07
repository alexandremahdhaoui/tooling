# Keeping Documentation Up to Date - AI Coding Agent Guide

**Purpose**: This document is a comprehensive cross-reference database and maintenance guide for AI Coding Agents to ensure documentation stays synchronized with code changes.

**Critical**: This document itself must be kept up to date. See [Maintaining This Document](#maintaining-this-document) section.

---

## Table of Contents

- [Overview](#overview)
- [The 4 Documentation Types](#the-4-documentation-types)
- [Cross-Reference Matrix](#cross-reference-matrix)
- [Change Detection Procedure](#change-detection-procedure)
- [Update Procedures by Change Type](#update-procedures-by-change-type)
- [Drift Detection Methodology](#drift-detection-methodology)
- [Maintaining This Document](#maintaining-this-document)
- [Quick Reference Commands](#quick-reference-commands)

---

## Overview

### When to Use This Guide

Use this guide **whenever you make changes** to:
- Code in `cmd/*/` (CLI tools, MCP servers)
- Code in `pkg/*/` (public packages)
- Code in `internal/*/` (internal utilities)
- Configuration schemas (`forge.yaml` structure)
- Test infrastructure
- Build system

### Documentation Philosophy

**Golden Rule**: Documentation must reflect reality. When code changes, documentation must change immediately.

**Documentation Debt**: Never commit code changes without updating corresponding documentation. This creates technical debt that compounds over time.

---

## The 4 Documentation Types

### Type 1: Normal READMEs (User-Facing)

**Structure**: WHY → HOW → WHAT
**Location**: `README.md`, `cmd/*/README.md`
**Update Frequency**: When features change, new tools added, or usage changes
**Style**: Concise, action-oriented

### Type 2: Prompts (AI Assistant Instructions)

**Structure**: Instructions + Reference Guide (at end)
**Location**: `docs/prompts/*.md`
**Update Frequency**: When APIs change, new patterns emerge, or workflows change
**Style**: Token-efficient, step-by-step, includes complete guide at end

### Type 3: MCP Scripts (MCP Server Documentation)

**Structure**: Purpose → Invocation → Tools → Integration → See Also
**Location**: `cmd/*/MCP.md` (co-located with MCP servers)
**Update Frequency**: When MCP tools change (input/output schemas, new tools, etc.)
**Style**: Concise but detailed, schema-focused

### Type 4: ARCHITECTURE.md (System Architecture)

**Structure**: Overview → Components → Diagrams → Patterns
**Location**: `ARCHITECTURE.md` (single file at root)
**Update Frequency**: When architecture changes, new components added, or system design evolves
**Style**: Detailed with diagrams, token-efficient prose

---

## Cross-Reference Matrix

This matrix maps code locations to documentation that must be updated.

### CMD Binary Changes

| Code Change | Documentation to Update | Priority | Notes |
|-------------|------------------------|----------|-------|
| New binary in `cmd/` | 1. `README.md` (Available Tools)<br>2. `ARCHITECTURE.md` (Command-Line Tools)<br>3. `component-inventory.md`<br>4. This document | High | Add tool count, update categories |
| Binary renamed | 1. All mentions in all docs<br>2. All prompts<br>3. All examples in `forge.yaml`<br>4. `ARCHITECTURE.md` | Critical | Use global search/replace, verify examples |
| Binary deleted | 1. Remove from `README.md`<br>2. Remove from `ARCHITECTURE.md`<br>3. Archive related docs<br>4. Update tool count | High | Mark as deprecated first if used |
| Binary flags changed | 1. `cmd/*/README.md`<br>2. Prompts that reference it<br>3. Examples in guides | Medium | Update all command examples |
| Binary dependencies changed | 1. `ARCHITECTURE.md` (Dependencies)<br>2. `cmd/*/README.md` | Low | Document major dependency changes |

### MCP Server Changes

| Code Change | Documentation to Update | Priority | Notes |
|-------------|------------------------|----------|-------|
| New MCP tool added | 1. `cmd/*/MCP.md` (add tool section)<br>2. `mcp-tools-inventory.md`<br>3. Relevant prompt | High | Include input/output schemas |
| MCP tool removed | 1. `cmd/*/MCP.md` (remove tool)<br>2. `mcp-tools-inventory.md`<br>3. Update examples | High | Check for usage in examples |
| MCP tool input schema changed | 1. `cmd/*/MCP.md` (update schema)<br>2. All examples using that tool<br>3. Related prompt | Critical | Breaking change - update all examples |
| MCP tool output schema changed | 1. `cmd/*/MCP.md` (update schema)<br>2. Consumers of that output<br>3. Test examples | Critical | Breaking change - verify consumers |
| New MCP server created | 1. Create `cmd/*/MCP.md`<br>2. Update `README.md`<br>3. Update `ARCHITECTURE.md`<br>4. Update `mcp-tools-inventory.md`<br>5. This document | Critical | Follow MCP.md template |
| MCP server deleted | 1. Remove `cmd/*/MCP.md`<br>2. Update all cross-references<br>3. Remove from inventories | High | Check for dependencies |

### Package (pkg/) Changes

| Code Change | Documentation to Update | Priority | Notes |
|-------------|------------------------|----------|-------|
| New public package | 1. `ARCHITECTURE.md` (Core Packages)<br>2. `README.md` if user-facing<br>3. GoDoc comments in code | High | Explain purpose and use cases |
| Package API changed | 1. `ARCHITECTURE.md` examples<br>2. Related prompts<br>3. Code examples in guides | High | Update all code examples |
| Package deprecated | 1. Mark in `ARCHITECTURE.md`<br>2. Add migration guide<br>3. Update prompts | Medium | Provide alternatives |
| New struct/type added | 1. `ARCHITECTURE.md` if central<br>2. Related MCP.md if used in schemas<br>3. GoDoc comments | Medium | Document if part of public API |

### forge.yaml Schema Changes

| Code Change | Documentation to Update | Priority | Notes |
|-------------|------------------------|----------|-------|
| New top-level field | 1. `docs/forge-schema.md`<br>2. All example `forge.yaml` snippets<br>3. `README.md` quick start<br>4. Related prompts | Critical | Update ALL examples |
| Field renamed | 1. Global search/replace in all docs<br>2. `docs/forge-schema.md`<br>3. All examples | Critical | Breaking change |
| Field deprecated | 1. Mark in `docs/forge-schema.md`<br>2. Add migration notes<br>3. Update examples to new pattern | High | Keep examples for 1 version |
| New build spec field | 1. `docs/forge-schema.md`<br>2. Build-related prompts<br>3. Examples in `README.md` | High | Show in examples |
| New test spec field | 1. `docs/forge-schema.md`<br>2. Test-related prompts<br>3. `docs/forge-test-usage.md` | High | Show in examples |

### Test Infrastructure Changes

| Code Change | Documentation to Update | Priority | Notes |
|-------------|------------------------|----------|-------|
| testenv orchestration changed | 1. `docs/testenv-architecture.md`<br>2. `ARCHITECTURE.md` (Testing Infrastructure)<br>3. `cmd/testenv/MCP.md`<br>4. Related prompts | High | Update diagrams |
| TestEnvironment schema changed | 1. All docs showing TestEnvironment struct<br>2. `cmd/testenv/MCP.md`<br>3. `ARCHITECTURE.md` examples | Critical | Breaking change |
| Test stage added/removed | 1. `docs/forge-test-usage.md`<br>2. Examples in all guides<br>3. Test-related prompts | Medium | Update stage lists |
| Test report format changed | 1. `cmd/test-report/MCP.md`<br>2. `pkg/forge` docs<br>3. TestReport examples | High | Update schema docs |

### Build System Changes

| Code Change | Documentation to Update | Priority | Notes |
|-------------|------------------------|----------|-------|
| Makefile targets changed | 1. `ARCHITECTURE.md` (Build System)<br>2. `README.md` if affects users<br>3. Development guides | Medium | Document new workflows |
| Build dependencies changed | 1. `README.md` (Prerequisites)<br>2. `ARCHITECTURE.md` (Dependencies)<br>3. Installation guides | High | Update version numbers |
| CI/CD pipeline changed | 1. Relevant workflow docs<br>2. `docs/forge-usage.md` if affects users | Medium | Document new processes |

---

## Change Detection Procedure

Follow this procedure when making **any** code change:

### Step 1: Identify Change Type

```bash
# What did you change?
git diff --name-only

# Categorize changes:
# - cmd/* → Binary/MCP change
# - pkg/* → Package API change
# - internal/* → Usually no doc update (unless public-facing behavior)
# - *.go files → Check for schema/API changes
# - forge.yaml → Schema change
# - Test files → Check if test infrastructure changed
```

### Step 2: Find Affected Documentation

Use the [Cross-Reference Matrix](#cross-reference-matrix) above to identify which docs need updates.

**Quick Search Method**:
```bash
# Search for references to changed component
grep -r "component-name" docs/
grep -r "component-name" *.md
grep -r "component-name" cmd/*/README.md
grep -r "component-name" cmd/*/MCP.md
```

### Step 3: Verify Update Completeness

Checklist before committing:
- [ ] All docs in Cross-Reference Matrix updated?
- [ ] All code examples tested and working?
- [ ] All schemas/types reflect new reality?
- [ ] Statistics updated (file counts, tool counts, etc.)?
- [ ] Cross-references between docs still valid?
- [ ] No broken links?
- [ ] Old names/references removed?

### Step 4: Update This Document

If you added a new component type, changed documentation structure, or discovered a missing cross-reference:
- [ ] Update the [Cross-Reference Matrix](#cross-reference-matrix)
- [ ] Update [Quick Reference Commands](#quick-reference-commands)
- [ ] Document the new pattern

---

## Update Procedures by Change Type

### Procedure: Adding a New MCP Tool

When you add a new tool to an existing MCP server:

```bash
# 1. Update the MCP.md file
# File: cmd/<server-name>/MCP.md
# Add new section under "Available Tools" with:
# - Tool name and description
# - Input schema (JSON)
# - Output schema (JSON)
# - Example usage
# - Integration example (forge.yaml)

# 2. Update the MCP tools inventory
# File: .ai/plan/doc-review/mcp-tools-inventory.md
# Add tool under the appropriate server section

# 3. Update related prompts
# Search for prompts that reference this MCP server
grep -r "go://<server-name>" docs/prompts/

# 4. Update examples in guides
# Search for usage examples of this server
grep -r "<server-name>" docs/

# 5. Test the documentation
# Verify all examples work:
<server-name> --mcp  # Test MCP server starts
# Test example MCP call from docs
```

### Procedure: Renaming a Binary

Critical procedure - affects many files:

```bash
# 1. Plan the rename
OLD_NAME="old-name"
NEW_NAME="new-name"

# 2. Search for all references
grep -r "$OLD_NAME" . --include="*.md" | grep -v ".git"

# 3. Update in order of priority:
# Priority 1: Code references (separate commit)
# Priority 2: MCP.md if exists
# Priority 3: README.md
# Priority 4: ARCHITECTURE.md
# Priority 5: All prompts
# Priority 6: All guides
# Priority 7: All examples

# 4. Use replace_all for systematic updates
# In each file, use Edit tool with replace_all: true

# 5. Verify no old references remain
grep -r "$OLD_NAME" . --include="*.md" | grep -v ".git"

# 6. Update statistics
# - Tool counts in README.md
# - Tool counts in ARCHITECTURE.md
# - Component inventory

# 7. Test all examples that used the binary
```

### Procedure: Changing a Schema

When modifying struct definitions used in documentation:

```bash
# 1. Identify the struct
# Example: TestReport in pkg/forge/spec_tst.go

# 2. Find all documentation references
grep -r "TestReport" docs/
grep -r "TestReport" cmd/*/MCP.md
grep -r "TestReport" ARCHITECTURE.md

# 3. Update all struct displays
# - ARCHITECTURE.md examples
# - MCP.md output schemas
# - forge-schema.md field descriptions

# 4. Update all code examples using the struct
# Search for example usage

# 5. Verify fields match code
# For each documented field, verify it exists in actual struct
# For each struct field, verify it's documented

# 6. Update JSON schemas in MCP.md
# Ensure JSON examples match Go struct tags
```

### Procedure: Adding a New Document Type

If you create a new documentation pattern:

```bash
# 1. Document it in doc-convention.md
# Add as "Type 5" or appropriate category

# 2. Update this document
# Add to Cross-Reference Matrix
# Add update procedures

# 3. Update the documentation review plan
# File: .ai/plan/doc-review/tasks.md
# Add tasks for maintaining this doc type

# 4. Create template or example
# Show the pattern for others to follow
```

---

## Drift Detection Methodology

Use this procedure to systematically detect documentation drift.

### Method 1: Automated Checks (Quick)

Run these checks regularly:

```bash
# Check 1: Tool count consistency
CMD_COUNT=$(ls -d cmd/*/ | wc -l)
README_TOOLS=$(grep -c "^- " README.md | head -1 || echo "0")
echo "CMD binaries: $CMD_COUNT"
echo "README tools: Verify manually against 20"

# Check 2: MCP server count
MCP_COUNT=$(find cmd -name "mcp.go" | wc -l)
MCP_DOCS=$(find cmd -name "MCP.md" | wc -l)
echo "MCP servers: $MCP_COUNT"
echo "MCP.md files: $MCP_DOCS"
if [ $MCP_COUNT -ne $MCP_DOCS ]; then
  echo "⚠️  DRIFT: Missing MCP.md files"
fi

# Check 3: Code file count vs documented
GO_FILES=$(find . -name "*.go" -not -path "./vendor/*" | wc -l)
echo "Go files: $GO_FILES (documented: 123)"

# Check 5: forge.yaml examples valid
# Extract and validate all forge.yaml snippets in docs
for file in $(find docs -name "*.md"); do
  echo "Checking $file for forge.yaml examples..."
  # Manual review needed - check for old field names
done
```

### Method 2: Manual Review (Thorough)

Quarterly or after major refactoring:

#### Phase 1: Inventory Reality

```bash
# 1. Current tool list
ls -1 cmd/

# 2. Current package list
ls -1 pkg/

# 3. Current MCP servers
find cmd -name "mcp.go"

# 4. Current statistics
echo "Go files:" $(find . -name "*.go" -not -path "./vendor/*" | wc -l)
echo "Lines:" $(find . -name "*.go" -not -path "./vendor/*" -exec cat {} \; | wc -l)
```

#### Phase 2: Compare Against Documentation

For each documented item:
- [ ] Does it exist in code?
- [ ] Are names current?
- [ ] Are schemas current?
- [ ] Are examples runnable?

For each code item:
- [ ] Is it documented?
- [ ] Is documentation current?
- [ ] Are cross-references valid?

#### Phase 3: Schema Validation

```bash
# For each MCP.md file
for mcp_doc in $(find cmd -name "MCP.md"); do
  server_dir=$(dirname $mcp_doc)
  server_code="$server_dir/mcp.go"

  echo "Checking $server_dir..."

  # Extract tool names from code
  grep "Name:" $server_code | sed 's/.*Name: *"\([^"]*\)".*/\1/'

  # Verify against MCP.md
  grep "^### " $mcp_doc | sed 's/### //'

  # Manual comparison needed
done
```

#### Phase 4: Example Validation

```bash
# Extract all code blocks from docs
for doc in $(find docs -name "*.md"); do
  echo "Checking $doc..."
  # Look for ```yaml, ```bash, ```go blocks
  # Test if they're valid/current
done
```

### Method 3: Semantic Drift Detection

Look for these patterns:

1. **Feature described but not implemented**: Docs mention features that don't exist in code
2. **Implementation without docs**: Code has features not mentioned in docs
3. **Outdated workflows**: Step-by-step guides that don't match current CLI
4. **Broken examples**: Examples that produce errors when run
5. **Inconsistent terminology**: Same concept called different names
6. **Orphaned references**: Links to deleted files or sections

### Method 4: Continuous Drift Prevention

Best practice - integrate into workflow:

```bash
# Pre-commit check
cat > .git/hooks/pre-commit <<'EOF'
#!/bin/bash
# Check for common drift patterns

if git diff --cached --name-only | grep -q "^cmd/"; then
  echo "CMD changes detected. Have you updated:"
  echo "  - README.md?"
  echo "  - ARCHITECTURE.md?"
  echo "  - Relevant MCP.md?"
  echo ""
  read -p "Confirm documentation updated (y/n): " confirm
  if [ "$confirm" != "y" ]; then
    echo "Commit aborted. Update docs first."
    exit 1
  fi
fi
EOF
chmod +x .git/hooks/pre-commit
```

---

## Maintaining This Document

### This Document Must Stay Current

**Critical**: This cross-reference database is only useful if it reflects reality.

### When to Update This Document

Update `keeping-docs-up-to-date.md` when:

1. **New documentation type created**: Add to [The 4 Documentation Types](#the-4-documentation-types)
2. **New component type added**: Add row to [Cross-Reference Matrix](#cross-reference-matrix)
3. **Documentation structure changes**: Update procedures
4. **New drift pattern discovered**: Add to detection methodology
5. **New automation added**: Update check scripts

### How to Update This Document

```bash
# 1. Identify what changed
# - New doc type?
# - New component type?
# - New procedure?
# - New drift pattern?

# 2. Update relevant section
# - Keep structure consistent
# - Add to cross-reference matrix
# - Add to drift detection if applicable

# 3. Update Table of Contents if needed

# 4. Test procedures
# Verify new procedures work as described

# 5. Commit with clear message
git add docs/keeping-docs-up-to-date.md
git commit -m "docs: update doc maintenance guide - [what changed]"
```

### Detecting Drift in This Document

This document drifts when:
- New docs added but not listed in cross-references
- Procedures become outdated
- Checks no longer work
- New patterns emerge but aren't documented

**Detection**:
```bash
# Compare documented components vs reality
# 1. List all doc types mentioned here
grep "Type [0-9]:" docs/keeping-docs-up-to-date.md

# 2. List all actual doc patterns in repo
find . -name "*.md" -not -path "./.git/*" -not -path "./vendor/*"

# 3. Identify gaps
# Manual review - are there doc types not covered?

# 4. Check procedures
# Try following each procedure - does it work?
```

### Ownership

**Maintainer**: AI Coding Agents + Human Developers
**Review Frequency**: Monthly or after major refactoring
**Update Triggers**: Any structural change to documentation or codebase

---

## Quick Reference Commands

### Finding What to Update

```bash
# Find all docs mentioning a component
grep -r "component-name" --include="*.md" .

# Find all MCP.md files
find cmd -name "MCP.md"

# Find all READMEs
find . -name "README.md" -not -path "./vendor/*"

# Find all prompts
ls docs/prompts/
```

### Validation Commands

```bash
# Count tools
ls -d cmd/*/ | wc -l

# Count MCP servers
find cmd -name "mcp.go" | wc -l

# Count MCP.md files
find cmd -name "MCP.md" | wc -l

# Count Go files
find . -name "*.go" -not -path "./vendor/*" | wc -l

# Count Go lines
find . -name "*.go" -not -path "./vendor/*" -exec cat {} \; | wc -l

# List packages
ls -d pkg/*/
```

### Testing Examples

```bash
# Test MCP server starts
<binary-name> --mcp

# Test CLI commands from docs
# Copy command from docs and run

# Validate YAML
# Copy forge.yaml snippet to temp file
# Attempt to use with forge
```

---

## Examples of Good Maintenance

### Example 1: Adding a New MCP Tool

**Code Change**: Added `buildBatch` tool to `build-go` MCP server

**Documentation Updates**:
1. ✅ Updated `cmd/build-go/MCP.md` - added buildBatch section with schema
2. ✅ Updated `mcp-tools-inventory.md` - added tool to build-go section
3. ✅ Updated `docs/prompts/create-build-engine.md` - mentioned batch capabilities
4. ✅ Added example to `README.md` showing batch usage

**Verification**:
- Tested MCP call with new tool
- Verified JSON schema matches code
- Ran example from README.md

### Example 2: Renaming a Binary

**Code Change**: Renamed `generic-engine` to `generic-builder`

**Documentation Updates**:
1. ✅ Renamed `docs/prompts/use-generic-engine.md` → `use-generic-builder.md`
2. ✅ Updated all references in prompts (replace_all)
3. ✅ Updated all examples in guides
4. ✅ Updated README.md tool list
5. ✅ Updated ARCHITECTURE.md references
6. ✅ Updated forge.yaml examples
7. ✅ Verified no "generic-engine" remains (except historical notes)

**Verification**:
```bash
grep -r "generic-engine" --include="*.md" . | grep -v ".git"
# Should only show historical references
```

### Example 3: Schema Change

**Code Change**: Added `artifactFiles` field to `TestReport` struct

**Documentation Updates**:
1. ✅ Updated `cmd/generic-test-runner/MCP.md` - output schema includes new field
2. ✅ Updated `cmd/test-report/MCP.md` - get/list output includes field
3. ✅ Updated `ARCHITECTURE.md` - TestEnvironment example shows field
4. ✅ Updated `docs/forge-test-usage.md` - example output includes field
5. ✅ Updated `docs/prompts/use-generic-test-runner.md` - documented field purpose

**Verification**:
- Checked all TestReport struct examples match new fields
- Verified JSON schemas in MCP.md files match Go struct tags
- Tested example commands produce output matching docs

---

## Conclusion

Documentation maintenance is **critical** and **continuous**. Use this guide to:

1. **Before making code changes**: Understand what docs will need updates
2. **During development**: Update docs alongside code
3. **Before committing**: Run drift detection checks
4. **After committing**: Verify docs match new reality
5. **Regularly**: Run full drift detection methodology

**Remember**: Documentation debt compounds. Fix it immediately, every time.

---

**Last Updated**: 2025-01-06
**Version**: 1.0
**Maintainer**: AI Coding Agents + Human Developers
