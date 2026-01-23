<script lang="ts">
	import { page } from '$app/stores';
	import { api, type BankAccount, type BankTransaction, type MatchSuggestion, type TransactionStatus } from '$lib/api';
	import { goto } from '$app/navigation';
	import * as m from '$lib/paraglide/messages.js';
	import DateRangeFilter from '$lib/components/DateRangeFilter.svelte';
	import StatusBadge, { type StatusConfig } from '$lib/components/StatusBadge.svelte';
	import { formatDate } from '$lib/utils/formatting';

	let tenantId = $state('');
	let bankAccounts = $state<BankAccount[]>([]);
	let selectedAccount = $state<BankAccount | null>(null);
	let transactions = $state<BankTransaction[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Modal states
	let showAddAccountModal = $state(false);
	let showMatchModal = $state(false);
	let selectedTransaction = $state<BankTransaction | null>(null);
	let matchSuggestions = $state<MatchSuggestion[]>([]);
	let matchLoading = $state(false);

	// New account form
	let newAccount = $state({
		name: '',
		account_number: '',
		bank_name: '',
		currency: 'EUR',
		opening_balance: '0'
	});

	// Filter state
	let statusFilter = $state<'all' | 'UNMATCHED' | 'MATCHED' | 'RECONCILED'>('all');
	let filterFromDate = $state('');
	let filterToDate = $state('');

	$effect(() => {
		const urlTenantId = $page.url.searchParams.get('tenant');
		if (urlTenantId) {
			tenantId = urlTenantId;
			loadBankAccounts();
		} else {
			loadData();
		}
	});

	async function loadBankAccounts() {
		loading = true;
		error = null;
		try {
			bankAccounts = await api.listBankAccounts(tenantId);
			if (bankAccounts.length > 0 && !selectedAccount) {
				selectedAccount = bankAccounts[0];
			}
		} catch (e) {
			error = e instanceof Error ? e.message : m.banking_failedToLoad();
		} finally {
			loading = false;
		}
	}

	async function loadData() {
		loading = true;
		error = null;
		try {
			const memberships = await api.getMyTenants();
			if (memberships.length === 0) {
				error = m.banking_noTenantAvailable();
				return;
			}
			tenantId = memberships[0].tenant.id;
			bankAccounts = await api.listBankAccounts(tenantId);
			if (bankAccounts.length > 0 && !selectedAccount) {
				selectedAccount = bankAccounts[0];
			}
		} catch (e) {
			error = e instanceof Error ? e.message : m.banking_failedToLoad();
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		if (selectedAccount && tenantId) {
			loadTransactions();
		}
	});

	async function loadTransactions() {
		if (!selectedAccount) return;
		try {
			const filter: Record<string, string | undefined> = {};
			if (statusFilter !== 'all') filter.status = statusFilter;
			if (filterFromDate) filter.from_date = filterFromDate;
			if (filterToDate) filter.to_date = filterToDate;
			transactions = await api.listBankTransactions(tenantId, selectedAccount.id, Object.keys(filter).length > 0 ? filter : undefined);
		} catch (e) {
			console.error('Failed to load transactions:', e);
		}
	}

	function handleDateFilter() {
		loadTransactions();
	}

	async function createAccount() {
		try {
			const account = await api.createBankAccount(tenantId, newAccount);
			bankAccounts = [...bankAccounts, account];
			selectedAccount = account;
			showAddAccountModal = false;
			newAccount = { name: '', account_number: '', bank_name: '', currency: 'EUR', opening_balance: '0' };
		} catch (e) {
			alert(e instanceof Error ? e.message : m.banking_failedToCreate());
		}
	}

	async function openMatchModal(transaction: BankTransaction) {
		selectedTransaction = transaction;
		matchLoading = true;
		showMatchModal = true;
		try {
			matchSuggestions = await api.getMatchSuggestions(tenantId, transaction.id);
		} catch (e) {
			console.error('Failed to get suggestions:', e);
			matchSuggestions = [];
		} finally {
			matchLoading = false;
		}
	}

	async function matchToPayment(paymentId: string) {
		if (!selectedTransaction) return;
		try {
			await api.matchBankTransaction(tenantId, selectedTransaction.id, paymentId);
			showMatchModal = false;
			selectedTransaction = null;
			await loadTransactions();
		} catch (e) {
			alert(e instanceof Error ? e.message : m.banking_failedToMatch());
		}
	}

	async function unmatchTransaction(transactionId: string) {
		if (!confirm(m.banking_confirmUnmatch())) return;
		try {
			await api.unmatchBankTransaction(tenantId, transactionId);
			await loadTransactions();
		} catch (e) {
			alert(e instanceof Error ? e.message : m.banking_failedToUnmatch());
		}
	}

	async function createPaymentFromTransaction(transactionId: string) {
		try {
			await api.createPaymentFromTransaction(tenantId, transactionId);
			await loadTransactions();
		} catch (e) {
			alert(e instanceof Error ? e.message : m.banking_failedToCreatePayment());
		}
	}

	async function autoMatch() {
		if (!selectedAccount) return;
		try {
			const result = await api.autoMatchTransactions(tenantId, selectedAccount.id, 0.7);
			alert(m.banking_matchedCount({ count: result.matched.toString() }));
			await loadTransactions();
		} catch (e) {
			alert(e instanceof Error ? e.message : m.banking_failedToAutoMatch());
		}
	}

	function formatAmount(amount: any): string {
		if (amount === null || amount === undefined) return '0.00';
		const num = typeof amount === 'object' && amount.toNumber ? amount.toNumber() : Number(amount);
		if (isNaN(num)) return '0.00';
		return new Intl.NumberFormat('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(num);
	}

	const statusConfig: Record<TransactionStatus, StatusConfig> = {
		UNMATCHED: { class: 'badge-unmatched', label: m.banking_unmatched() },
		MATCHED: { class: 'badge-matched', label: m.banking_matched() },
		RECONCILED: { class: 'badge-reconciled', label: m.banking_reconciled() }
	};

	const filteredTransactions = $derived(
		statusFilter === 'all' ? transactions : transactions.filter(t => t.status === statusFilter)
	);
</script>

<svelte:head>
	<title>{m.banking_bankReconciliation()} - Open Accounting</title>
</svelte:head>

<div class="max-w-7xl mx-auto px-4 py-8">
	<div class="flex justify-between items-center mb-6">
		<h1 class="text-2xl font-bold text-gray-900">{m.banking_bankReconciliation()}</h1>
		<div class="flex gap-2">
			<button onclick={() => showAddAccountModal = true} class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700">
				{m.banking_addBankAccount()}
			</button>
			{#if selectedAccount}
				<button onclick={() => goto(`/banking/import?account=${selectedAccount?.id}`)} class="px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700">
					{m.banking_importTransactions()}
				</button>
			{/if}
		</div>
	</div>

	{#if loading}
		<div class="flex justify-center py-12">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
		</div>
	{:else if error}
		<div class="bg-red-50 text-red-600 p-4 rounded-lg">{error}</div>
	{:else if bankAccounts.length === 0}
		<div class="empty-state card">
			<p>{m.banking_noAccounts()}</p>
		</div>
	{:else}
		<!-- Bank Account Selector -->
		<div class="bg-white rounded-lg shadow mb-6 p-4">
			<div class="flex flex-wrap gap-4 items-center">
				<label class="font-medium text-gray-700">{m.banking_bankAccount()}</label>
				<select
					bind:value={selectedAccount}
					class="border border-gray-300 rounded-lg px-3 py-2 min-w-[200px]"
				>
					{#each bankAccounts as account}
						<option value={account}>{account.name} ({account.account_number})</option>
					{/each}
				</select>
				{#if selectedAccount}
					<div class="ml-auto flex items-center gap-4">
						<span class="text-gray-600">{m.banking_balance()} <strong class="text-gray-900">{selectedAccount.currency} {formatAmount(selectedAccount.current_balance)}</strong></span>
						<button onclick={autoMatch} class="px-3 py-1 text-sm bg-purple-100 text-purple-700 rounded hover:bg-purple-200">
							{m.banking_autoMatch()}
						</button>
					</div>
				{/if}
			</div>
		</div>

		{#if selectedAccount}
			<!-- Filters -->
			<div class="bg-white rounded-lg shadow mb-6 p-4">
				<div class="flex flex-wrap gap-4 items-center">
					<div class="flex gap-2">
						<button
							onclick={() => statusFilter = 'all'}
							class="px-3 py-1 rounded {statusFilter === 'all' ? 'bg-blue-600 text-white' : 'bg-gray-100 text-gray-700 hover:bg-gray-200'}"
						>{m.banking_all()}</button>
						<button
							onclick={() => statusFilter = 'UNMATCHED'}
							class="px-3 py-1 rounded {statusFilter === 'UNMATCHED' ? 'bg-yellow-600 text-white' : 'bg-yellow-100 text-yellow-700 hover:bg-yellow-200'}"
						>{m.banking_unmatched()}</button>
						<button
							onclick={() => statusFilter = 'MATCHED'}
							class="px-3 py-1 rounded {statusFilter === 'MATCHED' ? 'bg-green-600 text-white' : 'bg-green-100 text-green-700 hover:bg-green-200'}"
						>{m.banking_matched()}</button>
						<button
							onclick={() => statusFilter = 'RECONCILED'}
							class="px-3 py-1 rounded {statusFilter === 'RECONCILED' ? 'bg-blue-600 text-white' : 'bg-blue-100 text-blue-700 hover:bg-blue-200'}"
						>{m.banking_reconciled()}</button>
					</div>
					<DateRangeFilter
						bind:fromDate={filterFromDate}
						bind:toDate={filterToDate}
						onchange={handleDateFilter}
						compact
					/>
				</div>
			</div>

			<!-- Transactions Table -->
			<div class="bg-white rounded-lg shadow overflow-hidden">
				<table class="min-w-full divide-y divide-gray-200">
					<thead class="bg-gray-50">
						<tr>
							<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">{m.common_date()}</th>
							<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">{m.common_description()}</th>
							<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">{m.banking_counterparty()}</th>
							<th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase">{m.common_amount()}</th>
							<th class="px-6 py-3 text-center text-xs font-medium text-gray-500 uppercase">{m.common_status()}</th>
							<th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase">{m.common_actions()}</th>
						</tr>
					</thead>
					<tbody class="bg-white divide-y divide-gray-200">
						{#each filteredTransactions as transaction}
							<tr class="hover:bg-gray-50">
								<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
									{formatDate(transaction.transaction_date)}
								</td>
								<td class="px-6 py-4 text-sm text-gray-900">
									<div class="max-w-xs truncate">{transaction.description}</div>
									{#if transaction.reference}
										<div class="text-xs text-gray-500">{m.banking_ref()} {transaction.reference}</div>
									{/if}
								</td>
								<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-600">
									{transaction.counterparty_name || '-'}
								</td>
								<td class="px-6 py-4 whitespace-nowrap text-sm text-right font-mono {Number(transaction.amount) >= 0 ? 'text-green-600' : 'text-red-600'}">
									{formatAmount(transaction.amount)}
								</td>
								<td class="px-6 py-4 whitespace-nowrap text-center">
									<StatusBadge status={transaction.status} config={statusConfig} />
								</td>
								<td class="px-6 py-4 whitespace-nowrap text-right text-sm">
									{#if transaction.status === 'UNMATCHED'}
										<button onclick={() => openMatchModal(transaction)} class="text-blue-600 hover:text-blue-800 mr-2">
											{m.banking_match()}
										</button>
										<button onclick={() => createPaymentFromTransaction(transaction.id)} class="text-green-600 hover:text-green-800">
											{m.banking_createPayment()}
										</button>
									{:else if transaction.status === 'MATCHED'}
										<button onclick={() => unmatchTransaction(transaction.id)} class="text-red-600 hover:text-red-800">
											{m.banking_unmatch()}
										</button>
									{/if}
								</td>
							</tr>
						{:else}
							<tr>
								<td colspan="6" class="px-6 py-12 text-center text-gray-500">
									{m.banking_noTransactions()}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{:else}
			<div class="bg-white rounded-lg shadow p-12 text-center">
				<p class="text-gray-500 mb-4">{m.banking_noAccounts()}</p>
				<button onclick={() => showAddAccountModal = true} class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700">
					{m.banking_addFirstAccount()}
				</button>
			</div>
		{/if}
	{/if}
</div>

<!-- Add Account Modal -->
{#if showAddAccountModal}
	<div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
		<div class="bg-white rounded-lg shadow-xl w-full max-w-md p-6">
			<h2 class="text-xl font-bold mb-4">{m.banking_addBankAccount()}</h2>
			<form onsubmit={(e) => { e.preventDefault(); createAccount(); }}>
				<div class="space-y-4">
					<div>
						<label class="block text-sm font-medium text-gray-700 mb-1">{m.banking_accountName()}</label>
						<input type="text" bind:value={newAccount.name} required class="w-full border border-gray-300 rounded-lg px-3 py-2" placeholder="Main Business Account" />
					</div>
					<div>
						<label class="block text-sm font-medium text-gray-700 mb-1">{m.banking_accountNumber()}</label>
						<input type="text" bind:value={newAccount.account_number} required class="w-full border border-gray-300 rounded-lg px-3 py-2" placeholder="EE123456789" />
					</div>
					<div>
						<label class="block text-sm font-medium text-gray-700 mb-1">{m.banking_bankName()}</label>
						<input type="text" bind:value={newAccount.bank_name} class="w-full border border-gray-300 rounded-lg px-3 py-2" placeholder="Swedbank" />
					</div>
					<div class="grid grid-cols-2 gap-4">
						<div>
							<label class="block text-sm font-medium text-gray-700 mb-1">{m.settings_currency()}</label>
							<select bind:value={newAccount.currency} class="w-full border border-gray-300 rounded-lg px-3 py-2">
								<option value="EUR">EUR</option>
								<option value="USD">USD</option>
								<option value="GBP">GBP</option>
							</select>
						</div>
						<div>
							<label class="block text-sm font-medium text-gray-700 mb-1">{m.banking_openingBalance()}</label>
							<input type="text" bind:value={newAccount.opening_balance} class="w-full border border-gray-300 rounded-lg px-3 py-2" placeholder="0.00" />
						</div>
					</div>
				</div>
				<div class="mt-6 flex justify-end gap-2">
					<button type="button" onclick={() => showAddAccountModal = false} class="px-4 py-2 border border-gray-300 rounded-lg hover:bg-gray-50">{m.common_cancel()}</button>
					<button type="submit" class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700">{m.banking_create()}</button>
				</div>
			</form>
		</div>
	</div>
{/if}

<!-- Match Modal -->
{#if showMatchModal && selectedTransaction}
	<div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
		<div class="bg-white rounded-lg shadow-xl w-full max-w-2xl p-6">
			<h2 class="text-xl font-bold mb-4">{m.banking_matchTransaction()}</h2>
			<div class="bg-gray-50 rounded-lg p-4 mb-4">
				<div class="grid grid-cols-2 gap-4">
					<div>
						<span class="text-gray-500 text-sm">{m.common_date()}:</span>
						<span class="ml-2 font-medium">{formatDate(selectedTransaction.transaction_date)}</span>
					</div>
					<div>
						<span class="text-gray-500 text-sm">{m.common_amount()}:</span>
						<span class="ml-2 font-medium font-mono {Number(selectedTransaction.amount) >= 0 ? 'text-green-600' : 'text-red-600'}">
							{formatAmount(selectedTransaction.amount)}
						</span>
					</div>
					<div class="col-span-2">
						<span class="text-gray-500 text-sm">{m.common_description()}:</span>
						<span class="ml-2">{selectedTransaction.description}</span>
					</div>
				</div>
			</div>

			{#if matchLoading}
				<div class="flex justify-center py-8">
					<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
				</div>
			{:else if matchSuggestions.length === 0}
				<p class="text-gray-500 text-center py-8">{m.banking_noMatchingPayments()}</p>
			{:else}
				<h3 class="font-medium mb-2">{m.banking_suggestedMatches()}</h3>
				<div class="space-y-2 max-h-80 overflow-y-auto">
					{#each matchSuggestions as suggestion}
						<div class="border border-gray-200 rounded-lg p-4 hover:bg-gray-50 flex justify-between items-center">
							<div>
								<div class="font-medium">{suggestion.payment_number}</div>
								<div class="text-sm text-gray-600">
									{formatDate(suggestion.payment_date)} - {suggestion.contact_name || m.banking_noContact()}
								</div>
								<div class="text-xs text-gray-500">{suggestion.match_reason}</div>
							</div>
							<div class="text-right">
								<div class="font-mono font-medium">{formatAmount(suggestion.amount)}</div>
								<div class="text-xs text-gray-500">{m.banking_confidence()} {Math.round(suggestion.confidence * 100)}%</div>
								<button onclick={() => matchToPayment(suggestion.payment_id)} class="mt-1 px-3 py-1 text-sm bg-blue-600 text-white rounded hover:bg-blue-700">
									{m.banking_match()}
								</button>
							</div>
						</div>
					{/each}
				</div>
			{/if}

			<div class="mt-6 flex justify-end">
				<button onclick={() => { showMatchModal = false; selectedTransaction = null; }} class="px-4 py-2 border border-gray-300 rounded-lg hover:bg-gray-50">
					{m.common_cancel()}
				</button>
			</div>
		</div>
	</div>
{/if}
