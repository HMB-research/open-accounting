<script lang="ts">
	import { api, type KMDDeclaration } from '$lib/api';
	import { goto } from '$app/navigation';
	import Decimal from 'decimal.js';

	let tenantId = $state('');
	let loading = $state(true);
	let generating = $state(false);
	let error = $state<string | null>(null);
	let declarations = $state<KMDDeclaration[]>([]);

	let selectedYear = $state(new Date().getFullYear());
	let selectedMonth = $state(new Date().getMonth() + 1);

	$effect(() => {
		loadData();
	});

	async function loadData() {
		loading = true;
		error = null;
		try {
			const memberships = await api.getMyTenants();
			if (memberships.length === 0) {
				error = 'No tenant available';
				return;
			}
			tenantId = memberships[0].tenant.id;
			declarations = await api.listKMD(tenantId);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load data';
		} finally {
			loading = false;
		}
	}

	async function generateKMD() {
		generating = true;
		error = null;
		try {
			const decl = await api.generateKMD(tenantId, {
				year: selectedYear,
				month: selectedMonth
			});
			// Reload to get the updated list
			declarations = await api.listKMD(tenantId);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to generate KMD';
		} finally {
			generating = false;
		}
	}

	async function downloadXml(decl: KMDDeclaration) {
		try {
			await api.downloadKMDXml(tenantId, decl.year, decl.month);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to download XML';
		}
	}

	function formatCurrency(value: Decimal | string | number): string {
		const num = value instanceof Decimal ? value.toNumber() : Number(value);
		return new Intl.NumberFormat('et-EE', {
			style: 'currency',
			currency: 'EUR'
		}).format(num);
	}

	function getPayable(decl: KMDDeclaration): Decimal {
		const output =
			decl.total_output_vat instanceof Decimal
				? decl.total_output_vat
				: new Decimal(decl.total_output_vat || 0);
		const input =
			decl.total_input_vat instanceof Decimal
				? decl.total_input_vat
				: new Decimal(decl.total_input_vat || 0);
		return output.minus(input);
	}
</script>

<svelte:head>
	<title>VAT Declarations - Open Accounting</title>
</svelte:head>

<div class="max-w-6xl mx-auto px-4 py-8">
	<div class="flex items-center justify-between mb-6">
		<div class="flex items-center gap-4">
			<button onclick={() => goto('/dashboard')} class="text-gray-600 hover:text-gray-800">
				&larr; Back
			</button>
			<h1 class="text-2xl font-bold text-gray-900">VAT Declarations (KMD)</h1>
		</div>
	</div>

	{#if error}
		<div class="bg-red-50 text-red-600 p-4 rounded-lg mb-4">{error}</div>
	{/if}

	{#if loading}
		<div class="flex justify-center py-12">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
		</div>
	{:else}
		<!-- Generate KMD Form -->
		<div class="bg-white rounded-lg shadow p-6 mb-6">
			<h2 class="text-lg font-semibold mb-4">Generate VAT Declaration</h2>
			<div class="flex gap-4 items-end">
				<div>
					<label for="year" class="block text-sm font-medium text-gray-700 mb-1">Year</label>
					<select
						id="year"
						bind:value={selectedYear}
						class="border border-gray-300 rounded-lg px-3 py-2"
					>
						{#each [2024, 2025, 2026] as year}
							<option value={year}>{year}</option>
						{/each}
					</select>
				</div>
				<div>
					<label for="month" class="block text-sm font-medium text-gray-700 mb-1">Month</label>
					<select
						id="month"
						bind:value={selectedMonth}
						class="border border-gray-300 rounded-lg px-3 py-2"
					>
						{#each Array.from({ length: 12 }, (_, i) => i + 1) as month}
							<option value={month}>{String(month).padStart(2, '0')}</option>
						{/each}
					</select>
				</div>
				<button
					onclick={generateKMD}
					disabled={generating}
					class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:bg-gray-400"
				>
					{generating ? 'Generating...' : 'Generate KMD'}
				</button>
			</div>
		</div>

		<!-- Declarations List -->
		{#if declarations.length > 0}
			<div class="bg-white rounded-lg shadow">
				<div class="px-6 py-4 border-b">
					<h2 class="text-lg font-semibold">Generated Declarations</h2>
				</div>
				<div class="divide-y">
					{#each declarations as decl}
						<div class="px-6 py-4 flex items-center justify-between">
							<div>
								<div class="font-medium">{decl.year}-{String(decl.month).padStart(2, '0')}</div>
								<div class="text-sm text-gray-500">
									Output VAT: {formatCurrency(decl.total_output_vat)} | Input VAT: {formatCurrency(
										decl.total_input_vat
									)} | Payable: {formatCurrency(getPayable(decl))}
								</div>
							</div>
							<div class="flex gap-2 items-center">
								<span
									class="px-2 py-1 text-xs rounded-full {decl.status === 'DRAFT'
										? 'bg-yellow-100 text-yellow-800'
										: decl.status === 'SUBMITTED'
											? 'bg-blue-100 text-blue-800'
											: 'bg-green-100 text-green-800'}"
								>
									{decl.status}
								</span>
								<button
									onclick={() => downloadXml(decl)}
									class="px-3 py-1 text-sm border border-gray-300 rounded hover:bg-gray-50"
								>
									Download XML
								</button>
							</div>
						</div>
					{/each}
				</div>
			</div>
		{:else}
			<div class="bg-white rounded-lg shadow p-12 text-center text-gray-500">
				<p>No VAT declarations yet. Generate one to get started.</p>
			</div>
		{/if}
	{/if}
</div>
