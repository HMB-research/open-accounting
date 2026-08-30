# SmartAccounts Brave debugger handoff

**Updated:** 2026-08-27

## Required browser surface

- Use the user's **Brave** browser through the Chrome debugger MCP external
  browser binding (`extension`). The Chrome-debugger name does **not** mean
  Google Chrome must be installed.
- The Brave binding is available and the existing SmartAccounts tab at
  `https://sa.smartaccounts.eu/et/` is signed in to **Hold My Beer OÜ**.
- Reuse that tab. Name the browser session before claiming it, and do not
  reload it merely to discover state.
- Keep the tab active throughout this work. At each browser interaction (and
  at least once per longer implementation/test interval), detect the visible
  session-expiry prompt. If it explicitly says **Continue** or **Jätka** to
  remain signed in, click that button and no other page action. Record only
  that the prompt was continued; never record the page contents or session
  state. If no such prompt is visible, leave the tab untouched.
- If the binding becomes unavailable, ask the owner to enable the Codex/ChatGPT
  browser extension in Brave through **Settings → Computer use**. Do not
  substitute raw DevTools, CDP, Playwright, a separate browser, or a browser
  profile scraper.

## Authentication direction

- Implement a **no-copy/paste API-key** connection path. The intended UX is:
  the owner signs in once in Brave, approves a short-lived and revocable
  handoff, then starts the tenant-bound read-only capture.
- Do **not** claim that the handoff contract exists until it is observed while
  the owner is signed in and tested with a non-production fixture where
  possible.
- Keep browser authentication inside Brave. Never extract, persist, log,
  commit, display, or forward browser cookies, local storage, passwords,
  session tokens, authorization headers, or a browser profile to Open
  Accounting, server-nuc, or the bridge.
- The present API-key/secret connection is a temporary fallback only. It must
  not be silently selected as the no-key flow.
- The current local implementation provides a 10-minute, one-time Brave
  pairing token. It is delivered only through a same-window event to a
  locally installed extension, stored server-side as a SHA-256 hash, and can
  bind only an opaque `sa-browser-v1-...` UI selector to the issuing OA
  tenant. It does **not** yet capture or transfer SmartAccounts records.

## Read-only discovery rules

- Inspect only visible UI state and verified same-origin **read** requests.
  The authenticated app has read-only navigation for entries, sales invoices,
  vendor invoices, payments, reports, inventory, assets, and payroll.
- Record only redacted implementation metadata: HTTP method, path, names of
  non-sensitive request fields, pagination/change semantics, response shape,
  error semantics, and a schema/hash fixture.
- Do not save or print raw source records, balances, names, IDs, or request
  payloads. Source data may move only as a protected canonical package through
  the tenant-scoped archive receiver after the applicable confirmation.
- Do not guess undocumented endpoints. Keep missing account-balance and
  warehouse routes marked `brave_discovery_required` until they are observed
  and independently validated.
- On 2026-08-27, the signed-in Brave pages for `warehouses` and
  `worker_absences` were inspected without reading records. Each exposed only
  its existing POST filter form and no visible CSV/export affordance. They
  remain `page_only`; a future agent must obtain a separate read-only export
  contract rather than repurposing either form as an unverified export API.
- Browser discovery must never invoke save, delete, import, payment, tax,
  user, settings, credential, or other state-changing SmartAccounts actions.
  Stop for immediate owner confirmation before an action would create a
  credential, change source settings, or transmit financial data.

## Open Accounting policy retained

- SmartAccounts general ledger is authoritative for Hold My Beer OÜ.
- Apply only reviewed, balanced GL journals after explicit final confirmation.
  Invoices, payments, attachments, and other non-GL records remain
  evidence/archive-only unless the authority policy is changed explicitly.
- Preserve tenant/source binding, idempotency, revision/tombstone review,
  source-as-of cutoff, date-window coverage, and reconciliation gates. A
  browser-backed path may not bypass them.
