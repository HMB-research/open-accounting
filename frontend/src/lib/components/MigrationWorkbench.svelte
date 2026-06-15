<script lang="ts">
	import { onMount } from 'svelte';
	import {
		api,
		type BundleFile,
		type BundleValidationReport,
		type EInvoiceContactMode,
		type ExecuteMigrationRequest,
		type MigrationExecutionPlan,
		type MigrationExecutionRun,
		type MigrationExecutionRunEvent,
		type MigrationExecutionRunSummary,
		type MigrationFileKind,
		type MigrationProviderPresetInfo,
		type MigrationProviderPresetKindInfo,
		type MigrationProviderPreset,
		type PlanMigrationExecutionRequest,
		type ValidateBundleRequest
	} from '$lib/api';

	type DraftBundleFile = {
		id: string;
		kind: MigrationFileKind;
		fileName: string;
		content: string;
	};

	let { tenantId, runId = '' }: { tenantId: string; runId?: string } = $props();

	const fileKinds: Array<{ kind: MigrationFileKind; label: string }> = [
		{ kind: 'accounts', label: 'Accounts' },
		{ kind: 'contacts', label: 'Contacts' },
		{ kind: 'employees', label: 'Employees' },
		{ kind: 'invoices', label: 'Invoices' },
		{ kind: 'e_invoices', label: 'E-invoices XML' },
		{ kind: 'payments', label: 'Payments' },
		{ kind: 'bank_accounts', label: 'Bank accounts' },
		{ kind: 'bank_transactions', label: 'Bank transactions' },
		{ kind: 'payroll_history', label: 'Payroll history' },
		{ kind: 'leave_balances', label: 'Leave balances' },
		{ kind: 'tsd_history', label: 'TSD history' },
		{ kind: 'kmd_history', label: 'KMD history' },
		{ kind: 'quotes', label: 'Quotes' },
		{ kind: 'orders', label: 'Orders' },
		{ kind: 'recurring_invoices', label: 'Recurring invoices' },
		{ kind: 'cost_centers', label: 'Cost centers' },
		{ kind: 'cost_allocations', label: 'Cost allocations' },
		{ kind: 'product_categories', label: 'Product categories' },
		{ kind: 'warehouses', label: 'Warehouses' },
		{ kind: 'products', label: 'Products' },
		{ kind: 'stock_adjustments', label: 'Stock adjustments' },
		{ kind: 'fixed_assets', label: 'Fixed assets' },
		{ kind: 'opening_balances', label: 'Opening balances' },
		{ kind: 'journal_entries', label: 'Historical journals' }
	];

	const statusFilters = ['', 'needs_confirmation', 'running', 'blocked', 'failed', 'succeeded'];
	const fallbackProviderPresets: MigrationProviderPresetInfo[] = [
		{
			preset: 'generic',
			label: 'Generic',
			description: 'Uses Open Accounting canonical cutover headers without vendor-specific alias expansion.',
			file_kind_count: fileKinds.length,
			preset_alias_count: 0,
			file_kinds: []
		},
		{
			preset: 'merit',
			label: 'Merit',
			description: 'Adds Merit Aktiva and Merit Palk CSV header aliases before canonical validation.',
			file_kind_count: fileKinds.length,
			preset_alias_count: 0,
			file_kinds: []
		},
		{
			preset: 'smartaccounts',
			label: 'SmartAccounts',
			description: 'Adds SmartAccounts CSV export header aliases before canonical validation.',
			file_kind_count: fileKinds.length,
			preset_alias_count: 0,
			file_kinds: []
		},
		{
			preset: 'directo',
			label: 'Directo',
			description:
				'Adds Directo mass-import and XML Direct-style CSV header aliases before canonical validation.',
			file_kind_count: fileKinds.length,
			preset_alias_count: 0,
			file_kinds: []
		}
	];

	let nextDraftId = 1;
	let selectedKind = $state<MigrationFileKind>('accounts');
	let draftFileName = $state('');
	let draftContent = $state('');
	let providerPreset = $state<MigrationProviderPreset>('generic');
	let eInvoiceContactMode = $state<EInvoiceContactMode>('supplier');
	let bankTransactionAccountId = $state('');
	let bankTransactionFormat = $state('auto');
	let openingBalanceEntryDate = $state('');
	let eInvoiceInvoiceType = $state('');
	let resumeRunId = $state('');
	let executionConfirmed = $state(false);
	let runStatus = $state('');
	let runLimit = $state(10);
	let working = $state(false);
	let loadingHistory = $state(false);
	let loadingProviderPresets = $state(false);
	let providerPresetStatus = $state('');
	let error = $state('');
	let success = $state('');
	let bundleFiles = $state<DraftBundleFile[]>([]);
	let providerPresets = $state<MigrationProviderPresetInfo[]>(fallbackProviderPresets);
	let validation = $state<BundleValidationReport | null>(null);
	let plan = $state<MigrationExecutionPlan | null>(null);
	let run = $state<MigrationExecutionRun | null>(null);
	let savedRuns = $state<MigrationExecutionRun[]>([]);
	let selectedRun = $state<MigrationExecutionRun | null>(null);
	let streamingRunId = $state('');
	let streamStatus = $state('');
	let streamController: AbortController | null = null;
	let loadedDeepLinkKey = '';

	let canSubmitBundle = $derived(tenantId.trim().length > 0 && bundleFiles.length > 0 && !working);
	let canExecute = $derived(canSubmitBundle && executionConfirmed);
	let selectedProvider = $derived(
		providerPresets.find((preset) => preset.preset === providerPreset) ?? fallbackProviderPresets[0]
	);
	let selectedProviderAliasKinds = $derived(
		(selectedProvider.file_kinds ?? []).filter((kind) => kind.preset_alias_count > 0).slice(0, 4)
	);

	onMount(() => {
		void initializeWorkbench();
		return () => stopRunStream();
	});

	$effect(() => {
		const trimmedRunId = runId.trim();
		const trimmedTenantId = tenantId.trim();
		const deepLinkKey = `${trimmedTenantId}:${trimmedRunId}`;
		if (!trimmedTenantId || !trimmedRunId || deepLinkKey === loadedDeepLinkKey) {
			return;
		}
		loadedDeepLinkKey = deepLinkKey;
		void openSavedRunById(trimmedRunId);
	});

	function defaultFileName(kind: MigrationFileKind): string {
		const extension = kind === 'e_invoices' ? 'xml' : 'csv';
		return `${kind.replaceAll('_', '-')}.${extension}`;
	}

	function isXMLKind(kind: MigrationFileKind): boolean {
		return kind === 'e_invoices';
	}

	function toBundleFile(file: DraftBundleFile): BundleFile {
		const payload: BundleFile = {
			kind: file.kind,
			file_name: file.fileName
		};

		if (isXMLKind(file.kind)) {
			payload.xml_content = file.content;
		} else {
			payload.csv_content = file.content;
		}

		return payload;
	}

	function buildValidateRequest(): ValidateBundleRequest {
		const request: ValidateBundleRequest = {
			files: bundleFiles.map(toBundleFile),
			provider_preset: providerPreset,
			e_invoice_contact_mode: eInvoiceContactMode
		};

		if (eInvoiceInvoiceType.trim()) {
			request.e_invoice_invoice_type = eInvoiceInvoiceType.trim();
		}

		return request;
	}

	function buildPlanRequest(): PlanMigrationExecutionRequest {
		const request: PlanMigrationExecutionRequest = {
			...buildValidateRequest()
		};

		if (bankTransactionAccountId.trim()) {
			request.bank_transaction_account_id = bankTransactionAccountId.trim();
		}
		if (openingBalanceEntryDate.trim()) {
			request.opening_balance_entry_date = openingBalanceEntryDate.trim();
		}

		return request;
	}

	function buildExecuteRequest(confirm: boolean): ExecuteMigrationRequest {
		const request: ExecuteMigrationRequest = {
			...buildPlanRequest(),
			confirm
		};

		if (bankTransactionFormat.trim()) {
			request.bank_transaction_format = bankTransactionFormat.trim();
		}
		if (eInvoiceInvoiceType.trim()) {
			request.e_invoice_invoice_type = eInvoiceInvoiceType.trim();
		}
		if (resumeRunId.trim()) {
			request.resume_from_run_id = resumeRunId.trim();
		}

		return request;
	}

	function addTextFile() {
		error = '';
		success = '';
		if (!draftContent.trim()) {
			error = 'File content is required.';
			return;
		}

		bundleFiles = [
			...bundleFiles,
			{
				id: `draft-${nextDraftId++}`,
				kind: selectedKind,
				fileName: draftFileName.trim() || defaultFileName(selectedKind),
				content: draftContent
			}
		];
		draftFileName = '';
		draftContent = '';
		validation = null;
		plan = null;
		run = null;
		success = 'File added to migration bundle.';
	}

	async function addUploadedFiles(event: Event) {
		const input = event.target as HTMLInputElement;
		const files = Array.from(input.files ?? []);
		if (files.length === 0) return;

		error = '';
		success = '';
		try {
			const additions = await Promise.all(
				files.map(async (file) => ({
					id: `draft-${nextDraftId++}`,
					kind: selectedKind,
					fileName: file.name || defaultFileName(selectedKind),
					content: await file.text()
				}))
			);
			bundleFiles = [...bundleFiles, ...additions];
			validation = null;
			plan = null;
			run = null;
			success = `${additions.length} file${additions.length === 1 ? '' : 's'} added to migration bundle.`;
			input.value = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to read selected files.';
		}
	}

	function removeFile(fileId: string) {
		bundleFiles = bundleFiles.filter((file) => file.id !== fileId);
		validation = null;
		plan = null;
		run = null;
	}

	async function validateBundle() {
		if (!canSubmitBundle) return;
		working = true;
		error = '';
		success = '';
		try {
			validation = await api.validateMigrationBundle(tenantId, buildValidateRequest());
			plan = null;
			run = null;
			success = validation.summary.ready
				? 'Migration bundle validation passed.'
				: 'Migration bundle validation completed with actions.';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to validate migration bundle.';
		} finally {
			working = false;
		}
	}

	async function buildPlan() {
		if (!canSubmitBundle) return;
		working = true;
		error = '';
		success = '';
		try {
			plan = await api.planMigrationExecution(tenantId, buildPlanRequest());
			validation = plan.validation;
			run = null;
			success = plan.summary.ready
				? 'Migration execution plan is ready.'
				: 'Migration execution plan needs attention.';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to build migration execution plan.';
		} finally {
			working = false;
		}
	}

	async function savePlannedRun() {
		if (!canSubmitBundle) return;
		working = true;
		error = '';
		success = '';
		try {
			applyRunSnapshot(await api.executeMigration(tenantId, buildExecuteRequest(false)));
			success = 'Planned migration run saved.';
			await loadRunHistory();
			maybeStartRunStream(run);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to save planned migration run.';
		} finally {
			working = false;
		}
	}

	async function executeConfirmedRun() {
		if (!canExecute) return;
		working = true;
		error = '';
		success = '';
		try {
			applyRunSnapshot(await api.executeMigration(tenantId, buildExecuteRequest(true)));
			executionConfirmed = false;
			success = 'Migration execution run completed.';
			await loadRunHistory();
			maybeStartRunStream(run);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to execute migration run.';
		} finally {
			working = false;
		}
	}

	async function loadRunHistory() {
		if (!tenantId.trim()) return;
		loadingHistory = true;
		try {
			savedRuns = await api.listMigrationExecutionRuns(tenantId, {
				status: runStatus || undefined,
				limit: runLimit
			});
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load migration run history.';
		} finally {
			loadingHistory = false;
		}
	}

	async function loadProviderPresets() {
		if (!tenantId.trim()) return;
		loadingProviderPresets = true;
		providerPresetStatus = '';
		try {
			const presets = await api.listMigrationProviderPresets(tenantId);
			if (presets.length > 0) {
				providerPresets = presets;
			}
		} catch (err) {
			providerPresetStatus =
				err instanceof Error ? err.message : 'Provider preset catalog could not be loaded.';
		} finally {
			loadingProviderPresets = false;
		}
	}

	async function openSavedRun(savedRun: MigrationExecutionRun) {
		const savedRunId = savedRun.id;
		if (!savedRunId) return;

		await openSavedRunById(savedRunId);
	}

	async function initializeWorkbench() {
		await Promise.all([loadRunHistory(), loadProviderPresets()]);
		const trimmedRunId = runId.trim();
		const trimmedTenantId = tenantId.trim();
		const deepLinkKey = `${trimmedTenantId}:${trimmedRunId}`;
		if (trimmedTenantId && trimmedRunId && deepLinkKey !== loadedDeepLinkKey) {
			loadedDeepLinkKey = deepLinkKey;
			await openSavedRunById(trimmedRunId);
		}
	}

	async function openSavedRunById(savedRunId: string) {
		if (!savedRunId.trim()) return;
		working = true;
		error = '';
		success = '';
		try {
			applyRunSnapshot(await api.getMigrationExecutionRun(tenantId, savedRunId));
			loadedDeepLinkKey = `${tenantId.trim()}:${savedRunId}`;
			success = 'Saved migration run loaded.';
			maybeStartRunStream(run);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load saved migration run.';
		} finally {
			working = false;
		}
	}

	function applyRunSnapshot(nextRun: MigrationExecutionRun) {
		run = nextRun;
		selectedRun = nextRun;
		plan = nextRun.plan ?? plan;
		validation = nextRun.plan?.validation ?? validation;

		if (!nextRun.id) {
			return;
		}

		savedRuns = savedRuns.some((saved) => saved.id === nextRun.id)
			? savedRuns.map((saved) => (saved.id === nextRun.id ? nextRun : saved))
			: [nextRun, ...savedRuns];
	}

	function isTerminalRunStatus(status: string | undefined): boolean {
		switch ((status ?? '').toLowerCase()) {
			case 'succeeded':
			case 'failed':
			case 'blocked':
			case 'needs_confirmation':
				return true;
			default:
				return false;
		}
	}

	function isAbortError(err: unknown): boolean {
		return err instanceof Error && err.name === 'AbortError';
	}

	function stopRunStream(message = '') {
		if (streamController && !streamController.signal.aborted) {
			streamController.abort();
		}
		streamController = null;
		streamingRunId = '';
		streamStatus = message;
	}

	function finishRunStream(controller: AbortController, message: string) {
		if (streamController !== controller) return;
		streamController = null;
		streamingRunId = '';
		streamStatus = message;
	}

	function maybeStartRunStream(nextRun: MigrationExecutionRun | null) {
		if (!nextRun?.id) return;
		if (isTerminalRunStatus(nextRun.summary.status)) {
			if (streamingRunId === nextRun.id) {
				stopRunStream('Migration run stream completed.');
			}
			return;
		}
		startRunStream(nextRun.id);
	}

	function handleRunStreamEvent(event: MigrationExecutionRunEvent, controller: AbortController) {
		if (event.type === 'error') {
			throw new Error('Migration run stream returned an error event.');
		}
		if (event.run) {
			applyRunSnapshot(event.run);
		}
		if (event.type === 'complete' || isTerminalRunStatus(event.run?.summary.status)) {
			finishRunStream(controller, 'Migration run stream completed.');
		}
	}

	function startRunStream(savedRunId: string | undefined) {
		const trimmedRunId = savedRunId?.trim() ?? '';
		if (!tenantId.trim() || !trimmedRunId) return;
		if (streamingRunId === trimmedRunId && streamController && !streamController.signal.aborted) {
			return;
		}

		stopRunStream();
		const controller = new AbortController();
		let streamReachedTerminal = false;
		streamController = controller;
		streamingRunId = trimmedRunId;
		streamStatus = 'Streaming saved run telemetry.';

		void api
			.watchMigrationExecutionRun(tenantId, trimmedRunId, {
				intervalMs: 1000,
				maxEvents: 1000,
				signal: controller.signal,
				onEvent: (event) => {
					streamReachedTerminal =
						streamReachedTerminal ||
						event.type === 'complete' ||
						isTerminalRunStatus(event.run?.summary.status);
					handleRunStreamEvent(event, controller);
				}
			})
			.then(() => {
				finishRunStream(
					controller,
					streamReachedTerminal
						? 'Migration run stream completed.'
						: 'Migration run stream ended after reaching the event limit.'
				);
			})
			.catch((err) => {
				if (isAbortError(err)) return;
				if (streamController === controller) {
					streamController = null;
					streamingRunId = '';
				}
				error = err instanceof Error ? err.message : 'Migration run stream stopped.';
				streamStatus = 'Migration run stream stopped.';
			});
	}

	function setResumeRun(runId: string | undefined) {
		resumeRunId = runId ?? '';
		success = resumeRunId ? 'Resume run selected.' : '';
	}

	function fileKindLabel(kind: MigrationFileKind): string {
		return fileKinds.find((option) => option.kind === kind)?.label ?? kind.replaceAll('_', ' ');
	}

	function formatRequiredGroups(groups: string[][] | undefined): string {
		if (!groups?.length) return '-';
		return groups.map((group) => group.join(' or ')).join(', ');
	}

	function formatAliasSamples(kind: MigrationProviderPresetKindInfo): string {
		if (!kind.sample_aliases?.length) return '-';
		return kind.sample_aliases
			.map((sample) => `${sample.source_header} -> ${sample.canonical_header}`)
			.join(', ');
	}

	function statusClass(status: string | undefined): string {
		const normalized = (status ?? '').toLowerCase();
		if (normalized.includes('success')) return 'status-success';
		if (normalized.includes('fail') || normalized.includes('block')) return 'status-error';
		if (normalized.includes('running')) return 'status-running';
		if (normalized.includes('plan')) return 'status-planned';
		return 'status-neutral';
	}

	function formatDateTime(value: string | undefined): string {
		if (!value) return '-';
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return value;
		return date.toLocaleString();
	}

	function progressPercent(summary: MigrationExecutionRunSummary | undefined): number {
		const value = summary?.progress_percent ?? 0;
		return Math.max(0, Math.min(100, value));
	}

	function activeStepLabel(summary: MigrationExecutionRunSummary | undefined): string {
		if (!summary?.active_step_number) return '-';
		const parts = [`#${summary.active_step_number}`];
		if (summary.active_step_status) parts.push(summary.active_step_status);
		if (summary.active_step_kind) parts.push(summary.active_step_kind);
		if (summary.active_step_file_name) parts.push(summary.active_step_file_name);
		return parts.join(' ');
	}

	function formatDurationMs(value: number | undefined): string {
		if (!value || value <= 0) return '-';
		if (value < 1000) return `${value}ms`;
		if (value < 60000) {
			const seconds = value / 1000;
			return `${Number.isInteger(seconds) ? seconds.toFixed(0) : seconds.toFixed(1)}s`;
		}
		const minutes = Math.floor(value / 60000);
		const seconds = Math.round((value % 60000) / 1000);
		return seconds > 0 ? `${minutes}m ${seconds}s` : `${minutes}m`;
	}
