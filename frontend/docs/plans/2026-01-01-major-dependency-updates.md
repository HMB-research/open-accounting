# Major Dependency Updates Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Update frontend dependencies to latest major versions (Vite 7.3.0, @sveltejs/vite-plugin-svelte 6.2.1, @types/node 25.0.3)

**Architecture:** Incremental updates with verification at each step. Update dependencies in order of dependency chain: tsconfig first (for verbatimModuleSyntax), then vite-plugin-svelte, then Vite, finally @types/node.

**Tech Stack:** SvelteKit 2.49.2, Svelte 5.16.0, Vite, TypeScript, Vitest, Playwright

---

## Pre-requisites

- Node.js 22.12+ (current: 22.21.1 - OK)
- All tests passing before starting
- Clean git state

---

### Task 1: Verify Current State

**Files:**
- Check: `package.json`

**Step 1: Run all tests to establish baseline**

Run: `npm test`
Expected: All tests pass

**Step 2: Run build to verify current state**

Run: `npm run build`
Expected: Build succeeds

**Step 3: Commit baseline (if any uncommitted changes)**

```bash
git status
```
Expected: Clean working directory (or commit any pending changes)

---

### Task 2: Add verbatimModuleSyntax to TypeScript Config

**Files:**
- Modify: `tsconfig.json`

**Step 1: Read current tsconfig.json**

Verify the current configuration to understand baseline.

**Step 2: Add verbatimModuleSyntax option**

Add to compilerOptions:
```json
{
  "compilerOptions": {
    "verbatimModuleSyntax": true,
    // ... existing options
  }
}
```

**Step 3: Run TypeScript check**

Run: `npm run check`
Expected: PASS (or identify any type import issues to fix)

**Step 4: Fix any type import issues**

If check fails, update imports from:
```typescript
import { SomeType } from './module';
```
To:
```typescript
import type { SomeType } from './module';
```

**Step 5: Run tests to verify no regression**

Run: `npm test`
Expected: All tests pass

**Step 6: Commit**

```bash
git add tsconfig.json
git commit -m "chore: add verbatimModuleSyntax for vite-plugin-svelte 6 compatibility"
```

---

### Task 3: Update @sveltejs/vite-plugin-svelte to 6.2.1

**Files:**
- Modify: `package.json`
- Verify: `svelte.config.js` (vitePreprocess import)

**Step 1: Update the dependency**

Run: `npm install @sveltejs/vite-plugin-svelte@^6.2.1`
Expected: Package installs successfully

**Step 2: Verify vitePreprocess import still works**

The import in svelte.config.js should still work:
```javascript
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';
```

Run: `npm run check`
Expected: Check passes

**Step 3: Run all tests**

Run: `npm test`
Expected: All tests pass

**Step 4: Run build**

Run: `npm run build`
Expected: Build succeeds

**Step 5: Commit**

```bash
git add package.json package-lock.json
git commit -m "chore: update @sveltejs/vite-plugin-svelte to 6.2.1"
```

---

### Task 4: Update Vite to 7.3.0

**Files:**
- Modify: `package.json`
- Verify: `vite.config.ts`

**Step 1: Update the dependency**

Run: `npm install vite@^7.3.0`
Expected: Package installs successfully

**Step 2: Run TypeScript check**

Run: `npm run check`
Expected: Check passes

**Step 3: Run all tests**

Run: `npm test`
Expected: All tests pass

**Step 4: Run dev server briefly to verify**

Run: `npm run dev`
Expected: Dev server starts without errors (Ctrl+C to exit)

**Step 5: Run build**

Run: `npm run build`
Expected: Build succeeds

**Step 6: Commit**

```bash
git add package.json package-lock.json
git commit -m "chore: update vite to 7.3.0"
```

---

### Task 5: Update @types/node to 25.0.3

**Files:**
- Modify: `package.json`

**Step 1: Update the dependency**

Run: `npm install @types/node@^25.0.3`
Expected: Package installs successfully

**Step 2: Run TypeScript check**

Run: `npm run check`
Expected: Check passes (or identify any Node.js API breaking changes)

**Step 3: Run all tests**

Run: `npm test`
Expected: All tests pass

**Step 4: Run build**

Run: `npm run build`
Expected: Build succeeds

**Step 5: Commit**

```bash
git add package.json package-lock.json
git commit -m "chore: update @types/node to 25.0.3"
```

---

### Task 6: Run E2E Tests

**Files:**
- Verify: All E2E test specs

**Step 1: Run E2E test suite**

Run: `npm run test:e2e`
Expected: All E2E tests pass

**Step 2: If failures, debug and fix**

Run: `npm run test:e2e:debug`
Fix any issues related to the dependency updates.

---

### Task 7: Create Pull Request

**Step 1: Push branch**

```bash
git push -u origin chore/major-dependency-updates
```

**Step 2: Create PR**

```bash
gh pr create --title "chore: major frontend dependency updates" --body "$(cat <<'EOF'
## Summary
- Update @sveltejs/vite-plugin-svelte 5.0.3 → 6.2.1
- Update vite 6.0.6 → 7.3.0
- Update @types/node 22.10.2 → 25.0.3
- Add verbatimModuleSyntax to tsconfig.json (required for vite-plugin-svelte 6)

## Breaking Changes Addressed
- **vite-plugin-svelte 6**: Added `verbatimModuleSyntax: true` to tsconfig.json
- **Vite 7**: No config changes needed (Node 22.21.1 meets 22.12+ requirement)
- **@types/node 25**: Verified no breaking API changes affect our code

## Test plan
- [x] All unit tests pass
- [x] TypeScript check passes
- [x] Build succeeds
- [x] E2E tests pass
EOF
)"
```

**Step 3: Verify CI passes**

Run: `gh pr checks`
Expected: All checks pass

---

## Rollback Plan

If any step fails catastrophically:

```bash
git reset --hard HEAD~N  # N = number of commits to undo
npm install              # Restore original packages
```

---

## Known Breaking Changes Reference

### Vite 7.0
- Node.js 20.19+ or 22.12+ required (we have 22.21.1)
- Sass legacy API removed (we don't use Sass)
- splitVendorChunkPlugin removed (we don't use it)
- transformIndexHtml hook changes (handled by SvelteKit)

### @sveltejs/vite-plugin-svelte 6.0
- Requires verbatimModuleSyntax in tsconfig (Task 2)
- CommonJS config removed (we use ESM)
- Node 18 no longer supported (we use Node 22)
- Preprocess/compile split (handled internally by SvelteKit)

### @types/node 25
- Node.js 22+ type definitions
- Minor API type changes (verify at Task 5 Step 2)
