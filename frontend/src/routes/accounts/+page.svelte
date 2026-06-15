<script lang="ts">
  import { browser } from "$app/environment";
  import { page } from "$app/stores";
  import { api, type Account, type ImportAccountsResult } from "$lib/api";
  import Decimal from "decimal.js";
  import { SvelteMap } from "svelte/reactivity";
  import * as m from "$lib/paraglide/messages.js";

  let accounts = $state<Account[]>([]);
  let accountBalances = new SvelteMap<
    string,
    { debit: Decimal; credit: Decimal; balance: Decimal }
  >();
  let isLoading = $state(true);
  let error = $state("");
  let showAccountModal = $state(false);
  let showImportAccounts = $state(false);
  let selectedAccountForEdit = $state<Account | null>(null);
  let deletingAccountId = $state("");
  let importError = $state("");
  let importFileName = $state("");
  let importCSVContent = $state("");
  let isImporting = $state(false);
  let importResult = $state<ImportAccountsResult | null>(null);

  let newCode = $state("");
  let newName = $state("");
  let newType = $state<Account["account_type"]>("ASSET");
  let newDescription = $state("");
  let isEditMode = $derived(Boolean(selectedAccountForEdit));

  $effect(() => {
    const tenantId = $page.url.searchParams.get("tenant");
    if (tenantId) {
      loadAccounts(tenantId);
    }
  });

  async function loadAccounts(tenantId: string) {
    isLoading = true;
    error = "";

    try {
      const [accountsData, trialBalance] = await Promise.all([
        api.listAccounts(tenantId),
        api.getTrialBalance(tenantId).catch(() => null),
      ]);

      accounts = accountsData;
      accountBalances.clear();

      if (trialBalance) {
        for (const item of trialBalance.accounts) {
          const debit =
            item.debit_balance instanceof Decimal
              ? item.debit_balance
              : new Decimal(item.debit_balance || 0);
          const credit =
            item.credit_balance instanceof Decimal
              ? item.credit_balance
              : new Decimal(item.credit_balance || 0);
          const balance =
            item.net_balance instanceof Decimal
              ? item.net_balance
              : new Decimal(item.net_balance || 0);
          accountBalances.set(item.account_id, { debit, credit, balance });
        }
      }
    } catch (err) {
      error = err instanceof Error ? err.message : "Failed to load accounts";
    } finally {
      isLoading = false;
    }
  }

  function formatBalance(
    accountId: string,
    accountType: Account["account_type"],
  ): string {
    const balanceData = accountBalances.get(accountId);
    if (!balanceData) return "-";

    let balance = balanceData.balance;
    if (
      accountType === "LIABILITY" ||
      accountType === "EQUITY" ||
      accountType === "REVENUE"
    ) {
      balance = balance.neg();
    }

    if (balance.isZero()) return "-";

    return new Intl.NumberFormat("et-EE", {
      style: "currency",
      currency: "EUR",
      minimumFractionDigits: 2,
    }).format(balance.toNumber());
  }

  function resetAccountForm() {
    newCode = "";
    newName = "";
    newType = "ASSET";
    newDescription = "";
  }

  function populateAccountForm(account: Account) {
    newCode = account.code;
    newName = account.name;
    newType = account.account_type;
    newDescription = account.description || "";
  }

  function openCreateAccount() {
    selectedAccountForEdit = null;
    resetAccountForm();
    showAccountModal = true;
    error = "";
  }

  function openEditAccount(account: Account) {
    selectedAccountForEdit = account;
    populateAccountForm(account);
    showAccountModal = true;
    error = "";
  }

  function closeAccountModal() {
    showAccountModal = false;
    selectedAccountForEdit = null;
    resetAccountForm();
  }

  async function saveAccount(event: Event) {
    event.preventDefault();
    const tenantId = $page.url.searchParams.get("tenant");
    if (!tenantId) return;

    try {
      const payload = {
        code: newCode,
        name: newName,
        account_type: newType,
        description: newDescription || undefined,
      };
      const account = selectedAccountForEdit
        ? await api.updateAccount(tenantId, selectedAccountForEdit.id, payload)
        : await api.createAccount(tenantId, payload);
      const exists = accounts.some((existing) => existing.id === account.id);
      accounts = (
        exists
          ? accounts.map((existing) =>
              existing.id === account.id ? account : existing,
            )
          : [...accounts, account]
      ).sort((a, b) => a.code.localeCompare(b.code));
      closeAccountModal();
    } catch (err) {
      error = err instanceof Error ? err.message : "Failed to save account";
    }
  }

  async function deleteAccount(account: Account) {
    const tenantId = $page.url.searchParams.get("tenant");
    if (!tenantId || deletingAccountId || account.is_system) return;
    if (
      !window.confirm(
        m.accounts_confirmDelete({ code: account.code, name: account.name }),
      )
    ) {
      return;
    }

    deletingAccountId = account.id;
    error = "";
    try {
      const deletedAccount = await api.deleteAccount(tenantId, account.id);
      accounts = accounts.map((existing) =>
        existing.id === deletedAccount.id ? deletedAccount : existing,
      );
    } catch (err) {
      error =
        err instanceof Error ? err.message : "Failed to deactivate account";
    } finally {
      deletingAccountId = "";
    }
  }

  function openImportModal() {
    showImportAccounts = true;
    importError = "";
    importFileName = "";
    importCSVContent = "";
    importResult = null;
  }

  function closeImportModal() {
    showImportAccounts = false;
    importError = "";
    importFileName = "";
    importCSVContent = "";
    importResult = null;
  }

  async function handleImportFileChange(event: Event) {
    const input = event.currentTarget as HTMLInputElement | null;
    const file = input?.files?.[0];

    importResult = null;

    if (!file) {
      importFileName = "";
      importCSVContent = "";
      return;
    }

    importFileName = file.name;
    importCSVContent = await file.text();
    importError = "";
  }

  async function submitImport(event: Event) {
    event.preventDefault();

    const tenantId = $page.url.searchParams.get("tenant");
    if (!tenantId) return;

    if (!importCSVContent.trim()) {
      importError = m.accounts_importFileRequired();
      return;
    }

    isImporting = true;
    importError = "";

    try {
      importResult = await api.importAccounts(tenantId, {
        file_name: importFileName || undefined,
        csv_content: importCSVContent,
      });

      if (importResult.accounts_created > 0) {
        await loadAccounts(tenantId);
      }
    } catch (err) {
      importError =
        err instanceof Error ? err.message : "Failed to import accounts";
    } finally {
      isImporting = false;
    }
  }

  function downloadImportTemplate() {
    if (!browser) return;

    const template = [
      "code,name,account_type,description,parent_code",
      "1100,Cash in Office,ASSET,Petty cash account,",
      "1110,Cash Drawer,ASSET,Drawer cash,1100",
      "4000,Sales Revenue,REVENUE,Main revenue account,",
    ].join("\n");

    const blob = new Blob([template], { type: "text/csv;charset=utf-8" });
    const url = window.URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = "accounts-import-template.csv";
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    window.URL.revokeObjectURL(url);
  }

  function groupByType(currentAccounts: Account[]) {
    const groups: Record<Account["account_type"], Account[]> = {
      ASSET: [],
      LIABILITY: [],
      EQUITY: [],
      REVENUE: [],
      EXPENSE: [],
    };

    for (const account of currentAccounts) {
      groups[account.account_type].push(account);
    }

    return groups;
  }

  function getTypeLabel(type: Account["account_type"]): string {
    switch (type) {
      case "ASSET":
        return m.accounts_assets();
      case "LIABILITY":
        return m.accounts_liabilities();
      case "EQUITY":
        return m.accounts_equities();
      case "REVENUE":
        return m.accounts_revenues();
      case "EXPENSE":
        return m.accounts_expenses();
    }
  }

  const typeOrder: Account["account_type"][] = [
    "ASSET",
    "LIABILITY",
    "EQUITY",
    "REVENUE",
    "EXPENSE",
  ];
