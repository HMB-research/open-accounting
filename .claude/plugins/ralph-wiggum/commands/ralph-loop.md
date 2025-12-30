---
name: ralph-loop
description: Start a self-referential Ralph Wiggum loop for iterative task execution
allowed-tools: Bash
---

# Ralph Loop Command

Initialize a Ralph loop that runs your prompt iteratively until completion.

## Usage

```bash
/ralph-loop "<prompt>" [--max-iterations <n>] [--completion-promise "<text>"]
```

## Options

- `--max-iterations <n>`: Stop after N iterations (safety limit)
- `--completion-promise "<text>"`: Text that signals task completion

## How It Works

1. This command creates a state file at `.claude/ralph-loop.local.md`
2. The stop hook intercepts exit attempts and feeds the same prompt back
3. You see your previous work in files and git history each iteration
4. Loop ends when: max iterations reached, promise detected, or manually cancelled

## Completion Promise

To signal completion, output the promise in XML tags:

```xml
<promise>YOUR_COMPLETION_TEXT</promise>
```

**CRITICAL**: Only output the promise when the statement is **completely and unequivocally TRUE**.
Do NOT lie to exit the loop. The loop is designed to verify genuine completion.

## Execute

Run the setup script with the provided arguments:

```bash
"${CLAUDE_PLUGIN_ROOT}/scripts/setup-ralph-loop.sh" $ARGUMENTS
```

After setup, continue working on the task. The stop hook will handle iteration.
