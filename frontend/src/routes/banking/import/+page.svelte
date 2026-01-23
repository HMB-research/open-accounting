<script lang="ts">
	import { api, type BankAccount, type CSVColumnMapping } from '$lib/api';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import * as m from '$lib/paraglide/messages.js';

	let tenantId = $state('');
	let bankAccounts = $state<BankAccount[]>([]);
	let selectedAccountId = $state<string | null>(null);
	let loading = $state(true);
	let importing = $state(false);
	let error = $state<string | null>(null);

	// File state
	let csvFile = $state<File | null>(null);
	let csvContent = $state('');
	let csvPreview = $state<string[][]>([]);

	// Mapping configuration
	let mapping = $state<CSVColumnMapping>({
		date_column: 0,
		description_column: 1,
		amount_column: 2,
		reference_column: undefined,
		counterparty_column: undefined,
		date_format: '2006-01-02',
		decimal_separator: '.',
		thousands_separator: '',
		skip_header: true
	});

	let skipDuplicates = $state(true);

	// Preset bank formats
	const presets: Record<string, CSVColumnMapping> = {
		generic: {
			date_column: 0,
			description_column: 1,
			amount_column: 2,
			date_format: '2006-01-02',
			decimal_separator: '.',
			thousands_separator: '',
			skip_header: true
		},
		swedbank: {
			date_column: 0,
			description_column: 2,
			amount_column: 3,
			reference_column: 4,
			counterparty_column: 1,
			date_format: '02.01.2006',
			decimal_separator: ',',
			thousands_separator: ' ',
			skip_header: true
		},
		seb: {
			date_column: 0,
			description_column: 3,
			amount_column: 4,
			reference_column: 2,
			counterparty_column: 1,
			date_format: '02.01.2006',
			decimal_separator: ',',
			thousands_separator: '',
			skip_header: true
		},
		lhv: {
			date_column: 0,
			description_column: 2,
			amount_column: 3,
			reference_column: 4,
			counterparty_column: 1,
			date_format: '02.01.2006',
			decimal_separator: ',',
			thousands_separator: '',
			skip_header: true
		}
	};

	$effect(() => {
		loadData();
	});

	async function loadData() {
		loading = true;
		try {
			// Check URL param for tenant first
			const urlTenantId = $page.url.searchParams.get('tenant');
			if (urlTenantId) {
				tenantId = urlTenantId;
			} else {
				const memberships = await api.getMyTenants();
				if (memberships.length === 0) {
					error = m.bankingImport_noTenantAvailable();
					return;
				}
				tenantId = memberships[0].tenant.id;
			}
			bankAccounts = await api.listBankAccounts(tenantId);

			// Check URL param for account selection
			const urlAccountId = $page.url.searchParams.get('account');
			if (urlAccountId && bankAccounts.some((a) => a.id === urlAccountId)) {
				selectedAccountId = urlAccountId;
			} else if (bankAccounts.length > 0) {
				selectedAccountId = bankAccounts[0].id;
			}
		} catch (e) {
			error = e instanceof Error ? e.message : m.bankingImport_failedToLoad();
		} finally {
			loading = false;
		}
	}

	function handleFileSelect(event: Event) {
		const input = event.target as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;

		csvFile = file;
		const reader = new FileReader();
		reader.onload = (e) => {
			csvContent = e.target?.result as string;
			parsePreview();
		};
		reader.readAsText(file);
	}

	function parsePreview() {
		if (!csvContent) return;
		const lines = csvContent.split('\n').filter((line) => line.trim());
		const preview: string[][] = [];

		for (let i = 0; i < Math.min(lines.length, 10); i++) {
			const line = lines[i];
			// Simple CSV parsing (handles basic cases)
			const columns = parseCSVLine(line);
			preview.push(columns);
		}

		csvPreview = preview;
	}

	function parseCSVLine(line: string): string[] {
		const result: string[] = [];
		let current = '';
		let inQuotes = false;

		for (let i = 0; i < line.length; i++) {
			const char = line[i];
			if (char === '"') {
				inQuotes = !inQuotes;
			} else if ((char === ',' || char === ';') && !inQuotes) {
				result.push(current.trim());
				current = '';
			} else {
				current += char;
			}
		}
		result.push(current.trim());
		return result;
	}

	function applyPreset(presetName: string) {
		if (presets[presetName]) {
			mapping = { ...presets[presetName] };
		}
	}

	async function importTransactions() {
		if (!selectedAccountId || !csvContent || !csvFile) {
			alert(m.bankingImport_selectFileAndAccount());
			return;
		}

		importing = true;
		error = null;

		try {
			const result = await api.importBankTransactions(tenantId, selectedAccountId, {
				csv_content: csvContent,
				file_name: csvFile.name,
				mapping,
				skip_duplicates: skipDuplicates
			});

			let message: string = m.bankingImport_importedTransactions({ count: result.transactions_imported.toString() });
			if (result.transactions_duplicates > 0) {
				message += `, ${m.bankingImport_duplicatesSkipped({ count: result.transactions_duplicates.toString() })}`;
			}
			if (result.errors.length > 0) {
				message += `\n\nErrors:\n${result.errors.join('\n')}`;
			}

			alert(message);
			goto('/banking');
		} catch (e) {
			error = e instanceof Error ? e.message : m.bankingImport_importFailed();
		} finally {
			importing = false;
		}
	}

	function getColumnClass(colIndex: number): string {
		const classes = [];
		if (mapping.date_column === colIndex) classes.push('bg-blue-100');
		if (mapping.description_column === colIndex) classes.push('bg-green-100');
		if (mapping.amount_column === colIndex) classes.push('bg-yellow-100');
		if (mapping.reference_column === colIndex) classes.push('bg-purple-100');
		if (mapping.counterparty_column === colIndex) classes.push('bg-pink-100');
		return classes.join(' ') || 'bg-gray-50';
	}
