<script lang="ts">
  import { browser } from "$app/environment";
  import { page } from "$app/stores";
  import { onMount } from "svelte";
  import {
    api,
    type Account,
    type CreateJournalEntryRequest,
    type ImportOpeningBalancesResult,
    type JournalEntry,
  } from "$lib/api";
  import Decimal from "decimal.js";
  import * as m from "$lib/paraglide/messages.js";

  type JournalFormLine = {
    accountId: string;
    description: string;
    debit: string;
    credit: string;
  };

  function todayISODate(): string {
    return new Date().toISOString().split("T")[0];
  }

  function openingBalanceDefaultDate(): string {
    return `${new Date().getFullYear()}-01-01`;
  }

  function defaultOpeningBalanceReference(): string {
    return `OB-${new Date().getFullYear()}`;
  }

  function defaultJournalLines(): JournalFormLine[] {
    return [
      { accountId: "", description: "", debit: "", credit: "" },
      { accountId: "", description: "", debit: "", credit: "" },
    ];
  }

  let tenantId = $derived($page.url.searchParams.get("tenant") || "");
  let entries = $state<JournalEntry[]>([]);
  let accounts = $state<Account[]>([]);
  let isLoading = $state(true);
  let error = $state("");

  let showCreate = $state(false);
  let showImportOpeningBalances = $state(false);
  let formError = $state("");
  let openingBalanceImportError = $state("");
  let isImportingOpeningBalances = $state(false);
  let openingBalanceImportResult = $state<ImportOpeningBalancesResult | null>(
    null,
  );
  let openingBalanceFileName = $state("");
  let openingBalanceCSVContent = $state("");

  let entryDate = $state(todayISODate());
  let description = $state("");
  let reference = $state("");
  let lines = $state<JournalFormLine[]>(defaultJournalLines());

  let openingBalanceEntryDate = $state(openingBalanceDefaultDate());
  let openingBalanceDescription = $state(m.journal_importDefaultDescription());
  let openingBalanceReference = $state(defaultOpeningBalanceReference());

  // We don't have a list endpoint in the API yet, so we only show entries created in-session.
  onMount(async () => {
    if (!tenantId) {
      error = m.journal_noTenantSelected();
      isLoading = false;
      return;
    }

    try {
      accounts = await api.listAccounts(tenantId, true);
    } catch (err) {
      error =
        err instanceof Error ? err.message : m.journal_failedToLoadAccounts();
    } finally {
      isLoading = false;
    }
  });

  function resetCreateForm() {
    entryDate = todayISODate();
    description = "";
    reference = "";
    lines = defaultJournalLines();
    formError = "";
  }

  function resetOpeningBalanceImportState() {
    openingBalanceImportError = "";
    openingBalanceImportResult = null;
    openingBalanceFileName = "";
    openingBalanceCSVContent = "";
    openingBalanceEntryDate = openingBalanceDefaultDate();
    openingBalanceDescription = m.journal_importDefaultDescription();
    openingBalanceReference = defaultOpeningBalanceReference();
  }

  function openOpeningBalanceImportModal() {
    resetOpeningBalanceImportState();
    showImportOpeningBalances = true;
  }

  function closeOpeningBalanceImportModal() {
    showImportOpeningBalances = false;
    resetOpeningBalanceImportState();
  }

  function addLine() {
    lines = [
      ...lines,
      { accountId: "", description: "", debit: "", credit: "" },
    ];
  }

  function removeLine(index: number) {
    if (lines.length > 2) {
      lines = lines.filter((_, i) => i !== index);
    }
  }

  function getTotalDebits(): Decimal {
    return lines.reduce((sum, line) => {
      const amount = line.debit ? new Decimal(line.debit) : new Decimal(0);
      return sum.plus(amount);
    }, new Decimal(0));
  }

  function getTotalCredits(): Decimal {
    return lines.reduce((sum, line) => {
      const amount = line.credit ? new Decimal(line.credit) : new Decimal(0);
      return sum.plus(amount);
    }, new Decimal(0));
  }

  function isBalanced(): boolean {
    return getTotalDebits().equals(getTotalCredits());
  }

  async function createEntry(e: Event) {
    e.preventDefault();
    formError = "";

    if (!isBalanced()) {
      formError = m.journal_debitsMustEqualCredits();
      return;
    }

    const validLines = lines.filter(
      (l) => l.accountId && (l.debit || l.credit),
    );
    if (validLines.length < 2) {
      formError = m.journal_minTwoLines();
      return;
    }

    try {
      const request: CreateJournalEntryRequest = {
        entry_date: entryDate,
        description,
        reference: reference || undefined,
        lines: validLines.map((l) => ({
          account_id: l.accountId,
          description: l.description || undefined,
          debit_amount: l.debit || "0",
          credit_amount: l.credit || "0",
        })),
      };

      const entry = await api.createJournalEntry(tenantId, request);
      entries = [entry, ...entries];
      showCreate = false;
      resetCreateForm();
    } catch (err) {
      formError =
        err instanceof Error ? err.message : m.journal_failedToCreateEntry();
    }
  }

  async function handleOpeningBalanceFileChange(event: Event) {
    const input = event.currentTarget as HTMLInputElement | null;
    const file = input?.files?.[0];

    openingBalanceImportResult = null;

    if (!file) {
      openingBalanceFileName = "";
      openingBalanceCSVContent = "";
      return;
    }

    openingBalanceFileName = file.name;
    openingBalanceCSVContent = await file.text();
    openingBalanceImportError = "";
  }

  function downloadOpeningBalanceTemplate() {
    if (!browser) return;

    const template = [
      "account_code,debit,credit,description",
      "1000,1500.00,0,Cash opening balance",
      "3000,0,1500.00,Owner equity opening balance",
    ].join("\n");

    const blob = new Blob([template], { type: "text/csv;charset=utf-8" });
    const url = window.URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = "opening-balances-template.csv";
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    window.URL.revokeObjectURL(url);
  }

  async function submitOpeningBalanceImport(event: Event) {
    event.preventDefault();

    if (!openingBalanceCSVContent.trim()) {
      openingBalanceImportError = m.journal_importFileRequired();
      return;
    }

    isImportingOpeningBalances = true;
    openingBalanceImportError = "";

    try {
      const result = await api.importOpeningBalances(tenantId, {
        file_name: openingBalanceFileName || undefined,
        entry_date: openingBalanceEntryDate,
        description: openingBalanceDescription || undefined,
        reference: openingBalanceReference || undefined,
        csv_content: openingBalanceCSVContent,
      });

      openingBalanceImportResult = result;
      entries = [
        result.journal_entry,
        ...entries.filter((entry) => entry.id !== result.journal_entry.id),
      ];
    } catch (err) {
      openingBalanceImportError =
        err instanceof Error
          ? err.message
          : m.journal_failedToImportOpeningBalances();
    } finally {
      isImportingOpeningBalances = false;
    }
  }

  async function postEntry(entry: JournalEntry) {
    try {
      await api.postJournalEntry(tenantId, entry.id);
      entry.status = "POSTED";
      entries = [...entries];
    } catch (err) {
      error =
        err instanceof Error ? err.message : m.journal_failedToPostEntry();
    }
  }

  async function voidEntry(entry: JournalEntry) {
    const reason = prompt(m.journal_voidReasonPrompt());
    if (!reason) return;

    try {
      await api.voidJournalEntry(tenantId, entry.id, reason);
      entry.status = "VOIDED";
      entry.void_reason = reason;
      entries = [...entries];
    } catch (err) {
      error =
        err instanceof Error ? err.message : m.journal_failedToVoidEntry();
    }
  }

  function formatAmount(amount: Decimal | undefined): string {
    if (!amount || amount.isZero()) return "-";
    return amount.toFixed(2);
  }

  function getAccountName(accountId: string): string {
    const account = accounts.find((a) => a.id === accountId);
    return account ? `${account.code} - ${account.name}` : accountId;
  }