</script>

{#if !tenantId.trim()}
	<div class="container">
		<div class="alert alert-error">Select a tenant before opening migration cutover controls.</div>
	</div>
{:else}
	<div class="migration-workbench container">
		<header class="page-header">
			<div>
				<p class="eyebrow">Historical cutover</p>
				<h1>Migration Workbench</h1>
			</div>
			<div class="header-stats" aria-label="Migration bundle summary">
				<div>
					<strong>{bundleFiles.length}</strong>
					<span>files</span>
				</div>
				<div>
					<strong>{plan?.summary.ready_step_count ?? 0}</strong>
					<span>ready</span>
				</div>
				<div>
					<strong>{run?.summary.succeeded_step_count ?? 0}</strong>
					<span>succeeded</span>
				</div>
			</div>
		</header>

		{#if error}
			<div class="alert alert-error" role="alert">{error}</div>
		{/if}
		{#if success}
			<div class="alert alert-success" aria-live="polite">{success}</div>
		{/if}

		<div class="workbench-grid">
			<section class="card panel">
				<div class="panel-heading">
					<h2>Bundle files</h2>
					<span>{providerPreset}</span>
				</div>

				<div class="form-grid">
					<div class="form-group">
						<label class="label" for="migration-provider">Provider preset</label>
						<select id="migration-provider" class="input" bind:value={providerPreset}>
							{#each providerPresets as preset (preset.preset)}
								<option value={preset.preset}>{preset.label}</option>
							{/each}
						</select>
					</div>
					<div class="form-group">
						<label class="label" for="e-invoice-contact-mode">E-invoice contacts</label>
						<select id="e-invoice-contact-mode" class="input" bind:value={eInvoiceContactMode}>
							<option value="supplier">Supplier</option>
							<option value="customer">Customer</option>
							<option value="both">Both parties</option>
						</select>
					</div>
					<div class="form-group">
						<label class="label" for="migration-file-kind">File kind</label>
						<select id="migration-file-kind" class="input" bind:value={selectedKind}>
							{#each fileKinds as option (option.kind)}
								<option value={option.kind}>{option.label}</option>
							{/each}
						</select>
					</div>
					<div class="form-group">
						<label class="label" for="migration-file-name">File name</label>
						<input
							id="migration-file-name"
							class="input"
							bind:value={draftFileName}
							placeholder={defaultFileName(selectedKind)}
						/>
					</div>
				</div>

				<div class="preset-summary" aria-label="Provider preset metadata">
					<div class="preset-summary-header">
						<div>
							<strong>{selectedProvider.label}</strong>
							<span>
								{selectedProvider.file_kind_count} file kinds · {selectedProvider.preset_alias_count}
								aliases
							</span>
						</div>
						<span class="status-neutral">{loadingProviderPresets ? 'Loading' : 'Catalog'}</span>
					</div>
					<p>{providerPresetStatus || selectedProvider.description}</p>
					{#if selectedProviderAliasKinds.length > 0}
						<div class="preset-kind-grid">
							{#each selectedProviderAliasKinds as kind (kind.kind)}
								<div>
									<strong>{fileKindLabel(kind.kind)}</strong>
									<span>{kind.preset_alias_count} aliases</span>
									<small>{formatRequiredGroups(kind.required_column_groups)}</small>
									<code>{formatAliasSamples(kind)}</code>
								</div>
							{/each}
						</div>
					{/if}
				</div>

				<div class="form-group">
					<label class="label" for="migration-file-content">CSV or XML content</label>
					<textarea
						id="migration-file-content"
						class="input content-input"
						bind:value={draftContent}
						spellcheck="false"
					></textarea>
				</div>

				<div class="bundle-actions">
					<button class="btn btn-secondary" type="button" onclick={addTextFile}>Add text file</button>
					<label class="upload-button">
						<span>Upload files</span>
						<input type="file" accept=".csv,.txt,.xml" multiple onchange={addUploadedFiles} />
					</label>
				</div>

				{#if bundleFiles.length > 0}
					<div class="file-list" aria-label="Selected migration files">
						{#each bundleFiles as file (file.id)}
							<div class="file-row">
								<div>
									<strong>{file.fileName}</strong>
									<span>{file.kind} · {file.content.split(/\r?\n/).filter(Boolean).length} rows</span>
								</div>
								<button type="button" class="link-button" onclick={() => removeFile(file.id)}>Remove</button>
							</div>
						{/each}
					</div>
				{/if}
			</section>

			<section class="card panel">
				<div class="panel-heading">
					<h2>Execution controls</h2>
					<span>{working ? 'Working' : 'Ready'}</span>
				</div>

				<div class="form-grid single">
					<div class="form-group">
						<label class="label" for="bank-transaction-account-id">Bank transaction account ID</label>
						<input id="bank-transaction-account-id" class="input" bind:value={bankTransactionAccountId} />
					</div>
					<div class="form-group">
						<label class="label" for="bank-transaction-format">Bank transaction format</label>
						<select id="bank-transaction-format" class="input" bind:value={bankTransactionFormat}>
							<option value="auto">Auto</option>
							<option value="generic">Generic CSV</option>
							<option value="lhv">LHV CSV</option>
							<option value="camt053">camt.053</option>
							<option value="lhv-camt">LHV camt.053</option>
						</select>
					</div>
					<div class="form-group">
						<label class="label" for="opening-balance-entry-date">Opening balance entry date</label>
						<input
							id="opening-balance-entry-date"
							class="input"
							type="date"
							bind:value={openingBalanceEntryDate}
						/>
					</div>
					<div class="form-group">
						<label class="label" for="e-invoice-invoice-type">E-invoice invoice type</label>
						<input id="e-invoice-invoice-type" class="input" bind:value={eInvoiceInvoiceType} />
					</div>
					<div class="form-group">
						<label class="label" for="resume-run-id">Resume run ID</label>
						<input id="resume-run-id" class="input" bind:value={resumeRunId} />
					</div>
				</div>

				<div class="action-stack">
					<button class="btn btn-secondary" type="button" disabled={!canSubmitBundle} onclick={validateBundle}>
						Validate
					</button>
					<button class="btn btn-secondary" type="button" disabled={!canSubmitBundle} onclick={buildPlan}>
						Build plan
					</button>
					<button class="btn btn-secondary" type="button" disabled={!canSubmitBundle} onclick={savePlannedRun}>
						Save dry run
					</button>
					<label class="confirm-line">
						<input type="checkbox" bind:checked={executionConfirmed} />
						<span>Confirm execution</span>
					</label>
					<button class="btn btn-primary" type="button" disabled={!canExecute} onclick={executeConfirmedRun}>
						Execute confirmed cutover
					</button>
				</div>
			</section>
		</div>

		{#if validation}
			<section class="card result-section">
				<div class="panel-heading">
					<h2>Validation</h2>
					<span class={statusClass(validation.summary.ready ? 'success' : 'blocked')}>
						{validation.summary.ready ? 'Ready' : 'Actions'}
					</span>
				</div>
				<div class="metric-grid">
					<div><strong>{validation.summary.files_validated}</strong><span>files</span></div>
					<div><strong>{validation.summary.rows_validated}</strong><span>rows</span></div>
					<div><strong>{validation.summary.error_count}</strong><span>errors</span></div>
					<div><strong>{validation.summary.warning_count}</strong><span>warnings</span></div>
				</div>

				{#if validation.remediation_actions?.length}
					<div class="table-wrap">
						<table class="table compact-table">
							<thead>
								<tr>
									<th>Code</th>
									<th>Priority</th>
									<th>Message</th>
									<th>Command</th>
								</tr>
							</thead>
							<tbody>
								{#each validation.remediation_actions as action, index (action.assignment_key ?? `${action.code}-${index}`)}
									<tr>
										<td>{action.code}</td>
										<td>{action.priority ?? '-'}</td>
										<td>{action.message}</td>
										<td><code>{action.cli_command ?? '-'}</code></td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{/if}
			</section>
		{/if}

		{#if plan}
			<section class="card result-section">
				<div class="panel-heading">
					<h2>Execution plan</h2>
					<span class={statusClass(plan.summary.ready ? 'success' : 'blocked')}>
						{plan.summary.ready ? 'Ready' : 'Blocked'}
					</span>
				</div>
				<div class="metric-grid">
					<div><strong>{plan.summary.step_count}</strong><span>steps</span></div>
					<div><strong>{plan.summary.ready_step_count}</strong><span>ready</span></div>
					<div><strong>{plan.summary.needs_context_count}</strong><span>needs context</span></div>
					<div><strong>{plan.summary.blocked_step_count}</strong><span>blocked</span></div>
				</div>

				<div class="table-wrap">
					<table class="table compact-table">
						<thead>
							<tr>
								<th>#</th>
								<th>Kind</th>
								<th>Status</th>
								<th>File</th>
								<th>Command</th>
							</tr>
						</thead>
						<tbody>
							{#each plan.steps ?? [] as step (step.step_number)}
								<tr>
									<td>{step.step_number}</td>
									<td>{step.kind}</td>
									<td><span class={statusClass(step.status)}>{step.status}</span></td>
									<td>{step.file_name}</td>
									<td><code>{step.cli_command ?? '-'}</code></td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			</section>
		{/if}

		{#if run}
			<section class="card result-section">
				<div class="panel-heading">
					<h2>Execution run</h2>
					<span class={statusClass(run.summary.status)}>{run.summary.status}</span>
				</div>
				<div class="metric-grid">
					<div><strong>{run.summary.step_count}</strong><span>steps</span></div>
					<div><strong>{run.summary.progress_percent ?? 0}%</strong><span>progress</span></div>
					<div><strong>{run.summary.succeeded_step_count}</strong><span>succeeded</span></div>
					<div><strong>{run.summary.failed_step_count}</strong><span>failed</span></div>
					<div><strong>{run.summary.resumed_step_count}</strong><span>resumed</span></div>
					<div><strong>{formatDurationMs(run.summary.duration_ms)}</strong><span>duration</span></div>
				</div>
				<div class="progress-summary">
					<div class="progress-line">
						<strong>{run.summary.completed_step_count ?? run.summary.succeeded_step_count}</strong>
						<span>completed, {run.summary.remaining_step_count ?? 0} remaining</span>
					</div>
					<div class="progress-track" aria-label="Migration execution progress">
						<span style={`width: ${progressPercent(run.summary)}%;`}></span>
					</div>
					<p class="muted">Active step: {activeStepLabel(run.summary)}</p>
				</div>

				{#if run.id}
					<div class="stream-controls">
						<div>
							<strong>Live telemetry</strong>
							<span aria-live="polite">
								{streamStatus ||
									(isTerminalRunStatus(run.summary.status) ? 'Run is terminal.' : 'Stream idle.')}
							</span>
						</div>
						<div class="row-actions">
							<button
								type="button"
								class="link-button"
								disabled={streamingRunId === run?.id || isTerminalRunStatus(run?.summary.status)}
								onclick={() => startRunStream(run?.id)}
							>
								Stream live
							</button>
							{#if streamingRunId === run?.id}
								<button
									type="button"
									class="link-button"
									onclick={() => stopRunStream('Migration run stream stopped.')}
								>
									Stop stream
								</button>
							{/if}
						</div>
					</div>
				{/if}

				<div class="table-wrap">
					<table class="table compact-table">
						<thead>
							<tr>
								<th>#</th>
								<th>Kind</th>
								<th>Status</th>
								<th>File</th>
								<th>Duration</th>
								<th>Message</th>
							</tr>
						</thead>
						<tbody>
							{#each run.steps ?? [] as step (step.step_number)}
								<tr>
									<td>{step.step_number}</td>
									<td>{step.kind}</td>
									<td><span class={statusClass(step.status)}>{step.status}</span></td>
									<td>{step.file_name}</td>
									<td>{formatDurationMs(step.duration_ms)}</td>
									<td>{step.error || step.message || '-'}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			</section>
		{/if}

		<section class="card result-section">
			<div class="panel-heading">
				<h2>Saved runs</h2>
				<span>{loadingHistory ? 'Loading' : `${savedRuns.length} loaded`}</span>
			</div>

			<div class="history-toolbar">
				<div class="form-group">
					<label class="label" for="run-status-filter">Status</label>
					<select id="run-status-filter" class="input" bind:value={runStatus}>
						{#each statusFilters as status (status)}
							<option value={status}>{status || 'all'}</option>
						{/each}
					</select>
				</div>
				<div class="form-group limit-field">
					<label class="label" for="run-limit">Limit</label>
					<input id="run-limit" class="input" type="number" min="1" max="200" bind:value={runLimit} />
				</div>
				<button class="btn btn-secondary" type="button" onclick={loadRunHistory} disabled={loadingHistory}>
					Refresh
				</button>
			</div>

			{#if savedRuns.length === 0}
				<p class="muted">No saved migration runs.</p>
			{:else}
				<div class="table-wrap">
					<table class="table compact-table">
						<thead>
							<tr>
								<th>Run</th>
								<th>Status</th>
								<th>Updated</th>
								<th>Progress</th>
								<th>Duration</th>
								<th>Active</th>
								<th>Steps</th>
								<th>Actions</th>
							</tr>
						</thead>
						<tbody>
							{#each savedRuns as saved (saved.id ?? saved.created_at ?? saved.summary.status)}
								<tr class:selected-row={selectedRun?.id === saved.id}>
									<td><code>{saved.id ?? '-'}</code></td>
									<td><span class={statusClass(saved.summary.status)}>{saved.summary.status}</span></td>
									<td>{formatDateTime(saved.updated_at ?? saved.created_at)}</td>
									<td>{saved.summary.progress_percent ?? 0}%</td>
									<td>{formatDurationMs(saved.summary.duration_ms)}</td>
									<td>{activeStepLabel(saved.summary)}</td>
									<td>{saved.summary.succeeded_step_count}/{saved.summary.step_count}</td>
									<td>
										<div class="row-actions">
											<button type="button" class="link-button" onclick={() => openSavedRun(saved)}>Open</button>
											<button
												type="button"
												class="link-button"
												disabled={streamingRunId === saved.id || isTerminalRunStatus(saved.summary.status)}
												onclick={() => openSavedRun(saved)}
											>
												Stream
											</button>
											<button type="button" class="link-button" onclick={() => setResumeRun(saved.id)}>Resume</button>
										</div>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</section>
	</div>
{/if}

<style>
	.migration-workbench {
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}

	.page-header,
	.panel-heading,
	.file-row,
	.history-toolbar,
	.row-actions,
	.bundle-actions,
	.confirm-line {
		display: flex;
		align-items: center;
		gap: 1rem;
	}

	.page-header {
		justify-content: space-between;
	}

	h1 {
		margin: 0;
		font-size: 1.9rem;
	}

	h2 {
		margin: 0;
		font-size: 1.1rem;
	}

	.eyebrow {
		color: var(--color-text-muted);
		font-size: 0.75rem;
		font-weight: 700;
		letter-spacing: 0.08em;
		margin: 0 0 0.25rem;
		text-transform: uppercase;
	}

	.header-stats,
	.metric-grid {
		display: grid;
		gap: 0.75rem;
	}

	.header-stats {
		grid-template-columns: repeat(3, minmax(5rem, 1fr));
	}

	.header-stats div,
	.metric-grid div {
		background: rgba(255, 255, 255, 0.66);
		border: 1px solid var(--color-border);
		border-radius: 8px;
		padding: 0.7rem 0.85rem;
	}

	.header-stats strong,
	.metric-grid strong {
		display: block;
		font-size: 1.2rem;
	}

	.header-stats span,
	.metric-grid span {
		color: var(--color-text-muted);
		font-size: 0.78rem;
	}

	.workbench-grid {
		display: grid;
		grid-template-columns: minmax(0, 1.5fr) minmax(20rem, 0.9fr);
		gap: 1rem;
		align-items: start;
	}

	.panel,
	.result-section {
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}

	.panel-heading {
		justify-content: space-between;
	}

	.panel-heading > span {
		color: var(--color-text-muted);
		font-size: 0.82rem;
		font-weight: 700;
		text-transform: uppercase;
	}

	.form-grid {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 1rem;
	}

	.form-grid.single {
		grid-template-columns: 1fr;
		gap: 0.75rem;
	}

	.content-input {
		min-height: 11rem;
		resize: vertical;
		font-family: var(--font-mono);
	}

	.bundle-actions {
		justify-content: flex-start;
		flex-wrap: wrap;
	}

	.upload-button {
		display: inline-flex;
		align-items: center;
		border: 1px solid var(--color-border);
		border-radius: 999px;
		background: rgba(255, 255, 255, 0.65);
		color: var(--color-text);
		cursor: pointer;
		font-weight: 600;
		padding: 0.65rem 1rem;
	}

	.upload-button input {
		display: none;
	}

	.file-list {
		border-top: 1px solid var(--color-border);
		display: flex;
		flex-direction: column;
	}

	.file-row {
		justify-content: space-between;
		border-bottom: 1px solid var(--color-border);
		padding: 0.75rem 0;
	}

	.file-row span {
		color: var(--color-text-muted);
		display: block;
		font-size: 0.8rem;
	}

	.preset-summary {
		border: 1px solid var(--color-border);
		border-radius: 8px;
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
		padding: 0.85rem;
	}

	.preset-summary-header {
		align-items: flex-start;
		display: flex;
		gap: 1rem;
		justify-content: space-between;
	}

	.preset-summary-header span,
	.preset-summary p,
	.preset-kind-grid span,
	.preset-kind-grid small {
		color: var(--color-text-muted);
	}

	.preset-summary-header span,
	.preset-kind-grid span,
	.preset-kind-grid small {
		display: block;
		font-size: 0.8rem;
	}

	.preset-summary p {
		font-size: 0.88rem;
		margin: 0;
	}

	.preset-kind-grid {
		display: grid;
		gap: 0.65rem;
		grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
	}

	.preset-kind-grid div {
		border-top: 1px solid var(--color-border);
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
		min-width: 0;
		padding-top: 0.65rem;
	}

	.preset-kind-grid code {
		font-size: 0.76rem;
		white-space: normal;
		word-break: break-word;
	}

	.action-stack {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}

	.confirm-line {
		color: var(--color-text-muted);
		font-size: 0.9rem;
	}

	.metric-grid {
		grid-template-columns: repeat(auto-fit, minmax(7rem, 1fr));
	}

	.progress-summary {
		border: 1px solid var(--color-border);
		border-radius: 8px;
		display: flex;
		flex-direction: column;
		gap: 0.55rem;
		padding: 0.85rem;
	}

	.stream-controls {
		align-items: center;
		border: 1px solid var(--color-border);
		border-radius: 8px;
		display: flex;
		gap: 1rem;
		justify-content: space-between;
		padding: 0.8rem 0.85rem;
	}

	.stream-controls span {
		color: var(--color-text-muted);
		display: block;
		font-size: 0.82rem;
		margin-top: 0.2rem;
	}

	.progress-line {
		align-items: baseline;
		display: flex;
		gap: 0.5rem;
	}

	.progress-line span {
		color: var(--color-text-muted);
		font-size: 0.82rem;
	}

	.progress-track {
		background: rgba(100, 116, 139, 0.16);
		border-radius: 999px;
		height: 0.55rem;
		overflow: hidden;
	}

	.progress-track span {
		background: var(--color-primary);
		border-radius: inherit;
		display: block;
		height: 100%;
	}

	.table-wrap {
		overflow-x: auto;
	}

	.compact-table {
		min-width: 760px;
		font-size: 0.86rem;
	}

	.compact-table code {
		white-space: normal;
		word-break: break-word;
	}

	.link-button {
		background: none;
		border: 0;
		color: var(--color-primary);
		font-weight: 700;
		padding: 0;
	}

	.link-button:hover {
		text-decoration: underline;
	}

	.link-button:disabled {
		color: var(--color-text-muted);
		cursor: not-allowed;
		text-decoration: none;
	}

	.status-success,
	.status-error,
	.status-running,
	.status-planned,
	.status-neutral {
		border-radius: 999px;
		display: inline-flex;
		font-size: 0.72rem;
		font-weight: 800;
		padding: 0.2rem 0.5rem;
		text-transform: uppercase;
	}

	.status-success {
		background: rgba(34, 197, 94, 0.14);
		color: #15803d;
	}

	.status-error {
		background: rgba(239, 68, 68, 0.13);
		color: #b91c1c;
	}

	.status-running {
		background: rgba(37, 99, 235, 0.13);
		color: #1d4ed8;
	}

	.status-planned {
		background: rgba(245, 158, 11, 0.16);
		color: #a16207;
	}

	.status-neutral {
		background: rgba(100, 116, 139, 0.13);
		color: #475569;
	}

	.history-toolbar {
		flex-wrap: wrap;
	}

	.history-toolbar .form-group {
		margin-bottom: 0;
		min-width: 12rem;
	}

	.history-toolbar .limit-field {
		min-width: 7rem;
		max-width: 8rem;
	}

	.selected-row td {
		background: rgba(37, 99, 235, 0.07);
	}

	.muted {
		color: var(--color-text-muted);
	}

	.alert {
		border-radius: 8px;
		padding: 0.85rem 1rem;
	}

	.alert-error {
		background: rgba(239, 68, 68, 0.1);
		color: #991b1b;
	}

	.alert-success {
		background: rgba(34, 197, 94, 0.1);
		color: #166534;
	}

	@media (max-width: 900px) {
		.page-header {
			align-items: flex-start;
			flex-direction: column;
		}

		.header-stats,
		.workbench-grid,
		.metric-grid,
		.form-grid {
			grid-template-columns: 1fr;
			width: 100%;
		}
	}
</style>