</script>

<svelte:head>
  <title>{m.accounts_title()} - Open Accounting</title>
</svelte:head>

<div class="container">
  <div class="page-header">
    <h1>{m.accounts_title()}</h1>
    <div class="page-actions">
      <button class="btn btn-secondary" onclick={openImportModal}>
        {m.accounts_importAccounts()}
      </button>
      <button class="btn btn-primary" onclick={openCreateAccount}>
        + {m.accounts_newAccount()}
      </button>
    </div>
  </div>

  {#if error}
    <div class="alert alert-error">{error}</div>
  {/if}

  {#if isLoading}
    <p>{m.common_loading()}</p>
  {:else if accounts.length === 0}
    <div class="empty-state card">
      <p>{m.accounts_noAccounts()}</p>
    </div>
  {:else}
    {@const groups = groupByType(accounts)}
    {#each typeOrder as type (type)}
      {@const typeAccounts = groups[type]}
      {#if typeAccounts.length > 0}
        <section class="account-section card">
          <h2>{getTypeLabel(type)}</h2>
          <div class="table-container">
            <table class="table table-mobile-cards">
              <thead>
                <tr>
                  <th>{m.accounts_code()}</th>
                  <th>{m.common_name()}</th>
                  <th class="hide-mobile">{m.common_description()}</th>
                  <th class="balance-col">{m.common_balance()}</th>
                  <th>{m.common_status()}</th>
                  <th>{m.common_actions()}</th>
                </tr>
              </thead>
              <tbody>
                {#each typeAccounts as account (account.id)}
                  <tr class:inactive={!account.is_active}>
                    <td class="code" data-label="Code">{account.code}</td>
                    <td data-label="Name">{account.name}</td>
                    <td
                      class="description hide-mobile"
                      data-label="Description"
                    >
                      {account.description || "-"}
                    </td>
                    <td class="balance" data-label="Balance">
                      {formatBalance(account.id, account.account_type)}
                    </td>
                    <td data-label="Status">
                      {#if account.is_system}
                        <span class="badge badge-system"
                          >{m.accounts_system()}</span
                        >
                      {:else if !account.is_active}
                        <span class="badge badge-inactive"
                          >{m.accounts_inactive()}</span
                        >
                      {/if}
                    </td>
                    <td
                      class="actions actions-cell"
                      data-label={m.common_actions()}
                    >
                      {#if !account.is_system && account.is_active}
                        <button
                          type="button"
                          class="btn btn-small btn-secondary"
                          onclick={() => openEditAccount(account)}
                        >
                          {m.common_edit()}
                        </button>
                        <button
                          type="button"
                          class="btn btn-small btn-danger"
                          disabled={deletingAccountId === account.id}
                          onclick={() => deleteAccount(account)}
                        >
                          {deletingAccountId === account.id
                            ? m.accounts_deleting()
                            : m.common_delete()}
                        </button>
                      {:else}
                        <span class="muted">-</span>
                      {/if}
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        </section>
      {/if}
    {/each}
  {/if}
</div>

{#if showImportAccounts}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <div class="modal-backdrop" onclick={closeImportModal} role="presentation">
    <div
      class="modal card import-modal"
      onclick={(e) => e.stopPropagation()}
      role="dialog"
      aria-modal="true"
      aria-labelledby="import-accounts-title"
      tabindex="-1"
    >
      <h2 id="import-accounts-title">{m.accounts_importAccounts()}</h2>
      <p class="import-description">{m.accounts_importDescription()}</p>

      {#if importError}
        <div class="alert alert-error">{importError}</div>
      {/if}

      <form onsubmit={submitImport}>
        <div class="form-group">
          <label class="label" for="account-import-file"
            >{m.accounts_importChooseFile()}</label
          >
          <input
            class="input"
            id="account-import-file"
            type="file"
            accept=".csv,text/csv"
            onchange={handleImportFileChange}
          />
          <p class="form-hint">{m.accounts_importTemplateHint()}</p>
          {#if importFileName}
            <p class="selected-file">
              {m.accounts_importSelectedFile()}: <span>{importFileName}</span>
            </p>
          {/if}
        </div>

        <div class="import-toolbar">
          <button
            type="button"
            class="btn btn-secondary"
            onclick={downloadImportTemplate}
          >
            {m.accounts_importTemplate()}
          </button>
        </div>

        <div class="modal-actions">
          <button
            type="button"
            class="btn btn-secondary"
            onclick={closeImportModal}
          >
            {m.common_cancel()}
          </button>
          <button type="submit" class="btn btn-primary" disabled={isImporting}>
            {isImporting ? m.accounts_importing() : m.accounts_importAccounts()}
          </button>
        </div>
      </form>

      {#if importResult}
        <div class="import-summary">
          <h3>{m.accounts_importSummary()}</h3>
          <div class="summary-grid">
            <div class="summary-card">
              <span class="summary-label"
                >{m.accounts_importRowsProcessed()}</span
              >
              <strong>{importResult.rows_processed}</strong>
            </div>
            <div class="summary-card">
              <span class="summary-label"
                >{m.accounts_importAccountsCreated()}</span
              >
              <strong>{importResult.accounts_created}</strong>
            </div>
            <div class="summary-card">
              <span class="summary-label">{m.accounts_importRowsSkipped()}</span
              >
              <strong>{importResult.rows_skipped}</strong>
            </div>
          </div>

          {#if importResult.errors?.length}
            <h3>{m.accounts_importErrors()}</h3>
            <div class="table-container import-errors">
              <table class="table table-mobile-cards">
                <thead>
                  <tr>
                    <th>{m.accounts_importRow()}</th>
                    <th>{m.accounts_code()}</th>
                    <th>{m.common_name()}</th>
                    <th>{m.accounts_importMessage()}</th>
                  </tr>
                </thead>
                <tbody>
                  {#each importResult.errors as rowError (`${rowError.row}:${rowError.message}`)}
                    <tr>
                      <td data-label={m.accounts_importRow()}>{rowError.row}</td
                      >
                      <td data-label={m.accounts_code()}
                        >{rowError.code || "-"}</td
                      >
                      <td data-label={m.common_name()}
                        >{rowError.name || "-"}</td
                      >
                      <td data-label={m.accounts_importMessage()}
                        >{rowError.message}</td
                      >
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {:else}
            <div class="alert alert-success">
              {m.accounts_importAccountsCreated()}: {importResult.accounts_created}
            </div>
          {/if}
        </div>
      {/if}
    </div>
  </div>
{/if}

{#if showAccountModal}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <div class="modal-backdrop" onclick={closeAccountModal} role="presentation">
    <div
      class="modal card"
      onclick={(e) => e.stopPropagation()}
      role="dialog"
      aria-modal="true"
      aria-labelledby="create-account-title"
      tabindex="-1"
    >
      <h2 id="create-account-title">
        {isEditMode ? m.accounts_editAccount() : m.accounts_newAccount()}
      </h2>
      <form onsubmit={saveAccount}>
        <div class="form-group">
          <label class="label" for="code">{m.accounts_accountCode()}</label>
          <input
            class="input"
            type="text"
            id="code"
            bind:value={newCode}
            required
            placeholder="1100"
          />
        </div>

        <div class="form-group">
          <label class="label" for="name">{m.accounts_accountName()}</label>
          <input
            class="input"
            type="text"
            id="name"
            bind:value={newName}
            required
            placeholder={m.accounts_cashAndBank()}
          />
        </div>

        <div class="form-group">
          <label class="label" for="type">{m.accounts_accountType()}</label>
          <select class="input" id="type" bind:value={newType}>
            <option value="ASSET">{m.accounts_asset()}</option>
            <option value="LIABILITY">{m.accounts_liability()}</option>
            <option value="EQUITY">{m.accounts_equity()}</option>
            <option value="REVENUE">{m.accounts_revenue()}</option>
            <option value="EXPENSE">{m.accounts_expense()}</option>
          </select>
        </div>

        <div class="form-group">
          <label class="label" for="description">{m.common_description()}</label
          >
          <input
            class="input"
            type="text"
            id="description"
            bind:value={newDescription}
            placeholder={m.accounts_optionalDescription()}
          />
        </div>

        <div class="modal-actions">
          <button
            type="button"
            class="btn btn-secondary"
            onclick={closeAccountModal}
          >
            {m.common_cancel()}
          </button>
          <button type="submit" class="btn btn-primary">
            {isEditMode ? m.common_save() : m.common_create()}
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}

<style>
  h1 {
    font-size: 1.75rem;
  }

  .page-actions {
    display: flex;
    gap: 0.75rem;
    flex-wrap: wrap;
  }

  .account-section {
    margin-bottom: 1.5rem;
  }

  .account-section h2 {
    font-size: 1.125rem;
    margin-bottom: 1rem;
    color: var(--color-text-muted);
  }

  .code {
    font-family: var(--font-mono);
    font-weight: 500;
  }

  .description {
    color: var(--color-text-muted);
  }

  .balance-col {
    text-align: right;
  }

  .balance {
    font-family: var(--font-mono);
    text-align: right;
    white-space: nowrap;
  }

  .inactive {
    opacity: 0.6;
  }

  .actions {
    display: flex;
    gap: 0.35rem;
    justify-content: flex-end;
    flex-wrap: wrap;
  }

  .btn-small {
    padding: 0.25rem 0.5rem;
    font-size: 0.75rem;
  }

  .btn-danger {
    background: #dc2626;
    color: white;
  }

  .btn-danger:hover:not(:disabled) {
    background: #b91c1c;
  }

  .muted {
    color: var(--color-text-muted);
  }

  .badge-system {
    background: #e0e7ff;
    color: #3730a3;
  }

  .badge-inactive {
    background: #f3f4f6;
    color: #6b7280;
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
    max-width: 400px;
    margin: 1rem;
  }

  .import-modal {
    max-width: 760px;
  }

  .modal h2 {
    margin-bottom: 1.5rem;
  }

  .modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
    margin-top: 1.5rem;
  }

  .import-description,
  .form-hint,
  .selected-file,
  .summary-label {
    color: var(--color-text-muted);
  }

  .form-hint,
  .selected-file {
    margin-top: 0.5rem;
    font-size: 0.925rem;
  }

  .selected-file span {
    color: var(--color-text);
    font-weight: 500;
  }

  .import-toolbar {
    display: flex;
    justify-content: flex-start;
    margin-top: 1rem;
  }

  .import-summary {
    margin-top: 2rem;
    padding-top: 1.5rem;
    border-top: 1px solid var(--color-border);
  }

  .import-summary h3 {
    margin-bottom: 1rem;
  }

  .summary-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 0.75rem;
    margin-bottom: 1.5rem;
  }

  .summary-card {
    padding: 1rem;
    border: 1px solid var(--color-border);
    border-radius: 0.75rem;
    background: var(--color-bg);
  }

  .summary-card strong {
    display: block;
    font-size: 1.5rem;
    margin-top: 0.35rem;
  }

  .import-errors {
    margin-top: 1rem;
  }

  @media (max-width: 768px) {
    h1 {
      font-size: 1.25rem;
    }

    .account-section {
      padding: 1rem;
    }

    .account-section h2 {
      font-size: 1rem;
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
      overflow-y: auto;
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

    .summary-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
