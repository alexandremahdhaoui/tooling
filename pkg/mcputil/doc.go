// Package mcputil provides common utilities for MCP (Model Context Protocol) servers.
// It includes helpers for batch operations, input validation, and result creation.
//
// This package simplifies the creation of MCP servers by providing reusable patterns for:
//   - Batch operation handling (HandleBatchBuild)
//   - Input validation (ValidateRequired)
//   - Standardized result creation (ErrorResult, SuccessResult, SuccessResultWithArtifact)
package mcputil
