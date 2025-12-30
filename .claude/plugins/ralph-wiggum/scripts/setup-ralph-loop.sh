#!/usr/bin/env bash
# Ralph Loop Setup Script
# Initializes the loop state file for the stop hook

set -euo pipefail

LOOP_FILE=".claude/ralph-loop.local.md"

# Parse arguments
MAX_ITERATIONS=""
COMPLETION_PROMISE=""
PROMPT=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --max-iterations)
            MAX_ITERATIONS="$2"
            shift 2
            ;;
        --completion-promise)
            COMPLETION_PROMISE="$2"
            shift 2
            ;;
        *)
            if [[ -z "$PROMPT" ]]; then
                PROMPT="$1"
            else
                PROMPT="$PROMPT $1"
            fi
            shift
            ;;
    esac
done

if [[ -z "$PROMPT" ]]; then
    echo "Error: No prompt provided" >&2
    echo "Usage: setup-ralph-loop.sh \"<prompt>\" [--max-iterations <n>] [--completion-promise \"<text>\"]" >&2
    exit 1
fi

# Create .claude directory if needed
mkdir -p .claude

# Create loop state file
cat > "$LOOP_FILE" << EOF
---
iteration: 1
max_iterations: ${MAX_ITERATIONS:-}
completion_promise: ${COMPLETION_PROMISE:-}
created: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
---

$PROMPT
EOF

echo "Ralph loop initialized. The stop hook will now intercept exit attempts."
echo ""
echo "Loop settings:"
echo "  Max iterations: ${MAX_ITERATIONS:-unlimited}"
echo "  Completion promise: ${COMPLETION_PROMISE:-none}"
echo ""
echo "To cancel: /cancel-ralph"
echo "To check progress: grep '^iteration:' $LOOP_FILE"
