# SmartAccounts Sync Coverage Tracker

Updated: 2026-09-01

This is the canonical checklist for the three-source SmartAccounts → Open Accounting batch. It tracks source capture, OA target capability, import status, and reconciliation separately. A target table being empty is not evidence that the SmartAccounts source is empty.

## Status rules

| Value | Meaning |
|---|---|
| `EXACT` | OA has a direct target record and the cutover layer has a matching import kind. Field-level 1:1 parity still requires a captured source contract. |
| `TRANSFORMED` | OA supports the business data, but source records change grain or split across OA entities. |
| `DERIVED` | This is a report/output, not a record import. OA must reproduce it from imported source-of-truth records. |
| `PARTIAL` | OA has related capability, but the source contract, adapter, or semantics are incomplete. |
| `NONE` | No approved 1:1 OA destination exists. Preserve as evidence or add a target model. |
| `VERIFIED_IMPORTED` | Canonical source identities and OA rows were reconciled successfully. |
| `NOT_CAPTURED` | No authoritative source artifact for this resource was included in the batch. Source count is unknown. |
| `EMPTY_TARGET` | OA has zero target rows. This is a gap unless an authoritative zero-record source export is captured. |
| `DERIVED_VERIFIED` | OA report output is non-empty and consistent with the posted GL over the source period. |

## Applied batch snapshot

| Source | Accounts | Journal entries | Journal lines | Account mapping | Posting/integrity |
|---|---:|---:|---:|---|---|
| `177481` | 15 | 51 | 110 | `VERIFIED_IMPORTED` | `VERIFIED_IMPORTED` |
| `84355` | 121 | 9,336 | 23,306 | `VERIFIED_IMPORTED` | `VERIFIED_IMPORTED` |
| `177750` | 10 | 14 | 33 | `VERIFIED_IMPORTED` | `VERIFIED_IMPORTED` |
| **Total** | **146** | **9,401** | **23,449** | **0 mismatches** | **0 drafts, imbalances, or duplicate source identities** |

## Browser/export resource checklist

| SmartAccounts resource | Source shape | OA destination | 1:1 support | Capture status | Import status | Reconciliation / missing coverage |
|---|---|---|---|---|---|---|
| `account_turnover` | CSV report | Account turnover/trial-balance reporting | `DERIVED` | `NOT_CAPTURED` | `DERIVED_VERIFIED` from GL | Capture source report for formal report-to-report parity. |
| `annual_report` | Page/report | Annual report pack | `DERIVED` | `NOT_CAPTURED` | Available, not source-reconciled | Page-only source evidence and accountant parity remain missing. |
| `articles` | CSV/master | `products`, categories | `EXACT` target | `NOT_CAPTURED` | `EMPTY_TARGET` | Capture stable article IDs, tax, prices, units, and category references. |
| `balance_sheet` | CSV report | Balance sheet report | `DERIVED` | `NOT_CAPTURED` | GL-backed report available | Capture source report at the same cutoff and compare totals. |
| `bank_payments` | CSV/transaction | `bank_transactions`, `payments`, allocations | `TRANSFORMED` | `NOT_CAPTURED` | `EMPTY_TARGET` | Capture bank account, transaction, allocation, and reconciliation identities. |
| `cash_flow_statement` | CSV report | Cash-flow statement/analytics | `DERIVED` | `NOT_CAPTURED` | `DERIVED_VERIFIED` from GL | Exact date-range chart is supported; source statement parity evidence remains missing. |
| `cash_payments` | CSV/transaction | `payments`, allocations | `TRANSFORMED` | `NOT_CAPTURED` | `EMPTY_TARGET` | Capture payment direction, counterparty, method, date, currency, and allocation. |
| `client_invoices` | CSV/commercial detail | Sales `invoices`, lines, allocations | `EXACT` target | `NOT_CAPTURED` | `EMPTY_TARGET` | Commercial selector/pager and source field contract remain review-gated. |
| `client_offers` | CSV/document | `quotes`, quote lines | `EXACT` target | `NOT_CAPTURED` | `EMPTY_TARGET` | Capture status, lines, tax, currency, validity, and customer identity. |
| `client_orders` | CSV/document | `orders`, order lines | `EXACT` target | `NOT_CAPTURED` | `EMPTY_TARGET` | Capture lifecycle, lines, stock reservations, and customer identity. |
| `clients` | CSV/master detail | Customer `contacts` | `EXACT` target | `NOT_CAPTURED` | `EMPTY_TARGET` | Capture stable ID, code, tax identity, addresses, payment terms, and activity state. |
| `depreciations` | CSV/transaction | `depreciation_entries`, asset journals | `TRANSFORMED` | `NOT_CAPTURED` | `EMPTY_TARGET` | Capture asset link, period, method, amount, and posting identity. |
| `fixed_asset_depreciation_report` | CSV report | Fixed-asset/depreciation reports | `DERIVED` | `NOT_CAPTURED` | Not source-reconciled | Requires fixed assets and depreciation records first. |
| `fixed_assets` | CSV/master | `fixed_assets`, categories | `EXACT` target | `NOT_CAPTURED` | `EMPTY_TARGET` | Capture acquisition, useful life, method, residual value, account links, and status. |
| `general_ledger` | CSV/grouped ledger | Accounts, journal entries, journal lines | `TRANSFORMED` | Captured for all 3 sources | `VERIFIED_IMPORTED` | Complete for supplied cutoff: deterministic source IDs, posted and balanced. |
| `income_statement` | CSV report | Income statement/dashboard P&L | `DERIVED` | `NOT_CAPTURED` | `DERIVED_VERIFIED` from GL | Expense/revenue classifications and non-zero period activity verified. |
| `journal_entries` | CSV/entry detail | Journal entries and lines | `EXACT` target | `NOT_CAPTURED` separately | Covered only through GL transformation | Do not import separately until overlap/deduplication against authoritative GL is proven. |
| `other_reports` | Page/report family | Multiple OA reports/evidence | `PARTIAL` | `NOT_CAPTURED` | Not reconciled | Inventory exact report list and disposition per report. |
| `salaries` | CSV/payroll result | Payroll runs, payslips, salary components, journals | `TRANSFORMED` | `NOT_CAPTURED` | `EMPTY_TARGET` | Capture worker links, period, components, taxes, statuses, and posting links. |
| `tsd_returns` | CSV/declaration | `tsd_declarations`, `tsd_rows` | `EXACT` target | `NOT_CAPTURED` | `EMPTY_TARGET` | Capture declaration period/version/status and row identities. |
| `vat_returns` | CSV/declaration | `kmd_declarations`, `kmd_rows` | `EXACT` target | `NOT_CAPTURED` | `EMPTY_TARGET` | Capture period, status, boxes/rows, corrections, and filing evidence. |
| `vendor_invoices` | CSV/document | Purchase `invoices`, lines, expenses/documents | `EXACT` target | `NOT_CAPTURED` | `EMPTY_TARGET` | Capture supplier, lines, tax, currency, attachments, and payment state. |
| `vendor_orders` | CSV/document | Purchase-order equivalent | `PARTIAL` | `NOT_CAPTURED` | `EMPTY_TARGET` | Confirm OA order direction/semantics before enabling import. |
| `vendors` | CSV/master detail | Supplier `contacts` | `EXACT` target | `NOT_CAPTURED` | `EMPTY_TARGET` | Capture stable ID, code, tax identity, addresses, terms, and activity state. |
| `wage_reports` | Page/report | Payroll reports | `DERIVED` | `NOT_CAPTURED` | Not source-reconciled | Requires payroll source records and same-period report evidence. |
| `warehouse_inventory` | Page/snapshot | Warehouses, stock levels, lots | `TRANSFORMED` | `NOT_CAPTURED` | `EMPTY_TARGET` | Page-only quantity/valuation snapshot contract is missing. |
| `warehouse_movements` | CSV/movement | Inventory movements, lots, stock levels | `TRANSFORMED` | `NOT_CAPTURED` | `EMPTY_TARGET` | Capture warehouse/product IDs, movement type, quantity, cost, date, and source document. |
| `warehouse_movements_report` | Page/report | Inventory movement/valuation reports | `DERIVED` | `NOT_CAPTURED` | Not source-reconciled | Requires movement capture before report parity. |
| `warehouses` | Page/master | `warehouses` | `EXACT` target | `NOT_CAPTURED` | `EMPTY_TARGET` | Page-only stable warehouse identity and active-state contract is missing. |
| `worker_absences` | Page/leave record | Leave records and balances | `EXACT` target | `NOT_CAPTURED` | `EMPTY_TARGET` | Capture employee, absence type, dates, units, status, and balance effect. |
| `workers` | CSV/master | `employees`, salary components | `EXACT` target | `NOT_CAPTURED` | `EMPTY_TARGET` | Capture stable worker ID, employment/tax attributes, contracts, and active state. |

