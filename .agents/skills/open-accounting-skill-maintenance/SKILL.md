---
name: open-accounting-skill-maintenance
description: Use when updating or creating open-accounting repo skills after workflow friction, repeated mistakes, user preference changes, staged PR learnings, or requests to improve the development flow.
---

# Open Accounting Skill Maintenance

Use this with `open-accounting-development` when the user asks to update skills or a stage reveals guidance future agents should follow.

## Update Rules

- Prefer updating the focused existing skill that owns the workflow. Create a new skill only when the trigger is distinct and likely to recur.
- Keep guidance concise and operational. Skills should say what to do, when to do it, and which command or file proves it.
- Encode repo-specific expectations, not generic engineering advice.
- Preserve the project defaults: ORM/repository persistence, reusable services/mappers, no legacy fallback paths, staged local gates, PR check follow-through, and exact evidence reporting.
- When a skill references another skill, add the relationship in `open-accounting-development` or the nearest coordinator skill so future activation is discoverable.
- Do not add auxiliary docs such as README or changelog files inside skill folders.

## Validation

After skill edits:

```bash
git diff --check -- .agents/skills
```

If skill changes accompany code changes, include them in the same stage commit after the relevant product gates pass.
