<script lang="ts">
	import ExcelJS from 'exceljs';
	import * as m from '$lib/paraglide/messages.js';

	interface Props {
		data: Record<string, unknown>[][];
		headers: string[][];
		filename: string;
		sheetNames?: string[];
	}

	let { data, headers, filename, sheetNames = ['Sheet1'] }: Props = $props();
	let isOpen = $state(false);

	async function exportToExcel() {
		const workbook = new ExcelJS.Workbook();

		data.forEach((sheetData, index) => {
			const sheetHeaders = headers[index] || headers[0];
			const worksheet = workbook.addWorksheet(sheetNames[index] || `Sheet${index + 1}`);

			// Set columns with headers and auto-width
			worksheet.columns = sheetHeaders.map((header, colIndex) => {
				const maxLength = Math.max(
					header.length,
					...sheetData.map(row => {
						const value = Object.values(row)[colIndex];
						return value !== undefined ? String(value).length : 0;
					})
				);
				return {
					header: header,
					key: `col${colIndex}`,
					width: Math.min(maxLength + 2, 50)
				};
			});

			// Add data rows
			sheetData.forEach(row => {
				const rowData: Record<string, unknown> = {};
				sheetHeaders.forEach((_, colIndex) => {
					const value = Object.values(row)[colIndex];
					rowData[`col${colIndex}`] = value !== undefined ? value : '';
				});
				worksheet.addRow(rowData);
			});
		});

		const buffer = await workbook.xlsx.writeBuffer();
		const blob = new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `${filename}.xlsx`;
		a.click();
		URL.revokeObjectURL(url);
		isOpen = false;
	}

	function exportToCsv() {
		// Export first sheet as CSV
		const sheetData = data[0] || [];
		const sheetHeaders = headers[0] || [];

		const csvContent = [
			sheetHeaders.join(','),
			...sheetData.map(row =>
				sheetHeaders.map((_, colIndex) => {
					const value = Object.values(row)[colIndex];
					const strValue = value !== undefined ? String(value) : '';
					// Escape quotes and wrap in quotes if contains comma or quote
					if (strValue.includes(',') || strValue.includes('"') || strValue.includes('\n')) {
						return `"${strValue.replace(/"/g, '""')}"`;
					}
					return strValue;
				}).join(',')
			)
		].join('\n');

		const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
		const link = document.createElement('a');
		link.href = URL.createObjectURL(blob);
		link.download = `${filename}.csv`;
		link.click();
		URL.revokeObjectURL(link.href);
		isOpen = false;
	}

	function exportToPdf() {
		window.print();
		isOpen = false;
	}

	function toggleDropdown() {
		isOpen = !isOpen;
	}

	function handleClickOutside(event: MouseEvent) {
		const target = event.target as HTMLElement;
		if (!target.closest('.export-dropdown')) {
			isOpen = false;
		}
	}
</script>

<svelte:window onclick={handleClickOutside} />

<div class="export-dropdown">
	<button class="btn btn-secondary" onclick={toggleDropdown} type="button">
		<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
			<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path>
			<polyline points="7 10 12 15 17 10"></polyline>
			<line x1="12" y1="15" x2="12" y2="3"></line>
		</svg>
		{m.reports_export()}
		<svg class="chevron" class:open={isOpen} xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
			<polyline points="6 9 12 15 18 9"></polyline>
		</svg>
	</button>

	{#if isOpen}
		<div class="dropdown-menu">
			<button class="dropdown-item" onclick={exportToExcel} type="button">
				<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path>
					<polyline points="14 2 14 8 20 8"></polyline>
					<line x1="16" y1="13" x2="8" y2="13"></line>
					<line x1="16" y1="17" x2="8" y2="17"></line>
					<polyline points="10 9 9 9 8 9"></polyline>
				</svg>
				{m.reports_exportExcel()}
			</button>
			<button class="dropdown-item" onclick={exportToCsv} type="button">
				<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path>
					<polyline points="14 2 14 8 20 8"></polyline>
					<line x1="16" y1="13" x2="8" y2="13"></line>
					<line x1="16" y1="17" x2="8" y2="17"></line>
				</svg>
				{m.reports_exportCsv()}
			</button>
			<button class="dropdown-item" onclick={exportToPdf} type="button">
				<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<polyline points="6 9 6 2 18 2 18 9"></polyline>
					<path d="M6 18H4a2 2 0 0 1-2-2v-5a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2v5a2 2 0 0 1-2 2h-2"></path>
					<rect x="6" y="14" width="12" height="8"></rect>
				</svg>
				{m.reports_exportPdf()}
			</button>
		</div>
	{/if}
</div>

<style>
	.export-dropdown {
		position: relative;
		display: inline-block;
	}

	.export-dropdown .btn {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.chevron {
		transition: transform 0.2s ease;
	}

	.chevron.open {
		transform: rotate(180deg);
	}

	.dropdown-menu {
		position: absolute;
		top: 100%;
		right: 0;
		margin-top: 0.25rem;
		background: var(--color-surface);
		border: 1px solid var(--color-border);
		border-radius: 0.5rem;
		box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06);
		z-index: 50;
		min-width: 180px;
		overflow: hidden;
	}

	.dropdown-item {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		width: 100%;
		padding: 0.75rem 1rem;
		border: none;
		background: transparent;
		color: var(--color-text);
		font-size: 0.875rem;
		text-align: left;
		cursor: pointer;
		transition: background-color 0.15s ease;
	}

	.dropdown-item:hover {
		background: var(--color-bg);
	}

	.dropdown-item svg {
		flex-shrink: 0;
		color: var(--color-text-muted);
	}

	@media (max-width: 768px) {
		.dropdown-menu {
			position: fixed;
			bottom: 0;
			left: 0;
			right: 0;
			top: auto;
			margin: 0;
			border-radius: 1rem 1rem 0 0;
			min-width: 100%;
		}

		.dropdown-item {
			padding: 1rem;
			min-height: 44px;
		}
	}
</style>