</script>

<svelte:head>
  <title>{m.journal_title()} - Open Accounting</title>
</svelte:head>

<div class="container">
  <div class="page-header">
    <h1>{m.journal_title()}</h1>
    {#if tenantId}
      <div class="page-actions">
        <button
          class="btn btn-secondary"
          onclick={openOpeningBalanceImportModal}
        >
          {m.journal_importOpeningBalances()}
        </button>
        <button class="btn btn-primary" onclick={() => (showCreate = true)}>
          {m.journal_newEntry()}
        </button>
      </div>
    {/if}
  </div>

  {#if error}
    <div class="alert alert-error">{error}</div>
  {/if}

  {#if isLoading}
    <p>{m.common_loading()}</p>
  {:else if !tenantId}
    <div class="card empty-state">
      <p>
        {m.journal_selectTenantDashboard()}
        <a href="/dashboard">{m.nav_dashboard()}</a>.
      </p>
    </div>
  {:else if entries.length === 0 && !showCreate && !showImportOpeningBalances}
    <div class="card empty-state">
      <h2>{m.journal_noEntriesTitle()}</h2>
      <p>{m.journal_noEntriesDesc()}</p>
      <div class="empty-actions">
        <button
          class="btn btn-secondary"
          onclick={openOpeningBalanceImportModal}
        >
          {m.journal_importOpeningBalances()}
        </button>
        <button class="btn btn-primary" onclick={() => (showCreate = true)}>
          {m.journal_createJournalEntry()}
        </button>
      </div>
    </div>
  {:else}
    <div class="entries-list">
      {#each entries as entry}
        <div class="card entry-card">
          <div class="entry-header">
            <div class="entry-info">
              <span class="entry-number">#{entry.entry_number}</span>
              <span class="entry-date">{entry.entry_date}</span>
              <span class="badge status-{entry.status.toLowerCase()}"
                >{entry.status}</span
              >
            </div>
            <div class="entry-actions">
              {#if entry.status === "DRAFT"}
                <button
                  class="btn btn-sm btn-primary"
                  onclick={() => postEntry(entry)}
                >
                  {m.journal_post()}
                </button>
              {/if}
              {#if entry.status === "POSTED"}
                <button
                  class="btn btn-sm btn-danger"
                  onclick={() => voidEntry(entry)}
                >
                  {m.journal_void()}
                </button>
              {/if}
            </div>
          </div>

          <p class="entry-description">{entry.description}</p>
          {#if entry.reference}
            <p class="entry-reference">{m.journal_ref()} {entry.reference}</p>
          {/if}

          <div class="table-container">
            <table class="lines-table">
              <thead>
                <tr>
                  <th>{m.journal_account()}</th>
                  <th class="hide-mobile">{m.common_description()}</th>
                  <th class="amount">{m.journal_debit()}</th>
                  <th class="amount">{m.journal_credit()}</th>
                </tr>
              </thead>
              <tbody>
                {#each entry.lines as line}
                  <tr>
                    <td>{getAccountName(line.account_id)}</td>
                    <td class="hide-mobile">{line.description || "-"}</td>
                    <td class="amount">{formatAmount(line.debit_amount)}</td>
                    <td class="amount">{formatAmount(line.credit_amount)}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>

          {#if entry.void_reason}
            <p class="void-reason">
              {m.journal_voidReason()}
              {entry.void_reason}
            </p>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>

{#if showImportOpeningBalances}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <div
    class="modal-backdrop"
    onclick={closeOpeningBalanceImportModal}
    role="presentation"
  >
    <div
      class="modal card"
      onclick={(e) => e.stopPropagation()}
      role="dialog"
      aria-modal="true"
      aria-labelledby="import-opening-balances-title"
      tabindex="-1"
    >
      <h2 id="import-opening-balances-title">
        {m.journal_importOpeningBalances()}
      </h2>
      <p class="import-description">{m.journal_importDescription()}</p>

      {#if openingBalanceImportError}
        <div class="alert alert-error">{openingBalanceImportError}</div>
      {/if}

      <form onsubmit={submitOpeningBalanceImport}>
        <div class="form-row">
          <div class="form-group">
            <label class="label" for="opening-balance-entry-date"
              >{m.common_date()}</label
            >
            <input
              class="input"
              type="date"
              id="opening-balance-entry-date"
              bind:value={openingBalanceEntryDate}
              required
            />
          </div>
          <div class="form-group">
            <label class="label" for="opening-balance-reference"
              >{m.journal_reference()}</label
            >
            <input
              class="input"
              type="text"
              id="opening-balance-reference"
              bind:value={openingBalanceReference}
              placeholder={m.journal_referencePlaceholder()}
            />
          </div>
        </div>

        <div class="form-group">
          <label class="label" for="opening-balance-description"
            >{m.common_description()}</label
          >
          <input
            class="input"
            type="text"
            id="opening-balance-description"
            bind:value={openingBalanceDescription}
            placeholder={m.journal_descriptionPlaceholder()}
          />
        </div>

        <div class="form-group">
          <label class="label" for="opening-balance-file"
            >{m.journal_importChooseFile()}</label
          >
          <input
            class="input"
            id="opening-balance-file"
            type="file"
            accept=".csv,text/csv"
            onchange={handleOpeningBalanceFileChange}
          />
          <p class="form-hint">{m.journal_importTemplateHint()}</p>
          {#if openingBalanceFileName}
            <p class="selected-file">
              {m.journal_importSelectedFile()}:
              <span>{openingBalanceFileName}</span>
            </p>
          {/if}
        </div>

        <div class="import-toolbar">
          <button
            type="button"
            class="btn btn-secondary"
            onclick={downloadOpeningBalanceTemplate}
          >
            {m.journal_importTemplate()}
          </button>
        </div>

        <div class="modal-actions">
          <button
            type="button"
            class="btn btn-secondary"
            onclick={closeOpeningBalanceImportModal}
          >
            {m.common_cancel()}
          </button>
          <button
            type="submit"
            class="btn btn-primary"
            disabled={isImportingOpeningBalances}
          >
            {#if isImportingOpeningBalances}
              {m.journal_importing()}
            {:else}
              {m.journal_importOpeningBalances()}
            {/if}
          </button>
        </div>
      </form>

      {#if openingBalanceImportResult}
        <div class="import-summary">
          <h3>{m.journal_importSummary()}</h3>
          <div class="summary-grid">
            <div class="summary-card">
              <span class="summary-label"
                >{m.journal_importRowsProcessed()}</span
              >
              <strong>{openingBalanceImportResult.rows_processed}</strong>
            </div>
            <div class="summary-card">
              <span class="summary-label"
                >{m.journal_importLinesImported()}</span
              >
              <strong>{openingBalanceImportResult.lines_imported}</strong>
            </div>
            <div class="summary-card">
              <span class="summary-label">{m.journal_importTotalDebit()}</span>
              <strong
                >{formatAmount(openingBalanceImportResult.total_debit)}</strong
              >
            </div>
            <div class="summary-card">
              <span class="summary-label">{m.journal_importTotalCredit()}</span>
              <strong
                >{formatAmount(openingBalanceImportResult.total_credit)}</strong
              >
            </div>
          </div>

          <div class="alert alert-success">
            {m.journal_importCreatedEntry()}
            {openingBalanceImportResult.journal_entry.entry_number}
          </div>
        </div>
      {/if}
    </div>
  </div>
{/if}

{#if showCreate}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <div
    class="modal-backdrop"
    onclick={() => (showCreate = false)}
    role="presentation"
  >
    <div
      class="modal card modal-lg"
      onclick={(e) => e.stopPropagation()}
      role="dialog"
      aria-modal="true"
      aria-labelledby="create-journal-title"
      tabindex="-1"
    >
      <h2 id="create-journal-title">{m.journal_createJournalEntry()}</h2>

      {#if formError}
        <div class="alert alert-error">{formError}</div>
      {/if}

      <form onsubmit={createEntry}>
        <div class="form-row">
          <div class="form-group">
            <label class="label" for="entryDate">{m.common_date()}</label>
            <input
              class="input"
              type="date"
              id="entryDate"
              bind:value={entryDate}
              required
            />
          </div>
          <div class="form-group">
            <label class="label" for="reference">{m.journal_reference()}</label>
            <input
              class="input"
              type="text"
              id="reference"
              bind:value={reference}
              placeholder={m.journal_referencePlaceholder()}
            />
          </div>
        </div>

        <div class="form-group">
          <label class="label" for="description">{m.common_description()}</label
          >
          <input
            class="input"
            type="text"
            id="description"
            bind:value={description}
            required
            placeholder={m.journal_descriptionPlaceholder()}
          />
        </div>

        <h3>{m.journal_lines()}</h3>
        <table class="lines-table edit-mode">
          <thead>
            <tr>
              <th>{m.journal_account()}</th>
              <th>{m.common_description()}</th>
              <th class="amount">{m.journal_debit()}</th>
              <th class="amount">{m.journal_credit()}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {#each lines as line, i}
              <tr>
                <td>
                  <select class="input" bind:value={line.accountId} required>
                    <option value="">{m.journal_selectAccount()}</option>
                    {#each accounts as account}
                      <option value={account.id}>
                        {account.code} - {account.name}
                      </option>
                    {/each}
                  </select>
                </td>
                <td>
                  <input
                    class="input"
                    type="text"
                    bind:value={line.description}
                  />
                </td>
                <td class="amount">
                  <input
                    class="input"
                    type="number"
                    step="0.01"
                    min="0"
                    bind:value={line.debit}
                    placeholder="0.00"
                  />
                </td>
                <td class="amount">
                  <input
                    class="input"
                    type="number"
                    step="0.01"
                    min="0"
                    bind:value={line.credit}
                    placeholder="0.00"
                  />
                </td>
                <td>
                  {#if lines.length > 2}
                    <button
                      type="button"
                      class="btn btn-sm btn-danger"
                      onclick={() => removeLine(i)}
                    >
                      X
                    </button>
                  {/if}
                </td>
              </tr>
            {/each}
          </tbody>
          <tfoot>
            <tr class="totals">
              <td colspan="2">
                <button
                  type="button"
                  class="btn btn-sm btn-secondary"
                  onclick={addLine}
                >
                  {m.journal_addLine()}
                </button>
              </td>
              <td class="amount">{getTotalDebits().toFixed(2)}</td>
              <td class="amount">{getTotalCredits().toFixed(2)}</td>
              <td></td>
            </tr>
            <tr class="balance-check">
              <td
                colspan="5"
                class:balanced={isBalanced()}
                class:unbalanced={!isBalanced()}
              >
                {#if isBalanced()}
                  {m.journal_balanced()}
                {:else}
                  {m.journal_difference()}
                  {getTotalDebits().minus(getTotalCredits()).toFixed(2)}
                {/if}
              </td>
            </tr>
          </tfoot>
        </table>

        <div class="modal-actions">
          <button
            type="button"
            class="btn btn-secondary"
            onclick={() => (showCreate = false)}
          >
            {m.common_cancel()}
          </button>
          <button
            type="submit"
            class="btn btn-primary"
            disabled={!isBalanced()}
          >
            {m.journal_createEntry()}
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}

<style>
  .header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 2rem;
  }

  h1 {
    font-size: 1.75rem;
  }

  .empty-state {
    text-align: center;
    padding: 3rem;
  }

  .empty-state h2 {
    margin-bottom: 0.5rem;
  }

  .empty-state p {
    color: var(--color-text-muted);
    margin-bottom: 1.5rem;
  }

  .empty-actions {
    display: flex;
    justify-content: center;
    gap: 0.75rem;
    flex-wrap: wrap;
  }

  .entries-list {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .entry-card {
    padding: 1.5rem;
  }

  .entry-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.75rem;
  }

  .entry-info {
    display: flex;
    align-items: center;
    gap: 1rem;
  }

  .entry-number {
    font-weight: 600;
    font-family: monospace;
  }

  .entry-date {
    color: var(--color-text-muted);
  }

  .entry-description {
    font-size: 1rem;
    margin-bottom: 0.5rem;
  }

  .entry-reference {
    font-size: 0.875rem;
    color: var(--color-text-muted);
    margin-bottom: 1rem;
  }

  .lines-table {
    width: 100%;
    border-collapse: collapse;
    margin-top: 1rem;
  }

  .lines-table th,
  .lines-table td {
    padding: 0.5rem;
    text-align: left;
    border-bottom: 1px solid var(--color-border);
  }

  .lines-table th {
    font-weight: 600;
    font-size: 0.75rem;
    text-transform: uppercase;
    color: var(--color-text-muted);
  }

  .lines-table .amount {
    text-align: right;
    font-family: monospace;
  }

  .lines-table.edit-mode td {
    padding: 0.25rem;
  }

  .lines-table.edit-mode .input {
    width: 100%;
  }

  .lines-table tfoot .totals {
    font-weight: 600;
    border-top: 2px solid var(--color-border);
  }

  .balance-check td {
    text-align: center;
    font-weight: 600;
    padding: 0.5rem;
  }

  .balanced {
    color: var(--color-success, #10b981);
  }

  .unbalanced {
    color: var(--color-error, #ef4444);
  }

  .void-reason {
    margin-top: 1rem;
    padding: 0.5rem;
    background: var(--color-bg-warning, #fef3c7);
    border-radius: 0.25rem;
    font-size: 0.875rem;
  }

  .status-draft {
    background: var(--color-bg);
  }

  .status-posted {
    background: var(--color-success-bg, #d1fae5);
    color: var(--color-success, #10b981);
  }

  .status-voided {
    background: var(--color-error-bg, #fee2e2);
    color: var(--color-error, #ef4444);
  }

  .form-row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1rem;
  }

  h3 {
    margin-top: 1.5rem;
    margin-bottom: 0.5rem;
  }

  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
  }

  .modal {
    width: 100%;
    max-width: 800px;
    max-height: 90vh;
    overflow-y: auto;
    margin: 1rem;
  }

  .modal-lg {
    max-width: 900px;
  }

  .modal h2 {
    margin-bottom: 1.5rem;
  }

  .import-description {
    color: var(--color-text-muted);
    margin-bottom: 1rem;
  }

  .form-hint {
    margin-top: 0.5rem;
    font-size: 0.875rem;
    color: var(--color-text-muted);
  }

  .selected-file {
    margin-top: 0.5rem;
    font-size: 0.875rem;
  }

  .selected-file span {
    font-weight: 600;
  }

  .import-toolbar {
    display: flex;
    justify-content: flex-start;
    margin-top: 1rem;
  }

  .import-summary {
    margin-top: 1.5rem;
  }

  .summary-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
    gap: 0.75rem;
    margin-bottom: 1rem;
  }

  .summary-card {
    border: 1px solid var(--color-border);
    border-radius: 0.5rem;
    padding: 0.75rem;
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }

  .summary-label {
    font-size: 0.875rem;
    color: var(--color-text-muted);
  }

  .modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
    margin-top: 1.5rem;
  }

  /* Mobile responsive */
  @media (max-width: 768px) {
    h1 {
      font-size: 1.25rem;
    }

    .entry-card {
      padding: 1rem;
    }

    .entry-header {
      flex-direction: column;
      align-items: flex-start;
      gap: 0.75rem;
    }

    .entry-info {
      flex-wrap: wrap;
      gap: 0.5rem;
    }

    .entry-actions {
      width: 100%;
      display: flex;
      gap: 0.5rem;
    }

    .entry-actions .btn {
      flex: 1;
      min-height: 44px;
      justify-content: center;
    }

    .empty-state {
      padding: 2rem 1rem;
    }

    .empty-actions {
      flex-direction: column;
    }

    .empty-actions .btn {
      width: 100%;
    }

    .form-row {
      grid-template-columns: 1fr;
      gap: 0;
    }

    .lines-table.edit-mode {
      display: block;
    }

    .lines-table.edit-mode thead {
      display: none;
    }

    .lines-table.edit-mode tbody tr {
      display: block;
      padding: 1rem;
      background: var(--color-bg);
      border-radius: 0.5rem;
      margin-bottom: 1rem;
      border-bottom: none;
    }

    .lines-table.edit-mode td {
      display: block;
      padding: 0.25rem 0;
      border-bottom: none;
    }

    .lines-table.edit-mode td::before {
      content: attr(data-label);
      display: block;
      font-size: 0.75rem;
      font-weight: 600;
      color: var(--color-text-muted);
      margin-bottom: 0.25rem;
    }

    .lines-table.edit-mode .input {
      min-height: 44px;
    }

    .lines-table.edit-mode tfoot {
      display: block;
    }

    .lines-table.edit-mode tfoot tr {
      display: flex;
      flex-wrap: wrap;
      gap: 0.5rem;
      padding: 0.75rem 0;
    }

    .lines-table.edit-mode tfoot td {
      display: block;
    }

    .modal-backdrop {
      padding: 0;
      align-items: flex-end;
    }

    .modal {
      max-width: 100%;
      max-height: 95vh;
      border-radius: 1rem 1rem 0 0;
      margin: 0;
    }

    .modal h2 {
      font-size: 1.25rem;
    }

    .modal-actions {
      flex-direction: column-reverse;
    }

    .modal-actions button {
      width: 100%;
      min-height: 44px;
    }
  }
</style>
