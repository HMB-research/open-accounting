---
name: cancel-ralph
description: Cancel active Ralph Wiggum loop
allowed-tools: Bash
---

# Cancel Ralph Loop

Stop the currently running Ralph loop.

## Execute

1. Check if a loop is active:

```bash
if [[ -f .claude/ralph-loop.local.md ]]; then
    ITERATION=$(grep '^iteration:' .claude/ralph-loop.local.md | sed 's/^iteration:[[:space:]]*//')
    rm .claude/ralph-loop.local.md
    echo "Cancelled Ralph loop (was at iteration $ITERATION)"
else
    echo "No active Ralph loop found."
fi
```
