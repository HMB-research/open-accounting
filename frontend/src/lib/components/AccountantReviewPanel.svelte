<script lang="ts">
	import Decimal from 'decimal.js';
	import { api, type BankAccount, type BankTransaction, type JournalEntry, type OverdueInvoicesSummary, type PeriodCloseEvent, type Tenant } from '$lib/api';
	import * as m from '$lib/paraglide/messages.js';

	type BankExceptionGroup = {
		account: BankAccount;
		transactions: BankTransaction[];
	};

	let { tenant }: { tenant: Tenant } = $props();

	let isLoading = $state(true);
	let error = $state('');
	let overdueSummary = $state<OverdueInvoicesSummary | null>(null);
	let bankExceptions = $state<BankExceptionGroup[]>([]);
	let periodCloseEvents = $state<PeriodCloseEvent[]>([]);
	let journalEntries = $state<JournalEntry[]>([]);
	let loadedTenantKey = '';

	$effect(() => {
		const tenantKey = buildTenantKey(tenant);
		if (!tenant.id || tenantKey === loadedTenantKey) {
			return;
		}

		loadedTenantKey = tenantKey;
		void loadReviewWorkspace(tenant.id);
	});

	async function loadReviewWorkspace(tenantId: string) {
		isLoading = true;
		error = '';

		const [overdueResult, accountsResult, closeResult, journalResult] = await Promise.allSettled([
			api.getOverdueInvoices(tenantId),
			api.listBankAccounts(tenantId, true),
			api.listPeriodCloseEvents(tenantId, 6),
			api.listJournalEntries(tenantId, 6)
		]);

		overdueSummary = overdueResult.status === 'fulfilled' ? overdueResult.value : null;
		periodCloseEvents = closeResult.status === 'fulfilled' ? closeResult.value : [];
		journalEntries = journalResult.status === 'fulfilled' ? journalResult.value : [];

		if (accountsResult.status === 'fulfilled') {
			bankExceptions = await loadUnmatchedTransactions(tenantId, accountsResult.value);
		} else {
			bankExceptions = [];
		}

		const failureCount = [overdueResult, accountsResult, closeResult, journalResult].filter((result) => result.status === 'rejected').length;
		if (failureCount === 4) {
			error = m.errors_loadFailed();
		}

		isLoading = false;
	}

	async function loadUnmatchedTransactions(tenantId: string, accounts: BankAccount[]): Promise<BankExceptionGroup[]> {
		const groups = await Promise.all(
			accounts.map(async (account) => {
				try {
					const transactions = await api.listBankTransactions(tenantId, account.id, { status: 'UNMATCHED' });
					return { account, transactions };
				} catch {
					return { account, transactions: [] };
				}
			})
		);

		return groups
			.filter((group) => group.transactions.length > 0)
			.sort((left, right) => right.transactions.length - left.transactions.length);
	}

	function buildTenantKey(value: Tenant): string {
		return `${value.id}:${value.updated_at}:${value.settings?.period_lock_date ?? ''}`;
	}

	function toDecimal(value: Decimal | number | string | null | undefined): Decimal {
		if (Decimal.isDecimal(value)) {
			return value;
		}
		if (value == null || value === '') {
			return new Decimal(0);
		}
		return new Decimal(value);
	}

	function formatCurrency(value: Decimal | number | string): string {
		return new Intl.NumberFormat('et-EE', {
			style: 'currency',
			currency: 'EUR',
			maximumFractionDigits: 0
		}).format(toDecimal(value).toNumber());
	}

	function formatDate(value: string | undefined | null): string {
		if (!value) {
			return m.common_notSet();
		}

		return new Intl.DateTimeFormat('et-EE', {
			year: 'numeric',
			month: 'short',
			day: 'numeric'
		}).format(new Date(value));
	}

	function parseDateValue(value: string | null | undefined): Date | null {
		if (!value) {
			return null;
		}

		const [year, month, day] = value.split('-').map((part) => Number(part));
		if (!year || !month || !day) {
			return null;
		}

		return new Date(Date.UTC(year, month - 1, day));
	}

	function formatIsoDate(value: Date): string {
		const year = value.getUTCFullYear();
		const month = String(value.getUTCMonth() + 1).padStart(2, '0');
		const day = String(value.getUTCDate()).padStart(2, '0');
		return `${year}-${month}-${day}`;
	}

	function monthEndOffset(value: Date, monthOffset: number): Date {
		return new Date(Date.UTC(value.getUTCFullYear(), value.getUTCMonth() + monthOffset + 1, 0));
	}

	function getSuggestedCloseDate(periodLockDate: string | null | undefined): string {
		const currentLock = parseDateValue(periodLockDate);
		if (currentLock) {
			return formatIsoDate(monthEndOffset(currentLock, 1));
		}

		const today = new Date();
		return formatIsoDate(monthEndOffset(today, -1));
	}

	function getEntryTotal(entry: JournalEntry): Decimal {
		return entry.lines.reduce((sum, line) => sum.add(toDecimal(line.base_debit)), new Decimal(0));
	}

	function getCloseActionLabel(event: PeriodCloseEvent): string {
		return event.action === 'reopen' ? m.settings_periodHistoryReopened() : m.settings_periodHistoryClosed();
	}

	function getCloseKindLabel(event: PeriodCloseEvent): string {
		return event.close_kind === 'year_end' ? m.settings_periodYearEnd() : m.settings_periodMonthEnd();
	}

	const unmatchedTransactions = $derived(
		bankExceptions
			.flatMap((group) => group.transactions.map((transaction) => ({ account: group.account, transaction })))
			.sort((left, right) => new Date(right.transaction.transaction_date).getTime() - new Date(left.transaction.transaction_date).getTime())
	);
	const unmatchedAmount = $derived(
		unmatchedTransactions.reduce((sum, item) => sum.add(toDecimal(item.transaction.amount).abs()), new Decimal(0))
	);
	const topOverdueInvoices = $derived(overdueSummary?.invoices.slice(0, 4) ?? []);
	const topUnmatchedTransactions = $derived(unmatchedTransactions.slice(0, 4));
	const topJournalEntries = $derived(journalEntries.slice(0, 4));
	const journalDraftCount = $derived(journalEntries.filter((entry) => entry.status === 'DRAFT').length);
	const journalPostedCount = $derived(journalEntries.filter((entry) => entry.status === 'POSTED').length);
	const currentLockDate = $derived(tenant.settings?.period_lock_date ?? null);
	const suggestedCloseDate = $derived(getSuggestedCloseDate(currentLockDate));
	const latestCloseEvent = $derived(periodCloseEvents[0] ?? null);
