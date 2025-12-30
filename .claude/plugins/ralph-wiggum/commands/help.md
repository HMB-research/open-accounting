---
name: ralph-help
description: Show help for Ralph Wiggum plugin
allowed-tools: []
---

# Ralph Wiggum Plugin Help

The Ralph Wiggum technique enables iterative, self-referential AI development loops.

## Core Concept

"Ralph is a Bash loop" - a `while true` that continuously feeds an AI agent a prompt,
allowing it to iteratively improve its work until completion.

## Commands

### `/ralph-loop "<prompt>" [options]`

Start an iterative loop:

- `--max-iterations <n>` - Stop after N iterations (default: unlimited)
- `--completion-promise "<text>"` - Phrase that signals completion

**Example:**
```bash
/ralph-loop "Increase test coverage to 95%. Run tests after each change. Output <promise>COVERAGE_95</promise> when coverage reaches 95%." --max-iterations 50 --completion-promise "COVERAGE_95"
```

### `/cancel-ralph`

Cancel the active Ralph loop.

## How Completion Works

1. Define a completion promise (e.g., "TESTS_PASS")
2. Work iteratively toward the goal
3. When genuinely complete, output: `<promise>TESTS_PASS</promise>`
4. The stop hook detects this and exits the loop

## Best Practices

1. **Set iteration limits** - Always use `--max-iterations` as a safety net
2. **Clear success criteria** - Define measurable completion conditions
3. **Use TDD** - Let tests verify your progress automatically
4. **Incremental goals** - Break complex tasks into phases

## When to Use Ralph

**Good for:**
- Tasks with clear success criteria (tests passing, coverage targets)
- Iterative refinement (bug fixes, code improvements)
- Greenfield projects where you can walk away
- Automated verification (linters, type checkers)

**Not good for:**
- Tasks requiring human judgment
- Design decisions
- Production debugging
- Ambiguous requirements

## Monitoring Progress

Check current iteration:
```bash
grep '^iteration:' .claude/ralph-loop.local.md
```

View loop state:
```bash
cat .claude/ralph-loop.local.md
```
