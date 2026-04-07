<script lang="ts">
	import Decimal from 'decimal.js';
	import { type PeriodCloseEvent, type TenantMembership } from '$lib/api';
	import * as m from '$lib/paraglide/messages.js';
	import {
		flattenUnmatchedTransactions,
		getSuggestedCloseDate,
		loadTenantReviewSnapshot,
		needsPeriodClose,
		toDecimal,
		type TenantReviewSnapshot
	} from '$lib/review/workspace';

	type PortfolioItem = {
		membership: TenantMembership;
		snapshot: TenantReviewSnapshot;
		overdueCount: number;
		overdueAmount: Decimal;
		unmatchedCount: number;
		unmatchedAmount: Decimal;
		missingEvidenceCount: number;
		pendingEvidenceCount: number;
		needsClose: boolean;
		suggestedCloseDate: string;
		lastCloseEvent: PeriodCloseEvent | null;
		openTasks: number;
		urgency: number;
		isCurrent: boolean;
	};

	let {
		memberships,
		currentTenantId = null
	}: {
		memberships: TenantMembership[];
		currentTenantId?: string | null;
	} = $props();

	let snapshots = $state<TenantReviewSnapshot[]>([]);
	let isLoading = $state(false);
	let error = $state('');
	let loadedMembershipKey = '';

	function buildMembershipKey(values: TenantMembership[]): string {
		return values
			.filter((membership) => membership.tenant.is_active)
			.map((membership) => `${membership.tenant.id}:${membership.tenant.updated_at}:${membership.tenant.settings?.period_lock_date ?? ''}`)
			.sort()
			.join('|');
	}

	$effect(() => {
		const nextKey = buildMembershipKey(memberships);
		const activeMembershipCount = memberships.filter((membership) => membership.tenant.is_active).length;

		if (activeMembershipCount < 2) {
			snapshots = [];
			error = '';
			isLoading = false;
			loadedMembershipKey = nextKey;
			return;
		}

		if (nextKey === loadedMembershipKey) {
			return;
		}

		loadedMembershipKey = nextKey;
		void loadPortfolio();
	});

	async function loadPortfolio() {
		const activeMemberships = memberships.filter((membership) => membership.tenant.is_active);
		if (activeMemberships.length < 2) {
			snapshots = [];
			return;
		}

		isLoading = true;
		error = '';

		const results = await Promise.allSettled(
			activeMemberships.map((membership) => loadTenantReviewSnapshot(membership.tenant))
		);

		snapshots = results
			.filter((result): result is PromiseFulfilledResult<TenantReviewSnapshot> => result.status === 'fulfilled')
			.map((result) => result.value);

		if (snapshots.length === 0 || snapshots.every((snapshot) => snapshot.errorCount === 4)) {
			error = m.errors_loadFailed();
		}

		isLoading = false;
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

	function getCloseStatusLabel(item: PortfolioItem): string {
		if (item.needsClose) {
			return m.dashboard_reviewPortfolioCloseDue({ date: formatDate(item.suggestedCloseDate) });
		}

		const lockDate = item.snapshot.tenant.settings?.period_lock_date;
		if (!lockDate) {
			return m.dashboard_reviewPortfolioNoLock();
		}

		return m.dashboard_reviewPortfolioLockedThrough({ date: formatDate(lockDate) });
	}

	const portfolioItems = $derived.by(() => {
		const items: PortfolioItem[] = [];

		for (const snapshot of snapshots) {
			const membership = memberships.find((value) => value.tenant.id === snapshot.tenant.id);
			if (!membership) {
				continue;
			}

			const unmatchedTransactions = flattenUnmatchedTransactions(snapshot.bankExceptions);
			const overdueCount = snapshot.overdueSummary?.invoice_count ?? 0;
			const overdueAmount = toDecimal(snapshot.overdueSummary?.total_overdue ?? 0);
			const unmatchedCount = unmatchedTransactions.length;
			const unmatchedAmount = unmatchedTransactions.reduce(
				(sum, item) => sum.add(toDecimal(item.transaction.amount).abs()),
				new Decimal(0)
			);
			const missingEvidenceCount = unmatchedTransactions.filter(
				(item) => item.documentSummary.missing_evidence
			).length;
			const pendingEvidenceCount = unmatchedTransactions.filter(
				(item) => item.documentSummary.has_pending_review
			).length;
			const periodLockDate = snapshot.tenant.settings?.period_lock_date ?? null;
			const closeIsDue = needsPeriodClose(periodLockDate);
			const openTasks =
				overdueCount +
				unmatchedCount +
				missingEvidenceCount +
				pendingEvidenceCount +
				(closeIsDue ? 1 : 0) +
				(snapshot.tenant.onboarding_completed ? 0 : 1);
			const urgency =
				openTasks +
				Math.min(5, Math.floor(overdueAmount.div(1000).toNumber())) +
				Math.min(5, unmatchedCount) +
				Math.min(5, missingEvidenceCount) +
				Math.min(3, pendingEvidenceCount) +
				(closeIsDue ? 3 : 0) +
				(snapshot.tenant.onboarding_completed ? 0 : 2);

			items.push({
				membership,
				snapshot,
				overdueCount,
				overdueAmount,
				unmatchedCount,
				unmatchedAmount,
				missingEvidenceCount,
				pendingEvidenceCount,
				needsClose: closeIsDue,
				suggestedCloseDate: getSuggestedCloseDate(periodLockDate),
				lastCloseEvent: snapshot.periodCloseEvents[0] ?? null,
				openTasks,
				urgency,
				isCurrent: membership.tenant.id === currentTenantId
			});
		}

		return items.sort((left, right) => {
			if (right.urgency !== left.urgency) {
				return right.urgency - left.urgency;
			}
			if (!left.overdueAmount.eq(right.overdueAmount)) {
				return right.overdueAmount.comparedTo(left.overdueAmount);
			}
			return right.unmatchedCount - left.unmatchedCount;
		});
	});

	const attentionTenants = $derived(portfolioItems.filter((item) => item.openTasks > 0));
	const totalOpenTasks = $derived(attentionTenants.reduce((sum, item) => sum + item.openTasks, 0));
	const totalOverdueAmount = $derived(
		attentionTenants.reduce((sum, item) => sum.add(item.overdueAmount), new Decimal(0))
	);
	const totalUnmatchedTransactions = $derived(
		attentionTenants.reduce((sum, item) => sum + item.unmatchedCount, 0)
	);
	const totalMissingEvidence = $derived(
		attentionTenants.reduce((sum, item) => sum + item.missingEvidenceCount, 0)
	);
	const totalPendingEvidence = $derived(
		attentionTenants.reduce((sum, item) => sum + item.pendingEvidenceCount, 0)
	);
	const tenantsNeedingClose = $derived(
		attentionTenants.filter((item) => item.needsClose).length
	);
	const priorityItems = $derived(attentionTenants.slice(0, 6));
</script>

{#if memberships.filter((membership) => membership.tenant.is_active).length > 1}
	<section class="portfolio-board card">
		<div class="portfolio-board-header">
			<div>
				<div class="portfolio-kicker">{m.dashboard_reviewPortfolio()}</div>
				<h3>{m.dashboard_reviewPortfolioTitle()}</h3>
				<p>{m.dashboard_reviewPortfolioDesc()}</p>
			</div>
			<button class="btn btn-secondary portfolio-refresh" type="button" onclick={loadPortfolio} disabled={isLoading}>
				{m.common_refresh()}
			</button>
		</div>

		{#if error}
			<div class="alert alert-error">{error}</div>
		{/if}

		{#if isLoading}
			<div class="portfolio-loading">{m.common_loading()}</div>
		{:else}
			<div class="portfolio-grid">
				<article class="portfolio-summary">
					<div class="portfolio-summary-topline">
						<span class="portfolio-card-kicker">{m.dashboard_reviewPortfolioSummary()}</span>
					</div>
					<div class="portfolio-summary-figure">
						<strong>{attentionTenants.length}</strong>
						<span>{m.dashboard_reviewPortfolioTenantsWithAttention()}</span>
					</div>
					<div class="portfolio-summary-metrics">
						<div>
							<strong>{totalOpenTasks}</strong>
							<span>{m.dashboard_reviewPortfolioOpenItems()}</span>
						</div>
						<div>
							<strong>{formatCurrency(totalOverdueAmount)}</strong>
							<span>{m.dashboard_reviewPortfolioOverdueBalance()}</span>
						</div>
						<div>
							<strong>{totalUnmatchedTransactions}</strong>
							<span>{m.dashboard_reviewPortfolioUnmatched()}</span>
						</div>
						<div>
							<strong>{totalMissingEvidence}</strong>
							<span>{m.dashboard_reviewPortfolioEvidenceMissing()}</span>
						</div>
						<div>
							<strong>{totalPendingEvidence}</strong>
							<span>{m.dashboard_reviewPortfolioEvidencePending()}</span>
						</div>
						<div>
							<strong>{tenantsNeedingClose}</strong>
							<span>{m.dashboard_reviewPortfolioCloseDueCount()}</span>
						</div>
					</div>
				</article>

				<article class="portfolio-list-card">
					<div class="portfolio-summary-topline">
						<span class="portfolio-card-kicker">{m.dashboard_reviewPortfolioPriorityList()}</span>
					</div>

					{#if priorityItems.length > 0}
						<ul class="portfolio-list">
							{#each priorityItems as item}
								<li class:current={item.isCurrent}>
									<div class="portfolio-list-copy">
										<div class="portfolio-tenant-head">
											<div>
												<strong>{item.membership.tenant.name}</strong>
												<span>{item.membership.role}</span>
											</div>
											{#if item.isCurrent}
												<span class="portfolio-current">{m.dashboard_reviewPortfolioCurrent()}</span>
											{/if}
										</div>

										<div class="portfolio-pill-row">
											{#if item.overdueCount > 0}
												<span class="portfolio-pill">{item.overdueCount} {m.dashboard_reviewPortfolioOverdueTag()}</span>
											{/if}
											{#if item.unmatchedCount > 0}
												<span class="portfolio-pill">{item.unmatchedCount} {m.dashboard_reviewPortfolioBankingTag()}</span>
											{/if}
											{#if item.missingEvidenceCount > 0}
												<span class="portfolio-pill portfolio-pill-alert">{item.missingEvidenceCount} {m.dashboard_reviewPortfolioEvidenceMissingTag()}</span>
											{/if}
											{#if item.pendingEvidenceCount > 0}
												<span class="portfolio-pill">{item.pendingEvidenceCount} {m.dashboard_reviewPortfolioEvidencePendingTag()}</span>
											{/if}
											{#if item.needsClose}
												<span class="portfolio-pill portfolio-pill-alert">{m.dashboard_reviewPortfolioCloseTag()}</span>
											{/if}
											{#if !item.snapshot.tenant.onboarding_completed}
												<span class="portfolio-pill">{m.dashboard_reviewPortfolioSetupTag()}</span>
											{/if}
										</div>

										<p>{getCloseStatusLabel(item)}</p>
										{#if item.missingEvidenceCount > 0 || item.pendingEvidenceCount > 0}
											<p class="portfolio-evidence-note">
												{m.dashboard_reviewPortfolioEvidenceStatus({
													missing: item.missingEvidenceCount.toString(),
													pending: item.pendingEvidenceCount.toString()
												})}
											</p>
										{/if}

										{#if item.lastCloseEvent}
											<div class="portfolio-list-meta">
												<span>{m.dashboard_reviewPortfolioLastClose({ date: formatDate(item.lastCloseEvent.period_end_date) })}</span>
												<span>{formatDate(item.lastCloseEvent.created_at)}</span>
											</div>
										{/if}
									</div>

									<div class="portfolio-list-side">
										<strong>{item.openTasks}</strong>
										<span>{m.dashboard_reviewPortfolioOpenItems()}</span>
										<a href="/dashboard?tenant={item.membership.tenant.id}" class="btn btn-secondary">
											{item.isCurrent ? m.dashboard_reviewPortfolioCurrent() : m.dashboard_reviewPortfolioOpenTenant()}
										</a>
									</div>
								</li>
							{/each}
						</ul>
					{:else}
						<p class="portfolio-empty">{m.dashboard_reviewPortfolioNoIssues()}</p>
					{/if}
				</article>
			</div>
		{/if}
	</section>
{/if}

<style>
	.portfolio-board {
		display: grid;
		gap: 1.5rem;
		padding: 1.75rem;
		background:
			radial-gradient(circle at top left, rgba(206, 144, 93, 0.12), transparent 36%),
			linear-gradient(180deg, rgba(255, 253, 247, 0.96), rgba(250, 245, 236, 0.94));
		border: 1px solid rgba(121, 85, 58, 0.14);
		box-shadow: 0 18px 42px rgba(70, 44, 22, 0.1);
	}

	.portfolio-board-header {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 1rem;
	}

	.portfolio-kicker,
	.portfolio-card-kicker {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		font-size: 0.72rem;
		font-weight: 700;
		letter-spacing: 0.14em;
		text-transform: uppercase;
		color: rgba(111, 76, 52, 0.74);
	}

	.portfolio-board-header h3 {
		margin: 0.35rem 0 0.4rem;
		font-family: var(--font-display);
		font-size: clamp(1.7rem, 2vw, 2.25rem);
		line-height: 1;
		color: rgba(49, 34, 23, 0.96);
	}

	.portfolio-board-header p,
	.portfolio-list-copy p,
	.portfolio-loading,
	.portfolio-empty,
	.portfolio-list-meta span,
	.portfolio-summary-figure span,
	.portfolio-summary-metrics span,
	.portfolio-tenant-head span {
		margin: 0;
		color: rgba(83, 63, 48, 0.72);
	}

	.portfolio-refresh {
		white-space: nowrap;
	}

	.portfolio-grid {
		display: grid;
		grid-template-columns: minmax(0, 0.95fr) minmax(0, 1.45fr);
		gap: 1rem;
	}

	.portfolio-summary,
	.portfolio-list-card {
		display: grid;
		gap: 1rem;
		padding: 1.25rem;
		border-radius: 1.2rem;
		border: 1px solid rgba(121, 85, 58, 0.12);
		background: rgba(255, 252, 247, 0.74);
	}

	.portfolio-summary {
		background:
			linear-gradient(180deg, rgba(60, 42, 28, 0.96), rgba(95, 64, 42, 0.95)),
			rgba(60, 42, 28, 0.96);
	}

	.portfolio-summary .portfolio-card-kicker,
	.portfolio-summary strong,
	.portfolio-summary span {
		color: rgba(255, 247, 236, 0.9);
	}

	.portfolio-summary-figure {
		display: grid;
		gap: 0.25rem;
	}

	.portfolio-summary-figure strong {
		font-family: var(--font-display);
		font-size: clamp(2.6rem, 4vw, 3.75rem);
		line-height: 0.95;
	}

	.portfolio-summary-metrics {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 0.85rem;
	}

	.portfolio-summary-metrics div {
		display: grid;
		gap: 0.15rem;
		padding-top: 0.85rem;
		border-top: 1px solid rgba(255, 243, 226, 0.16);
	}

	.portfolio-summary-metrics strong {
		font-size: 1.15rem;
		font-weight: 700;
	}

	.portfolio-list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
	}

	.portfolio-list li {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		gap: 1rem;
		padding: 1rem 0;
		border-top: 1px solid rgba(121, 85, 58, 0.12);
	}

	.portfolio-list li:first-child {
		padding-top: 0;
		border-top: none;
	}

	.portfolio-list li.current {
		background: rgba(206, 144, 93, 0.08);
		margin: 0 -0.75rem;
		padding: 1rem 0.75rem;
		border-radius: 0.9rem;
	}

	.portfolio-list-copy,
	.portfolio-list-side {
		display: grid;
		gap: 0.5rem;
	}

	.portfolio-evidence-note {
		font-size: 0.88rem;
		color: rgba(83, 63, 48, 0.72);
	}

	.portfolio-tenant-head {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 0.75rem;
	}

	.portfolio-tenant-head div {
		display: grid;
		gap: 0.12rem;
	}

	.portfolio-tenant-head strong {
		font-size: 1rem;
		color: rgba(36, 23, 16, 0.95);
	}

	.portfolio-tenant-head span {
		text-transform: capitalize;
		font-size: 0.82rem;
	}

	.portfolio-current {
		align-self: flex-start;
		padding: 0.28rem 0.6rem;
		border-radius: 999px;
		font-size: 0.72rem;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: rgba(95, 64, 42, 0.92);
		background: rgba(206, 144, 93, 0.16);
	}

	.portfolio-pill-row {
		display: flex;
		flex-wrap: wrap;
		gap: 0.45rem;
	}

	.portfolio-pill {
		display: inline-flex;
		align-items: center;
		padding: 0.26rem 0.62rem;
		border-radius: 999px;
		background: rgba(121, 85, 58, 0.08);
		color: rgba(74, 53, 39, 0.92);
		font-size: 0.76rem;
		font-weight: 600;
	}

	.portfolio-pill-alert {
		background: rgba(179, 74, 34, 0.12);
		color: rgba(131, 48, 18, 0.94);
	}

	.portfolio-list-meta {
		display: flex;
		flex-wrap: wrap;
		gap: 0.75rem;
		font-size: 0.82rem;
	}

	.portfolio-list-side {
		align-content: start;
		justify-items: end;
		text-align: right;
		min-width: 8.5rem;
	}

	.portfolio-list-side strong {
		font-size: 1.6rem;
		line-height: 1;
		color: rgba(40, 27, 18, 0.94);
	}

	.portfolio-loading,
	.portfolio-empty {
		padding: 0.4rem 0 0;
	}

	@media (max-width: 960px) {
		.portfolio-grid {
			grid-template-columns: 1fr;
		}
	}

	@media (max-width: 720px) {
		.portfolio-board-header,
		.portfolio-tenant-head,
		.portfolio-list li {
			grid-template-columns: 1fr;
			flex-direction: column;
		}

		.portfolio-board-header {
			display: grid;
		}

		.portfolio-refresh,
		.portfolio-list-side {
			justify-self: stretch;
			justify-items: start;
			text-align: left;
		}

		.portfolio-summary-metrics {
			grid-template-columns: 1fr;
		}
	}
</style>
