#!/usr/bin/env bash
# Copilot preToolUse hook — blocks forbidden bash patterns.
# Input: JSON payload via stdin (camelCase or PascalCase format).
# Output: {"permissionDecision": "deny"} to block, {} to allow.
# Exit code: non-zero = fail-closed (tool call denied).

set -euo pipefail

INPUT=$(cat)

# Support both camelCase (toolArgs.command) and PascalCase (tool_input.command) payloads
COMMAND=$(echo "$INPUT" | jq -r '.toolArgs.command // .tool_input.command // ""' 2>/dev/null || true)

BLOCKED_PATTERNS=(
  "git push --force"
  "git reset --hard"
  "rm -rf /"
  "DROP TABLE"
  "DELETE FROM.*WHERE.*1=1"
  "> \\.env"
)

for pattern in "${BLOCKED_PATTERNS[@]}"; do
  if echo "$COMMAND" | grep -qE "$pattern"; then
    jq -n --arg reason "BLOCKED: command matches forbidden pattern: $pattern" \
      '{permissionDecision: "deny", reason: $reason}'
    exit 0
  fi
done

echo "{}"
exit 0