## Target domains without a complete dedicated source surface

| OA domain | OA support | Current status | Missing source coverage |
|---|---|---|---|
| Chart of accounts | Supported/imported | `VERIFIED_IMPORTED` | Derived from GL account sections; 146 imported accounts and 57 corrected classifications. |
| Documents and attachments | Supported | `EMPTY_TARGET` | Capture attachment manifests, parent identity, MIME type, size, and exact-byte evidence. |
| Bank accounts and reconciliations | Supported | `EMPTY_TARGET` | Capture bank-account masters, statement imports, match decisions, and reconciliation cutoffs. |
| Payment allocations | Supported | `EMPTY_TARGET` | Requires invoice/payment source identities and allocation records. |
| Recurring invoices | Supported | `EMPTY_TARGET` | No authoritative recurring-template source capture was supplied. |
| Cost centers and allocations | Supported | `EMPTY_TARGET` | No authoritative source dimension/allocation export was supplied. |

## Completion gates

- [x] General Ledger source files captured for all three sources.
- [x] Canonical account and journal counts match OA.
- [x] Imported account metadata is correct and idempotent.
- [x] All imported journals are posted, balanced, and source-identity unique.
- [x] Dashboard revenue/expense and exact-period cash inflow/outflow parity pass over each source period.
- [ ] Capture authoritative zero/non-zero source counts for every non-GL resource above.
- [ ] Complete field-level source→OA disposition for every captured schema.
- [ ] Import master data before dependent commercial/payroll/inventory records.
- [ ] Reconcile invoices/payments, AR/AP, bank, VAT/TSD, payroll, assets, inventory, and attachments 1:1.
- [ ] Obtain final accountant sign-off only after every applicable row is `VERIFIED_IMPORTED`, `DERIVED_VERIFIED`, or explicitly excluded with evidence.

## Update contract

Update this file after every capture or apply. Never promote `NOT_CAPTURED` or `EMPTY_TARGET` to complete based only on GL presence. For record resources, completion requires source count, target count, stable identity uniqueness, parent/child integrity, and zero unexplained differences. For reports, completion requires same-cutoff source and OA evidence with zero unexplained variance.
