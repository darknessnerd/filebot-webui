#!/usr/bin/env bash
# Copilot postToolUse hook — writes an audit log entry.
# Input: JSON payload via stdin (camelCase or PascalCase format).
# Output: {} (no-op, logging only).

set -euo pipefail

LOG_DIR=".claude/logs"
mkdir -p "$LOG_DIR"

INPUT=$(cat)
TOOL=$(echo "$INPUT" | jq -r '.toolName // .tool_name // "unknown"' 2>/dev/null || echo "unknown")
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || echo "unknown")

echo "$TIMESTAMP tool=$TOOL" >> "$LOG_DIR/tool-audit.log"

echo "{}"
exit 0
