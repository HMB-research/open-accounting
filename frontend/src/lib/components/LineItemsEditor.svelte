<script lang="ts">
	import * as m from '$lib/paraglide/messages.js';
	import {
		calculateLineTotal,
		calculateLinesTotal,
		createEmptyLine,
		type LineItem
	} from '$lib/utils/formatting';
	import Decimal from 'decimal.js';

	/**
	 * Reusable line items editor for invoices, quotes, and orders.
	 * Handles add/remove, VAT calculations, and discounts.
	 */

	interface Props {
		/** Line items array - bindable */
		lines: LineItem[];
		/** Available VAT rates (default: ['22', '9', '0']) */
		vatRates?: string[];
		/** Whether to show the discount column */
		showDiscount?: boolean;
		/** Format currency function */
		formatCurrency: (value: Decimal) => string;
		/** Label for line items section */
		sectionLabel?: string;
		/** Label for add button */
		addLabel?: string;
		/** Placeholder for description input */
		descriptionPlaceholder?: string;
	}

	let {
		lines = $bindable(),
		vatRates = ['22', '9', '0'],
		showDiscount = true,
		formatCurrency,
		sectionLabel,
		addLabel,
		descriptionPlaceholder
	}: Props = $props();

	// Computed total for all lines
	let total = $derived(calculateLinesTotal(lines));

	function addLine() {
		lines = [...lines, createEmptyLine(vatRates[0] || '22')];
	}

	function removeLine(index: number) {
		if (lines.length > 1) {
			lines = lines.filter((_, i) => i !== index);
		}
	}
</script>

<div class="lines-section">
	{#if sectionLabel}
		<h3>{sectionLabel}</h3>
	{:else}
		<h3>{m.invoices_lineItems()}</h3>
	{/if}

	<div class="lines-table-wrapper">
		<table class="lines-table">
			<thead>
				<tr>
					<th class="col-description">{m.common_description()}</th>
					<th class="col-qty">{m.invoices_qty()}</th>
					<th class="col-price">{m.invoices_unitPrice()}</th>
					<th class="col-vat">{m.invoices_vat()} %</th>
					{#if showDiscount}
						<th class="col-discount">{m.invoices_discount()} %</th>
					{/if}
					<th class="col-total">{m.common_total()}</th>
					<th class="col-actions"></th>
				</tr>
			</thead>
			<tbody>
				{#each lines as line, i}
					<tr>
						<td class="col-description">
							<input
								class="input"
								type="text"
								bind:value={line.description}
								required
								placeholder={descriptionPlaceholder || m.invoices_productOrService()}
							/>
						</td>
						<td class="col-qty">
							<input
								class="input input-small"
								type="number"
								step="0.01"
								min="0.01"
								bind:value={line.quantity}
								required
							/>
						</td>
						<td class="col-price">
							<input
								class="input input-small"
								type="number"
								step="0.01"
								min="0"
								bind:value={line.unit_price}
								required
							/>
						</td>
						<td class="col-vat">
							<select class="input input-small" bind:value={line.vat_rate}>
								{#each vatRates as rate}
									<option value={rate}>{rate}%</option>
								{/each}
							</select>
						</td>
						{#if showDiscount}
							<td class="col-discount">
								<input
									class="input input-small"
									type="number"
									step="0.1"
									min="0"
									max="100"
									bind:value={line.discount_percent}
								/>
							</td>
						{/if}
						<td class="col-total amount">{formatCurrency(calculateLineTotal(line))}</td>
						<td class="col-actions">
							{#if lines.length > 1}
								<button
									type="button"
									class="btn btn-small btn-danger"
									onclick={() => removeLine(i)}
								>
									&times;
								</button>
							{/if}
						</td>
					</tr>
				{/each}
			</tbody>
			<tfoot>
				<tr>
					<td colspan={showDiscount ? 5 : 4} class="total-label">
						<strong>{m.common_total()}:</strong>
					</td>
					<td class="amount">
						<strong>{formatCurrency(total)}</strong>
					</td>
					<td></td>
				</tr>
			</tfoot>
		</table>
	</div>

	<button type="button" class="btn btn-secondary" onclick={addLine}>
		+ {addLabel || m.invoices_addLine()}
	</button>
</div>

<style>
	.lines-section {
		margin: 1.5rem 0;
	}

	.lines-section h3 {
		font-size: 1rem;
		margin-bottom: 0.75rem;
	}

	.lines-table-wrapper {
		overflow-x: auto;
		-webkit-overflow-scrolling: touch;
	}

	.lines-table {
		width: 100%;
		margin-bottom: 0.75rem;
		min-width: 600px;
	}

	.lines-table th,
	.lines-table td {
		padding: 0.5rem;
	}

	.lines-table th {
		text-align: left;
		font-weight: 500;
		font-size: 0.75rem;
		color: var(--color-text-muted);
	}

	.col-description {
		min-width: 200px;
	}

	.col-qty,
	.col-vat,
	.col-discount {
		width: 80px;
	}

	.col-price,
	.col-total {
		width: 100px;
	}

	.col-actions {
		width: 50px;
	}

	.input-small {
		width: 80px;
	}

	.col-description .input {
		width: 100%;
	}

	.amount {
		font-family: var(--font-mono);
		text-align: right;
	}

	.total-label {
		text-align: right;
	}

	.btn-small {
		padding: 0.25rem 0.5rem;
		font-size: 0.75rem;
	}

	.btn-danger {
		background: #dc2626;
		color: white;
	}

	.btn-secondary {
		background: #6b7280;
		color: white;
	}

	.btn-secondary:hover {
		background: #4b5563;
	}

	/* Mobile responsive */
	@media (max-width: 768px) {
		.input-small {
			width: 70px;
		}
	}
</style>