</script>

<section class="review-board card">
	<div class="review-board-header">
		<div>
			<div class="review-kicker">{m.dashboard_reviewQueue()}</div>
			<h3>{m.dashboard_reviewQueueTitle()}</h3>
			<p>{m.dashboard_reviewQueueDesc()}</p>
		</div>
		<button class="btn btn-secondary review-refresh" type="button" onclick={() => loadReviewWorkspace(tenant.id)} disabled={isLoading}>
			{m.common_refresh()}
		</button>
	</div>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if isLoading}
		<div class="review-loading">{m.common_loading()}</div>
	{:else}
		<div class="review-grid">
			<article class="review-card review-card-emphasis">
				<div class="review-card-topline">
					<span class="review-card-kicker">{m.dashboard_reviewOverdueTitle()}</span>
					<a href="/invoices/reminders?tenant={tenant.id}" class="review-action">{m.dashboard_reviewOpenReminders()}</a>
				</div>
				<div class="review-figure">
					<strong>{overdueSummary ? formatCurrency(overdueSummary.total_overdue) : formatCurrency(0)}</strong>
					<span>{m.dashboard_reviewOutstandingBalance()}</span>
				</div>
				<div class="review-metrics">
					<div>
						<strong>{overdueSummary?.invoice_count ?? 0}</strong>
						<span>{m.invoices_overdue()}</span>
					</div>
					<div>
						<strong>{overdueSummary?.contact_count ?? 0}</strong>
						<span>{m.dashboard_reviewContacts()}</span>
					</div>
					<div>
						<strong>{overdueSummary?.average_days_overdue ?? 0}</strong>
						<span>{m.dashboard_reviewAverageDays()}</span>
					</div>
				</div>

				{#if topOverdueInvoices.length > 0}
					<ul class="review-list">
						{#each topOverdueInvoices as invoice}
							<li>
								<div>
									<strong>{invoice.invoice_number}</strong>
									<span>{invoice.contact_name}</span>
								</div>
								<div class="review-list-meta">
									<strong>{formatCurrency(invoice.outstanding_amount)}</strong>
									<span>{invoice.days_overdue} {m.dashboard_reviewDaysShort()}</span>
								</div>
							</li>
						{/each}
					</ul>
				{:else}
					<p class="review-empty">{m.dashboard_reviewNoOverdue()}</p>
				{/if}
			</article>

			<article class="review-card">
				<div class="review-card-topline">
					<span class="review-card-kicker">{m.dashboard_reviewBankingTitle()}</span>
					<a href="/banking?tenant={tenant.id}" class="review-action">{m.dashboard_reviewOpenBanking()}</a>
				</div>
				<div class="review-figure">
					<strong>{unmatchedTransactions.length}</strong>
					<span>{m.dashboard_reviewUnmatchedTransactions()}</span>
				</div>
				<div class="review-metrics">
					<div>
						<strong>{formatCurrency(unmatchedAmount)}</strong>
						<span>{m.common_amount()}</span>
					</div>
					<div>
						<strong>{bankExceptions.length}</strong>
						<span>{m.dashboard_reviewAccounts()}</span>
					</div>
					<div>
						<strong>{topUnmatchedTransactions[0] ? formatDate(topUnmatchedTransactions[0].transaction.transaction_date) : m.common_notSet()}</strong>
						<span>{m.common_date()}</span>
					</div>
				</div>

				{#if topUnmatchedTransactions.length > 0}
					<ul class="review-list">
						{#each topUnmatchedTransactions as item}
							<li>
								<div>
									<strong>{item.account.name}</strong>
									<span>{item.transaction.description || item.transaction.counterparty_name || m.common_noData()}</span>
								</div>
								<div class="review-list-meta">
									<strong>{formatCurrency(toDecimal(item.transaction.amount).abs())}</strong>
									<span>{formatDate(item.transaction.transaction_date)}</span>
								</div>
							</li>
						{/each}
					</ul>
				{:else}
					<p class="review-empty">{m.dashboard_reviewNoBankingExceptions()}</p>
				{/if}
			</article>

			<article class="review-card">
				<div class="review-card-topline">
					<span class="review-card-kicker">{m.dashboard_reviewCloseTitle()}</span>
					<a href="/settings/company?tenant={tenant.id}" class="review-action">{m.dashboard_reviewOpenCloseControls()}</a>
				</div>
				<div class="review-figure">
					<strong>
						{#if currentLockDate}
							{m.settings_periodClosedThrough({ date: formatDate(currentLockDate) })}
						{:else}
							{m.dashboard_reviewNoLockedPeriods()}
						{/if}
					</strong>
					<span>{m.dashboard_reviewSuggestedCloseDate({ date: formatDate(suggestedCloseDate) })}</span>
				</div>
				<div class="review-metrics">
					<div>
						<strong>{latestCloseEvent ? getCloseActionLabel(latestCloseEvent) : m.common_notSet()}</strong>
						<span>{m.dashboard_reviewLastAction()}</span>
					</div>
					<div>
						<strong>{latestCloseEvent ? getCloseKindLabel(latestCloseEvent) : m.common_notSet()}</strong>
						<span>{m.dashboard_reviewLastCloseType()}</span>
					</div>
					<div>
						<strong>{latestCloseEvent ? formatDate(latestCloseEvent.period_end_date) : m.common_notSet()}</strong>
						<span>{m.dashboard_reviewPeriodEnd()}</span>
					</div>
				</div>

				{#if periodCloseEvents.length > 0}
					<ul class="review-list">
						{#each periodCloseEvents.slice(0, 4) as event}
							<li>
								<div>
									<strong>{getCloseActionLabel(event)}</strong>
									<span>{getCloseKindLabel(event)}</span>
								</div>
								<div class="review-list-meta">
									<strong>{formatDate(event.period_end_date)}</strong>
									<span>{formatDate(event.created_at)}</span>
								</div>
							</li>
						{/each}
					</ul>
				{:else}
					<p class="review-empty">{m.dashboard_reviewNoCloseHistory()}</p>
				{/if}
			</article>

			<article class="review-card">
				<div class="review-card-topline">
					<span class="review-card-kicker">{m.dashboard_reviewJournalTitle()}</span>
					<a href="/journal?tenant={tenant.id}" class="review-action">{m.dashboard_reviewOpenJournal()}</a>
				</div>
				<div class="review-figure">
					<strong>{journalEntries.length}</strong>
					<span>{m.dashboard_reviewRecentEntries()}</span>
				</div>
				<div class="review-metrics">
					<div>
						<strong>{journalDraftCount}</strong>
						<span>{m.dashboard_draft()}</span>
					</div>
					<div>
						<strong>{journalPostedCount}</strong>
						<span>{m.dashboard_reviewPosted()}</span>
					</div>
					<div>
						<strong>{topJournalEntries[0] ? formatDate(topJournalEntries[0].entry_date) : m.common_notSet()}</strong>
						<span>{m.common_date()}</span>
					</div>
				</div>

				{#if topJournalEntries.length > 0}
					<ul class="review-list">
						{#each topJournalEntries as entry}
							<li>
								<div>
									<strong>{entry.entry_number}</strong>
									<span>{entry.description}</span>
								</div>
								<div class="review-list-meta">
									<strong>{formatCurrency(getEntryTotal(entry))}</strong>
									<span>{entry.status}</span>
								</div>
							</li>
						{/each}
					</ul>
				{:else}
					<p class="review-empty">{m.dashboard_reviewNoJournalEntries()}</p>
				{/if}
			</article>
		</div>
	{/if}
</section>

<style>
	.review-board {
		margin-bottom: 1.75rem;
		padding: 1.5rem;
		background:
			radial-gradient(circle at top left, rgba(251, 191, 36, 0.14), transparent 28%),
			linear-gradient(145deg, rgba(255, 252, 247, 0.96), rgba(255, 255, 255, 0.86));
		border: 1px solid rgba(148, 163, 184, 0.18);
	}

	.review-board-header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: 1rem;
		margin-bottom: 1.25rem;
	}

	.review-kicker,
	.review-card-kicker {
		font-size: 0.78rem;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--color-text-muted);
	}

	.review-board-header h3 {
		font-family: var(--font-display);
		font-size: clamp(1.8rem, 4vw, 2.6rem);
		line-height: 0.95;
		margin: 0.4rem 0 0.5rem;
	}

	.review-board-header p {
		max-width: 42rem;
		color: var(--color-text-muted);
		margin: 0;
	}

	.review-refresh {
		flex-shrink: 0;
	}

	.review-loading {
		padding: 1rem 0 0.25rem;
		color: var(--color-text-muted);
	}

	.review-grid {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 1rem;
	}

	.review-card {
		padding: 1.25rem;
		border-radius: 1.25rem;
		background: rgba(255, 255, 255, 0.8);
		border: 1px solid rgba(148, 163, 184, 0.16);
		box-shadow: 0 24px 50px rgba(15, 23, 42, 0.06);
		display: flex;
		flex-direction: column;
		gap: 1rem;
		min-height: 100%;
	}

	.review-card-emphasis {
		background:
			linear-gradient(160deg, rgba(15, 23, 42, 0.94), rgba(30, 41, 59, 0.88)),
			radial-gradient(circle at top right, rgba(251, 191, 36, 0.18), transparent 32%);
		color: rgba(248, 250, 252, 0.94);
	}

	.review-card-emphasis .review-card-kicker,
	.review-card-emphasis .review-action,
	.review-card-emphasis .review-figure span,
	.review-card-emphasis .review-metrics span,
	.review-card-emphasis .review-empty,
	.review-card-emphasis .review-list span {
		color: rgba(226, 232, 240, 0.74);
	}

	.review-card-topline {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 1rem;
	}

	.review-action {
		font-size: 0.82rem;
		font-weight: 600;
		color: var(--color-primary);
		text-decoration: none;
	}

	.review-action:hover {
		text-decoration: underline;
	}

	.review-figure {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}

	.review-figure strong {
		font-size: 1.5rem;
		line-height: 1.1;
	}

	.review-figure span,
	.review-metrics span,
	.review-empty,
	.review-list span {
		color: var(--color-text-muted);
	}

	.review-metrics {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		gap: 0.75rem;
	}

	.review-metrics div {
		padding: 0.75rem;
		border-radius: 0.9rem;
		background: rgba(248, 250, 252, 0.72);
		display: flex;
		flex-direction: column;
		gap: 0.2rem;
	}

	.review-card-emphasis .review-metrics div {
		background: rgba(255, 255, 255, 0.08);
	}

	.review-metrics strong {
		font-size: 1rem;
	}

	.review-list {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 0.7rem;
	}

	.review-list li {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: 1rem;
		padding-top: 0.7rem;
		border-top: 1px solid rgba(148, 163, 184, 0.16);
	}

	.review-list li:first-child {
		padding-top: 0;
		border-top: none;
	}

	.review-list li > div,
	.review-list-meta {
		display: flex;
		flex-direction: column;
		gap: 0.15rem;
	}

	.review-list-meta {
		text-align: right;
	}

	.review-empty {
		margin: 0;
	}

	@media (max-width: 900px) {
		.review-grid {
			grid-template-columns: 1fr;
		}
	}

	@media (max-width: 640px) {
		.review-board-header {
			flex-direction: column;
		}

		.review-refresh {
			width: 100%;
		}

		.review-metrics {
			grid-template-columns: 1fr;
		}

		.review-list li {
			flex-direction: column;
		}

		.review-list-meta {
			text-align: left;
		}
	}
</style>
