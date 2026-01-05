# Claude Skill Design: open-accounting-development

**Date**: 2026-01-05
**Status**: Implemented

## Purpose

Create a project-local Claude skill that provides development context for open-accounting, focusing on:
- Multi-tenant architecture understanding
- Testing strategy (especially E2E for demo interface)
- Layer responsibilities and debugging guidance

## Problem

When working on open-accounting, Claude often needs repeated context about:
1. How multi-tenant data flows through the stack
2. Which test type to use for different layers
3. Demo mode specifics (seeding, reset, parallel testing)
4. Documentation update requirements

## Solution

A skill file at `.claude/skills/open-accounting-development/SKILL.md` that auto-loads for this project and provides:

### Architecture Context
- Multi-tenant data flow diagram
- Layer responsibility matrix (Handler vs Service vs Repository)
- Key files for debugging tenant issues

### Testing Strategy
- Decision tree for choosing test type
- Coverage targets (90%+ backend, 95%+ frontend, 100% demo E2E)
- Demo E2E priority list

### Demo Mode Reference
- Credentials and URLs
- Seeding flow explanation
- Key files for demo functionality
- Multi-user parallel testing details
- Debugging checklist

### Documentation Checklist
- Which docs to update for different change types
- Plan document conventions
- Commit message format

## Trade-offs Considered

1. **Single skill vs multiple skills**: Chose single comprehensive skill to avoid context fragmentation
2. **Detail level**: High-level principles + decision trees, not step-by-step tutorials (Claude can read code)
3. **Location**: Project-local (`.claude/skills/`) over global (`~/.claude/`) for project-specific focus

## File Location

```
.claude/skills/open-accounting-development/SKILL.md
```