</script>

<svelte:head>
	<title>Import Bank Transactions</title>
</svelte:head>

<div class="max-w-6xl mx-auto px-4 py-8">
	<div class="flex items-center gap-4 mb-6">
		<button onclick={() => goto('/banking')} class="text-gray-600 hover:text-gray-800">
			&larr; {m.common_back()}
		</button>
		<h1 class="text-2xl font-bold text-gray-900">{m.bankingImport_title()}</h1>
	</div>

	{#if loading}
		<div class="flex justify-center py-12">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
		</div>
	{:else if error}
		<div class="bg-red-50 text-red-600 p-4 rounded-lg mb-4">{error}</div>
	{:else}
		<div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
			<!-- Settings Panel -->
			<div class="bg-white rounded-lg shadow p-6">
				<h2 class="text-lg font-semibold mb-4">{m.bankingImport_importSettings()}</h2>

				<!-- Bank Account -->
				<div class="mb-4">
					<label for="import-bank-account" class="block text-sm font-medium text-gray-700 mb-1">{m.bankingImport_bankAccount()}</label>
					<select id="import-bank-account" bind:value={selectedAccountId} class="w-full border border-gray-300 rounded-lg px-3 py-2">
						{#each bankAccounts as account}
							<option value={account.id}>{account.name}</option>
						{/each}
					</select>
				</div>

				<!-- File Upload -->
				<div class="mb-4">
					<label for="import-csv-file" class="block text-sm font-medium text-gray-700 mb-1">{m.bankingImport_csvFile()}</label>
					<input id="import-csv-file" type="file" accept=".csv,.txt" onchange={handleFileSelect} class="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm" />
				</div>

				<!-- Bank Preset -->
				<div class="mb-4">
					<label for="import-bank-preset" class="block text-sm font-medium text-gray-700 mb-1">{m.bankingImport_bankFormatPreset()}</label>
					<select id="import-bank-preset" onchange={(e) => applyPreset((e.target as HTMLSelectElement).value)} class="w-full border border-gray-300 rounded-lg px-3 py-2">
						<option value="generic">{m.bankingImport_genericCsv()}</option>
						<option value="swedbank">{m.bankingImport_swedbankEstonia()}</option>
						<option value="seb">{m.bankingImport_sebEstonia()}</option>
						<option value="lhv">{m.bankingImport_lhvEstonia()}</option>
					</select>
				</div>

				<hr class="my-4" />

				<h3 class="font-medium mb-3">{m.bankingImport_columnMapping()}</h3>

				<div class="grid grid-cols-2 gap-3 mb-4">
					<div>
						<label for="mapping-date-column" class="block text-xs font-medium text-blue-700 mb-1">{m.bankingImport_dateColumn()}</label>
						<input id="mapping-date-column" type="number" bind:value={mapping.date_column} min="0" class="w-full border border-gray-300 rounded px-2 py-1 text-sm" />
					</div>
					<div>
						<label for="mapping-description-column" class="block text-xs font-medium text-green-700 mb-1">{m.bankingImport_descriptionColumn()}</label>
						<input id="mapping-description-column" type="number" bind:value={mapping.description_column} min="0" class="w-full border border-gray-300 rounded px-2 py-1 text-sm" />
					</div>
					<div>
						<label for="mapping-amount-column" class="block text-xs font-medium text-yellow-700 mb-1">{m.bankingImport_amountColumn()}</label>
						<input id="mapping-amount-column" type="number" bind:value={mapping.amount_column} min="0" class="w-full border border-gray-300 rounded px-2 py-1 text-sm" />
					</div>
					<div>
						<label for="mapping-reference-column" class="block text-xs font-medium text-purple-700 mb-1">{m.bankingImport_referenceColumn()}</label>
						<input id="mapping-reference-column" type="number" bind:value={mapping.reference_column} min="0" class="w-full border border-gray-300 rounded px-2 py-1 text-sm" placeholder={m.common_optional()} />
					</div>
					<div class="col-span-2">
						<label for="mapping-counterparty-column" class="block text-xs font-medium text-pink-700 mb-1">{m.bankingImport_counterpartyColumn()}</label>
						<input id="mapping-counterparty-column" type="number" bind:value={mapping.counterparty_column} min="0" class="w-full border border-gray-300 rounded px-2 py-1 text-sm" placeholder={m.common_optional()} />
					</div>
				</div>

				<h3 class="font-medium mb-3">{m.bankingImport_formatSettings()}</h3>

				<div class="grid grid-cols-2 gap-3 mb-4">
					<div class="col-span-2">
						<label for="mapping-date-format" class="block text-xs font-medium text-gray-700 mb-1">{m.bankingImport_dateFormat()}</label>
						<select id="mapping-date-format" bind:value={mapping.date_format} class="w-full border border-gray-300 rounded px-2 py-1 text-sm">
							<option value="2006-01-02">YYYY-MM-DD</option>
							<option value="02.01.2006">DD.MM.YYYY</option>
							<option value="01/02/2006">MM/DD/YYYY</option>
							<option value="02/01/2006">DD/MM/YYYY</option>
						</select>
					</div>
					<div>
						<label for="mapping-decimal-sep" class="block text-xs font-medium text-gray-700 mb-1">{m.bankingImport_decimalSep()}</label>
						<select id="mapping-decimal-sep" bind:value={mapping.decimal_separator} class="w-full border border-gray-300 rounded px-2 py-1 text-sm">
							<option value=".">{m.bankingImport_dotDecimal()}</option>
							<option value=",">{m.bankingImport_commaDecimal()}</option>
						</select>
					</div>
					<div>
						<label for="mapping-thousands-sep" class="block text-xs font-medium text-gray-700 mb-1">{m.bankingImport_thousandsSep()}</label>
						<select id="mapping-thousands-sep" bind:value={mapping.thousands_separator} class="w-full border border-gray-300 rounded px-2 py-1 text-sm">
							<option value="">{m.bankingImport_none()}</option>
							<option value=",">{m.bankingImport_commaDecimal()}</option>
							<option value=" ">{m.bankingImport_space()}</option>
						</select>
					</div>
				</div>

				<label class="flex items-center gap-2 mb-4">
					<input type="checkbox" bind:checked={mapping.skip_header} class="rounded" />
					<span class="text-sm">{m.bankingImport_skipHeaderRow()}</span>
				</label>

				<label class="flex items-center gap-2 mb-6">
					<input type="checkbox" bind:checked={skipDuplicates} class="rounded" />
					<span class="text-sm">{m.bankingImport_skipDuplicates()}</span>
				</label>

				<button
					onclick={importTransactions}
					disabled={importing || !csvContent || !selectedAccountId}
					class="w-full py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:bg-gray-400 disabled:cursor-not-allowed"
				>
					{importing ? m.bankingImport_importing() : m.bankingImport_importTransactions()}
				</button>
			</div>

			<!-- Preview Panel -->
			<div class="lg:col-span-2 bg-white rounded-lg shadow p-6">
				<h2 class="text-lg font-semibold mb-4">{m.bankingImport_filePreview()}</h2>

				{#if csvPreview.length === 0}
					<div class="text-center py-12 text-gray-500">
						<p>{m.bankingImport_selectCsvFile()}</p>
					</div>
				{:else}
					<div class="mb-4 flex flex-wrap gap-2 text-xs">
						<span class="px-2 py-1 bg-blue-100 text-blue-700 rounded">{m.common_date()}</span>
						<span class="px-2 py-1 bg-green-100 text-green-700 rounded">{m.common_description()}</span>
						<span class="px-2 py-1 bg-yellow-100 text-yellow-700 rounded">{m.common_amount()}</span>
						<span class="px-2 py-1 bg-purple-100 text-purple-700 rounded">{m.banking_reference()}</span>
						<span class="px-2 py-1 bg-pink-100 text-pink-700 rounded">{m.banking_counterparty()}</span>
					</div>

					<div class="overflow-x-auto">
						<table class="min-w-full text-sm border border-gray-200">
							<thead class="bg-gray-100">
								<tr>
									<th class="px-2 py-1 text-left text-xs text-gray-500">#</th>
									{#each csvPreview[0] || [] as _, i}
										<th class="px-2 py-1 text-left text-xs text-gray-500 {getColumnClass(i)}">
											Col {i}
										</th>
									{/each}
								</tr>
							</thead>
							<tbody>
								{#each csvPreview as row, rowIndex}
									<tr class="{mapping.skip_header && rowIndex === 0 ? 'opacity-50' : ''}">
										<td class="px-2 py-1 border-t text-gray-400">{rowIndex}</td>
										{#each row as cell, colIndex}
											<td class="px-2 py-1 border-t max-w-[150px] truncate {getColumnClass(colIndex)}">
												{cell}
											</td>
										{/each}
									</tr>
								{/each}
							</tbody>
						</table>
					</div>

					<p class="mt-2 text-xs text-gray-500">{m.bankingImport_showingRows({ count: csvPreview.length.toString() })}</p>
				{/if}
			</div>
		</div>
	{/if}
</div>
