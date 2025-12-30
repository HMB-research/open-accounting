#!/usr/bin/env bash
# Ralph Wiggum Stop Hook
# Implements self-referential loop by intercepting exit attempts

set -euo pipefail

LOOP_FILE=".claude/ralph-loop.local.md"

# Exit early if no active loop
if [[ ! -f "$LOOP_FILE" ]]; then
    echo '{"decision": "allow"}'
    exit 0
fi

# Read loop state
ITERATION=$(grep '^iteration:' "$LOOP_FILE" | head -1 | sed 's/^iteration:[[:space:]]*//')
MAX_ITERATIONS=$(grep '^max_iterations:' "$LOOP_FILE" | head -1 | sed 's/^max_iterations:[[:space:]]*//')
COMPLETION_PROMISE=$(grep '^completion_promise:' "$LOOP_FILE" | head -1 | sed 's/^completion_promise:[[:space:]]*//')
PROMPT=$(sed -n '/^---$/,/^---$/!p' "$LOOP_FILE" | tail -n +1)

# Validate iteration is numeric
if ! [[ "$ITERATION" =~ ^[0-9]+$ ]]; then
    echo "Warning: Invalid iteration value '$ITERATION', resetting to 1" >&2
    ITERATION=1
fi

# Check max iterations
if [[ -n "$MAX_ITERATIONS" ]] && [[ "$MAX_ITERATIONS" =~ ^[0-9]+$ ]]; then
    if (( ITERATION >= MAX_ITERATIONS )); then
        echo "Max iterations ($MAX_ITERATIONS) reached. Exiting loop." >&2
        rm -f "$LOOP_FILE"
        echo '{"decision": "allow"}'
        exit 0
    fi
fi

# Check for completion promise in transcript
if [[ -n "$COMPLETION_PROMISE" ]]; then
    TRANSCRIPT_FILE="${CLAUDE_TRANSCRIPT:-}"
    if [[ -n "$TRANSCRIPT_FILE" ]] && [[ -f "$TRANSCRIPT_FILE" ]]; then
        LAST_MESSAGE=$(jq -r 'select(.role == "assistant") | .content' "$TRANSCRIPT_FILE" 2>/dev/null | tail -1 || true)

        # Check for promise tag
        if echo "$LAST_MESSAGE" | perl -ne "exit 0 if /<promise>.*${COMPLETION_PROMISE}.*<\/promise>/s; exit 1" 2>/dev/null; then
            echo "Completion promise detected. Exiting loop." >&2
            rm -f "$LOOP_FILE"
            echo '{"decision": "allow"}'
            exit 0
        fi
    fi
fi

# Increment iteration
NEW_ITERATION=$((ITERATION + 1))
TEMP_FILE=$(mktemp)
sed "s/^iteration:.*/iteration: $NEW_ITERATION/" "$LOOP_FILE" > "$TEMP_FILE"
mv "$TEMP_FILE" "$LOOP_FILE"

# Block exit and feed prompt back
SYSTEM_MSG="[Ralph Loop Iteration $NEW_ITERATION"
if [[ -n "$MAX_ITERATIONS" ]]; then
    SYSTEM_MSG="$SYSTEM_MSG of $MAX_ITERATIONS"
fi
SYSTEM_MSG="$SYSTEM_MSG]"

cat << EOF
{
  "decision": "block",
  "message": $(echo "$PROMPT" | jq -Rs .),
  "system_message": "$SYSTEM_MSG"
}
EOF
