<script lang="ts">
	import { onMount } from 'svelte';
	import {
		api,
		getApiBase,
		type SmartAccountsBrowserDiscoveryIssue,
		type SmartAccountsBrowserDiscoveryReceipt,
		type SmartAccountsBrowserDiscoveryRelayResult,
		type SmartAccountsBrowserCSVSchemaApprovalResponse,
		type SmartAccountsBrowserCaptureIssue,
		type SmartAccountsBrowserCaptureStatus,
		type SmartAccountsBrowserCapturePlan,
		type SmartAccountsBrowserCaptureWorkflowStatus,
		type SmartAccountsBrowserMasterDetailIssue,
		type SmartAccountsBrowserMasterDetailStatus,
		type SmartAccountsBrowserOnboardingBatchResponse,
		type SmartAccountsBrowserOnboardingCatalogIssue,
		type SmartAccountsBrowserOnboardingResult,
		type SmartAccountsBrowserBatchWorkflowSource,
		type SmartAccountsBrowserBatchWorkflowStatus,
		type SmartAccountsArchiveCoverageReport,
		type SmartAccountsPackagePreview,
		type SmartAccountsReferencePreview,
		type SmartAccountsSyncStatus
	} from '$lib/api';
	import SmartAccountsAccountantReconciliationReview from '$lib/components/SmartAccountsAccountantReconciliationReview.svelte';
	import SmartAccountsBatchRunner from '$lib/components/SmartAccountsBatchRunner.svelte';

	let {
		tenantId,
		accountantReviewBatchId = '',
		accountantReviewSourceCompanyId = '',
		ownerContinuationBatchId = '',
		ownerContinuationSourceCompanyId = ''
	}: {
		tenantId: string;
		accountantReviewBatchId?: string;
		accountantReviewSourceCompanyId?: string;
		ownerContinuationBatchId?: string;
		ownerContinuationSourceCompanyId?: string;
	} = $props();
	type BrowserSourceCompany = { source_company_id: string; source_company_name: string };
	type BrowserOnboardingBinding = {
		sourceCompanyID: string;
		sourceCompanyName: string;
		tenantID?: string;
		tenantName?: string;
		pairingID?: string;
		status: 'waiting' | 'claimed' | 'capture_issued' | 'review_required' | 'failed';
		reasonCode?: string;
		tenantCreated?: boolean;
		tenantReused?: boolean;
		workflowID?: string;
	};
	type BatchDiscoveryAction = {
		batchID: string;
		sourceCompanyID: string;
		leaseID: string;
		phaseGeneration: number;
		discoveryID: string;
	};
	type BatchCaptureAction = {
		batchID: string;
		sourceCompanyID: string;
		leaseID: string;
		phaseGeneration: number;
		runID: string;
	};
	type RelayReadiness = 'checking' | 'ready' | 'missing' | 'stale' | 'signed_out' | 'unknown';

	const relayReadinessProtocolVersion = 'smartaccounts-browser-relay-v1';
	const relayCaptureManifestVersion = 'smartaccounts-brave-ui-v2';
	const relayWorkflowPlanVersion = 'smartaccounts-browser-capture-plan-v1';
	const relayReadinessTimeoutMs = 3_000;
	const browserOnboardingMaxSources = 250;
	const onboardingBatchAutoRefreshDelayMs = 1_000;
	const onboardingBatchAutoRefreshMaxAttempts = 6;
	const relayReadinessResponseFields = [
		'source', 'type', 'version', 'nonce', 'relay_protocol_version',
		'capture_manifest_version', 'workflow_plan_version', 'smartaccounts_session_state'
	];

	let status = $state<SmartAccountsSyncStatus | null>(null);
	let apiKey = $state('');
	let apiSecret = $state('');
	let saving = $state(false);
	let pairingPending = $state(false);
	let pairingID = $state('');
	let pairingMessage = $state('');
	let browserDiscoveryPending = $state(false);
	let browserDiscoveryMessage = $state('');
	let browserDiscoveryID = $state('');
	let browserDiscoveryReceipt = $state<SmartAccountsBrowserDiscoveryReceipt | null>(null);
	let browserDiscoveryMetadataConsent = $state(false);
	let browserDiscoveryHeaderProbeConsent = $state(false);
	let browserCSVSchemaReviewPending = $state(false);
	let browserCSVSchemaReviewConfirmed = $state(false);
	let browserCSVSchemaReview = $state<SmartAccountsBrowserCSVSchemaApprovalResponse | null>(null);
	let browserCapturePending = $state(false);
	let browserCaptureMessage = $state('');
	let browserCaptureStatus = $state<SmartAccountsBrowserCaptureStatus | null>(null);
	let browserCaptureWorkflow = $state<SmartAccountsBrowserCaptureWorkflowStatus | null>(null);
	let browserCaptureFrom = $state('');
	let browserCaptureConsent = $state(false);
	let browserCaptureRunID = $state('');
	let browserCaptureResumeConsent = $state(false);
	let masterDetailTransferConsent = $state(false);
	let masterDetailPending = $state(false);
	let masterDetailBatchID = $state('');
	let masterDetailStatuses = $state<SmartAccountsBrowserMasterDetailStatus[]>([]);
	let masterDetailResumeConsent = $state<Record<string, boolean>>({});
	let masterDetailMessage = $state('');
	let startingCapture = $state(false);
	let error = $state('');
	// `null` distinguishes the initial pre-tenant mount from an intentional
	// empty tenant ID, so durable selected/all onboarding can restore once.
	let loadedTenantID: string | null = null;
	let preview = $state<SmartAccountsPackagePreview | null>(null);
	let previewPackageID = $state('');
	let ownerContinuationSource = $state<SmartAccountsBrowserBatchWorkflowSource | null>(null);
	let ownerContinuationCoverage = $state<SmartAccountsArchiveCoverageReport | null>(null);
	let ownerContinuationLoading = $state(false);
	let ownerContinuationPreviewLoading = $state(false);
	let ownerContinuationMessage = $state('');
	let ownerContinuationCoverageMessage = $state('');
	let loadedOwnerContinuationKey = '';
	let referencePreview = $state<SmartAccountsReferencePreview | null>(null);
	let referencePreviewPackageID = $state('');
	let browserPreviewAutoRequestedKey = $state('');
	let preparingPreview = $state(false);
	let applyingPreview = $state(false);
	let applyConfirmed = $state(false);
	let preparingReferencePreview = $state(false);
	let applyingReferencePreview = $state(false);
	let referenceApplyConfirmed = $state(false);
	let dateWindowFrom = $state('');
	let dateWindowTo = $state('');
	let sourceDiscoveryPending = $state(false);
	let discoveredSourceCompanies = $state<BrowserSourceCompany[]>([]);
	let selectedSourceCompanyIDs = $state<string[]>([]);
	let onboardingCatalogConsent = $state(false);
	let onboardingCatalogReceiptID = $state('');
	let onboardingCatalogWorkflowID = '';
	let onboardingCatalogNonce = '';
	let onboardingCatalogGeneration = 0;
	let onboardingMode = $state<'' | 'selected' | 'all'>('');
	let onboardingBatchConsent = $state(false);
	let onboardingBatchID = $state('');
	let onboardingBatchStatus = $state<'' | 'PENDING' | 'REVIEW_REQUIRED' | 'READY' | 'COMPLETE'>('');
	let onboardingBatchWorkflow = $state<SmartAccountsBrowserBatchWorkflowStatus | null>(null);
	let onboardingResumeConfirmed = $state(false);
	let onboardingBusy = $state(false);
	let onboardingMessage = $state('');
	let onboardingBindings = $state<BrowserOnboardingBinding[]>([]);
	let onboardingBatchRefreshTimer: number | undefined;
	let onboardingBatchRefreshAttempts = 0;
	// These maps hold only short-lived control identifiers needed to relay an
	// exact action completion. They are never rendered, persisted, or populated
	// with source rows, cookie/session state, or capture capabilities.
	const batchDiscoveryActions = new Map<string, BatchDiscoveryAction>();
	const batchCaptureActions = new Map<string, BatchCaptureAction>();
	const batchCapturePollTimers = new Map<string, number>();
	let relayReadiness = $state<RelayReadiness>('checking');
	let relayReadinessNonce = '';
	let relayReadinessTimer: number | undefined;
	let relayListenerMounted = false;
	let hasTenant = $derived(tenantId.trim().length > 0);
	let accountantReviewRequested = $derived(Boolean(accountantReviewBatchId || accountantReviewSourceCompanyId));
	let ownerContinuationRequested = $derived(Boolean(ownerContinuationBatchId || ownerContinuationSourceCompanyId));

	let captureRuns = $derived(
		status?.capture_progresses?.length
			? status.capture_progresses
			: status?.capture_progress
				? [status.capture_progress]
				: []
	);
	let dateWindowResources = $derived.by(() => {
		const required = new Set<string>();
		for (const fullRun of captureRuns) {
			if (fullRun.scope_mode !== 'full_history') continue;
			for (const resource of fullRun.resources) {
				if (resource.status !== 'review_required' || resource.reason_code !== 'source_date_window_required') continue;
				const covered = captureRuns.some((windowRun) =>
					windowRun.scope_mode === 'window' &&
					Boolean(windowRun.date_from && windowRun.date_to && fullRun.source_as_of_date) &&
					windowRun.date_from! <= windowRun.date_to! &&
					windowRun.date_to! >= fullRun.source_as_of_date! &&
					windowRun.resources.some((candidate) => candidate.resource_id === resource.resource_id && candidate.status === 'completed')
				);
				if (!covered) required.add(resource.resource_id);
			}
		}
		return [...required].sort();
	});
	let requiredWindowEnd = $derived(
		captureRuns
			.filter((run) => run.scope_mode === 'full_history' && run.source_as_of_date)
			.map((run) => run.source_as_of_date!)
			.sort()
			.at(-1) ?? ''
	);
	let braveDiscoveryResources = $derived(
		[...new Set(captureRuns.flatMap((run) => run.resources.filter((resource) => resource.status === 'brave_discovery_required').map((resource) => resource.resource_id)))].sort()
	);
	let coverageBlocked = $derived(dateWindowResources.length > 0 || braveDiscoveryResources.length > 0);

	let canConfigure = $derived(Boolean(apiKey.trim() && apiSecret.trim() && !saving));
	let browserRelayReady = $derived(relayReadiness === 'ready');
	let relayReadinessMessage = $derived.by(() => {
		switch (relayReadiness) {
			case 'ready':
				return 'Relay ready — SmartAccounts is signed in.';
			case 'signed_out':
				return 'Relay is ready, but SmartAccounts is signed out. Sign in in the existing Brave tab, then check again.';
			case 'unknown':
				return 'Relay is reachable, but SmartAccounts sign-in could not be confirmed. Reload the signed-in SmartAccounts tab, then check again.';
			case 'stale':
				return 'Relay needs reload or update. Reload the Open Accounting page after updating the Browser Relay, then check again.';
			case 'missing':
				return 'Relay was not detected. Enable or reload the Browser Relay, then check again.';
			default:
				return 'Checking the installed Brave relay…';
		}
	});

	$effect(() => {
		const currentTenantID = tenantId.trim();
		if (currentTenantID === loadedTenantID) return;
		loadedTenantID = currentTenantID;
		status = null;
		apiKey = '';
		apiSecret = '';
		pairingPending = false;
		pairingID = '';
		pairingMessage = '';
		browserDiscoveryPending = false;
		browserDiscoveryMessage = '';
		browserDiscoveryID = '';
		browserDiscoveryReceipt = null;
		browserDiscoveryMetadataConsent = false;
		browserDiscoveryHeaderProbeConsent = false;
		browserCSVSchemaReviewPending = false;
		browserCSVSchemaReviewConfirmed = false;
		browserCSVSchemaReview = null;
		browserCapturePending = false;
		browserCaptureMessage = '';
		browserCaptureStatus = null;
		browserCaptureWorkflow = null;
		browserCaptureFrom = '';
		browserCaptureConsent = false;
		browserCaptureRunID = '';
		browserCaptureResumeConsent = false;
		masterDetailTransferConsent = false;
		masterDetailPending = false;
		masterDetailBatchID = '';
		masterDetailStatuses = [];
		masterDetailResumeConsent = {};
		masterDetailMessage = '';
		clearRelayReadinessTimer();
		relayReadinessNonce = '';
		relayReadiness = 'checking';
		preview = null;
		previewPackageID = '';
		ownerContinuationSource = null;
		ownerContinuationCoverage = null;
		ownerContinuationLoading = false;
		ownerContinuationPreviewLoading = false;
		ownerContinuationMessage = '';
		ownerContinuationCoverageMessage = '';
		loadedOwnerContinuationKey = '';
		referencePreview = null;
		referencePreviewPackageID = '';
		browserPreviewAutoRequestedKey = '';
		applyConfirmed = false;
		preparingReferencePreview = false;
		applyingReferencePreview = false;
		referenceApplyConfirmed = false;
		dateWindowFrom = '';
		dateWindowTo = '';
		sourceDiscoveryPending = false;
		discoveredSourceCompanies = [];
		selectedSourceCompanyIDs = [];
		onboardingCatalogConsent = false;
		onboardingCatalogReceiptID = '';
		onboardingCatalogWorkflowID = '';
		onboardingCatalogNonce = '';
		onboardingCatalogGeneration = 0;
		onboardingMode = '';
		onboardingBatchConsent = false;
		onboardingBatchID = '';
		onboardingBatchStatus = '';
		onboardingBatchWorkflow = null;
		onboardingResumeConfirmed = false;
		onboardingBusy = false;
		onboardingMessage = '';
		onboardingBindings = [];
		batchDiscoveryActions.clear();
		batchCaptureActions.clear();
		for (const timer of batchCapturePollTimers.values()) window.clearTimeout(timer);
		batchCapturePollTimers.clear();
		clearOnboardingBatchRefresh();
		onboardingBatchRefreshAttempts = 0;
		error = '';
		// Handoff views use only their dedicated safe endpoints. They do not
		// restore a separate session checkpoint or probe the Brave relay.
		if (accountantReviewRequested || ownerContinuationRequested) return;
		if (relayListenerMounted) requestRelayReadiness();
		// Pre-tenant catalog onboarding has no target tenant yet. Do not read or
		// call any tenant-scoped SmartAccounts status/capture endpoint until the
		// catalog batch has created or reused a target tenant.
		if (currentTenantID) {
			if (!ownerContinuationRequested) {
				const savedSourceCompanyId = sessionStorage.getItem(sourceStorageKey(currentTenantID));
				if (isSafeSourceCompanyId(savedSourceCompanyId)) void loadSavedStatus(savedSourceCompanyId);
			}
		} else {
			const savedBatchID = sessionStorage.getItem(onboardingBatchStorageKey());
			if (isSafeWorkflowID(savedBatchID)) void loadSavedOnboardingBatch(savedBatchID);
		}
	});

	function sourceStorageKey(currentTenantID: string): string {
		return `open-accounting:smartaccounts-source:${currentTenantID}`;
	}

	function browserCaptureWorkflowStorageKey(currentTenantID: string, sourceCompanyID: string): string {
		return `open-accounting:smartaccounts-browser-workflow:${currentTenantID}:${sourceCompanyID}`;
	}

	function onboardingBatchStorageKey(): string {
		// There is deliberately no tenant in this key: selected/all catalog
		// onboarding begins before any tenant exists. The value is only a UUID;
		// owner-authenticated server reads re-check every binding on restore.
		return 'open-accounting:smartaccounts-browser-onboarding-batch:v1';
	}

	function isSafeSourceCompanyId(value: string | null | undefined): value is string {
		return Boolean(value && /^[A-Za-z0-9_.-]{1,128}$/.test(value));
	}

	function isSafeWorkflowID(value: string | null | undefined): value is string {
		return Boolean(value && /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(value));
	}

	function isSafeTenantID(value: string | null | undefined): value is string {
		return Boolean(value && /^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/.test(value));
	}

	function isSafeSHA256(value: string | undefined): value is string {
		return Boolean(value && /^[0-9a-f]{64}$/.test(value));
	}

	function ownerContinuationSourceReady(source: SmartAccountsBrowserBatchWorkflowSource | null): source is SmartAccountsBrowserBatchWorkflowSource & { package_id: string; package_sha256: string; preview_id: string; preview_sha256: string } {
		return Boolean(
			source &&
			source.phase === 'PREVIEW_READY' &&
			isSafeOpaqueID(source.package_id) &&
			isSafeSHA256(source.package_sha256) &&
			isSafeOpaqueID(source.preview_id) &&
			isSafeSHA256(source.preview_sha256)
		);
	}

	function isSafeOpaqueID(value: string | undefined): value is string {
		return Boolean(value && /^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/.test(value));
	}

	function persistSourceCompanyId(currentStatus: SmartAccountsSyncStatus | null) {
		if (!hasTenant) return;
		const sourceCompanyId = currentStatus?.source_company_id?.trim();
		if (isSafeSourceCompanyId(sourceCompanyId)) sessionStorage.setItem(sourceStorageKey(tenantId.trim()), sourceCompanyId);
	}

	async function loadSavedStatus(sourceCompanyId: string) {
		try {
			status = await api.getSmartAccountsSyncStatus(tenantId.trim(), sourceCompanyId);
			const workflowID = sessionStorage.getItem(browserCaptureWorkflowStorageKey(tenantId.trim(), sourceCompanyId));
			if (isSafeWorkflowID(workflowID)) void refreshBraveBrowserCaptureWorkflowStatus(workflowID);
		} catch {
			// An expired/deleted binding must not leave stale UI state behind. The
			// stored value is only an opaque source identity, never a credential.
			sessionStorage.removeItem(sourceStorageKey(tenantId.trim()));
		}
	}

	async function loadOwnerContinuation() {
		const currentTenantID = tenantId.trim();
		const batchID = ownerContinuationBatchId.trim();
		const sourceCompanyID = ownerContinuationSourceCompanyId.trim();
		if (!isSafeTenantID(currentTenantID) || !isSafeWorkflowID(batchID) || !isSafeSourceCompanyId(sourceCompanyID)) {
			error = 'This selected/all tenant continuation has invalid binding identifiers. Return to the owner batch and request a fresh link.';
			return;
		}
		ownerContinuationLoading = true;
		ownerContinuationMessage = 'Loading the current owner-safe selected/all source checkpoint.';
		ownerContinuationCoverage = null;
		ownerContinuationCoverageMessage = '';
		error = '';
		try {
			const workflow = await api.getSmartAccountsBrowserOnboardingBatchWorkflow(batchID);
			if (workflow.workflow.batch_id !== batchID) throw new Error('Selected/all workflow binding mismatch.');
			const source = workflow.sources.find((candidate) =>
				candidate.batch_id === batchID &&
				candidate.tenant_id === currentTenantID &&
				candidate.source_company_id === sourceCompanyID
			);
			if (!source) throw new Error('Selected/all source binding is unavailable or belongs to another tenant.');
			ownerContinuationSource = source;
			if (!ownerContinuationSourceReady(source)) {
				ownerContinuationMessage = `The server reports ${source.phase.replaceAll('_', ' ').toLowerCase()} for this source. GL apply remains unavailable until its exact server-bound preview is ready.`;
				return;
			}
			ownerContinuationMessage = 'The server revalidated this tenant, selected/all source, staged package, and persisted GL preview. Load the current GL preview before the separate financial apply confirmation.';
			await refreshOwnerContinuationCoverage(currentTenantID, source);
		} catch (caught) {
			ownerContinuationSource = null;
			ownerContinuationCoverage = null;
			error = messageFrom(caught, 'The selected/all source continuation is unavailable, stale, or not authorized for this tenant.');
		} finally {
			ownerContinuationLoading = false;
		}
	}

	async function refreshOwnerContinuationCoverage(currentTenantID: string, source: SmartAccountsBrowserBatchWorkflowSource & { package_id: string; package_sha256: string; preview_id: string; preview_sha256: string }) {
		try {
			const report = await api.getSmartAccountsPackageArchiveCoverage(currentTenantID, source.package_id);
			if (report.package_id !== source.package_id || report.package_sha256 !== source.package_sha256) throw new Error('Archive coverage binding mismatch.');
			ownerContinuationCoverage = report;
			ownerContinuationCoverageMessage = '';
		} catch (caught) {
			ownerContinuationCoverage = null;
			ownerContinuationCoverageMessage = messageFrom(caught, 'Count-only archive coverage is not available for the current staged package.');
		}
	}

	async function loadOwnerContinuationPreview() {
		const source = ownerContinuationSource;
		const currentTenantID = tenantId.trim();
		if (!ownerContinuationSourceReady(source) || !isSafeTenantID(currentTenantID) || ownerContinuationPreviewLoading || applyingPreview) return;
		ownerContinuationPreviewLoading = true;
		error = '';
		try {
			// This is the existing non-financial preview endpoint. It reuses the
			// stored digest-identical plan or fails closed if server state changed.
			const next = await api.previewSmartAccountsPackage(currentTenantID, source.package_id, { use_source_chart: true });
			if (
				next.id !== source.preview_id ||
				next.tenant_id !== currentTenantID ||
				next.package_id !== source.package_id ||
				next.source_company_id !== source.source_company_id ||
				next.preview_sha256 !== source.preview_sha256 ||
				(next.status !== 'PREVIEW_READY' && next.status !== 'APPLIED')
			) {
				throw new Error('The reloaded GL preview no longer matches the selected/all server checkpoint.');
			}
			preview = next;
			previewPackageID = next.package_id;
			applyConfirmed = false;
			ownerContinuationMessage = next.status === 'APPLIED'
				? 'The server reports this exact GL preview as already applied. Any reconciliation replay remains a separate owner action.'
				: 'The exact server-bound GL preview is loaded. Financial apply still requires an independently approved accountant policy and an explicit owner confirmation.';
		} catch (caught) {
			preview = null;
			previewPackageID = '';
			error = messageFrom(caught, 'The selected/all GL preview could not be reloaded because its server binding changed or needs review.');
		} finally {
			ownerContinuationPreviewLoading = false;
		}
	}

	$effect(() => {
		const currentTenantID = tenantId.trim();
		const key = `${currentTenantID}\u0000${ownerContinuationBatchId.trim()}\u0000${ownerContinuationSourceCompanyId.trim()}`;
		if (key === loadedOwnerContinuationKey) return;
		loadedOwnerContinuationKey = key;
		ownerContinuationSource = null;
		ownerContinuationCoverage = null;
		ownerContinuationMessage = '';
		ownerContinuationCoverageMessage = '';
		if (!ownerContinuationRequested) return;
		if (accountantReviewRequested || !isSafeTenantID(currentTenantID) || !isSafeWorkflowID(ownerContinuationBatchId.trim()) || !isSafeSourceCompanyId(ownerContinuationSourceCompanyId.trim())) {
			error = 'This selected/all tenant continuation has invalid binding identifiers. Return to the owner batch and request a fresh link.';
			return;
		}
		void loadOwnerContinuation();
	});

	function persistBrowserCaptureWorkflow(workflow: SmartAccountsBrowserCaptureWorkflowStatus | null) {
		if (!hasTenant) return;
		const sourceCompanyID = workflow?.plan.source_company_id;
		if (workflow && isSafeWorkflowID(workflow.workflow_id) && isSafeSourceCompanyId(sourceCompanyID)) {
			sessionStorage.setItem(browserCaptureWorkflowStorageKey(tenantId.trim(), sourceCompanyID), workflow.workflow_id);
		}
	}

	function persistOnboardingBatch(batchID: string) {
		if (isSafeWorkflowID(batchID)) sessionStorage.setItem(onboardingBatchStorageKey(), batchID);
	}

	async function loadOnboardingBatchWorkflowIfReady(batchID: string, batchStatus: 'PENDING' | 'REVIEW_REQUIRED' | 'READY' | 'COMPLETE'): Promise<boolean> {
		if (batchStatus !== 'READY') {
			onboardingBatchWorkflow = null;
			return true;
		}
		try {
			const workflow = await api.getSmartAccountsBrowserOnboardingBatchWorkflow(batchID);
			const expectedTenantBySource = new Map(
				onboardingBindings
					.filter((binding): binding is BrowserOnboardingBinding & { tenantID: string } => Boolean(binding.tenantID))
					.map((binding) => [binding.sourceCompanyID, binding.tenantID])
			);
			if (
				workflow.workflow.batch_id !== batchID ||
				workflow.sources.length !== expectedTenantBySource.size ||
				workflow.sources.some((source) => source.batch_id !== batchID || expectedTenantBySource.get(source.source_company_id) !== source.tenant_id)
			) throw new Error('batch workflow checkpoint mismatch');
			onboardingBatchWorkflow = workflow;
			return true;
		} catch {
			// The paired batch remains the last safe server response. Do not invent
			// workflow state or discard a successful pairing merely because this
			// independent, owner-safe hydration read is temporarily unavailable.
			onboardingBatchWorkflow = null;
			return false;
		}
	}

	async function loadSavedOnboardingBatch(batchID: string) {
		try {
			const response = await api.getSmartAccountsBrowserOnboardingBatch(batchID);
			if (response.batch.batch_id !== batchID) throw new Error('batch checkpoint mismatch');
			onboardingBatchID = batchID;
			onboardingBatchStatus = response.batch.status;
			onboardingBindings = browserOnboardingBindings(response);
			const workflowLoaded = await loadOnboardingBatchWorkflowIfReady(batchID, response.batch.status);
			onboardingMessage = response.batch.status === 'READY'
				? workflowLoaded
					? 'Restored the same immutable selected/all batch and its current safe workflow. No capture or financial action has started.'
					: 'Restored the same immutable selected/all batch. Metadata discovery still needs its separate owner confirmation; no capture or financial action has started.'
				: 'Restored the same immutable selected/all pairing batch. Continuing a lost Brave pairing requires renewed confirmation; no capture or financial action has started.';
			if (response.batch.status === 'PENDING') scheduleOnboardingBatchRefresh(batchID);
		} catch {
			// A different owner, deleted batch, or stale checkpoint must not be
			// presented as resumable. The local value contains no source data.
			sessionStorage.removeItem(onboardingBatchStorageKey());
		}
	}

	function clearRelayReadinessTimer() {
		if (relayReadinessTimer !== undefined) {
			window.clearTimeout(relayReadinessTimer);
			relayReadinessTimer = undefined;
		}
	}

	function clearOnboardingBatchRefresh() {
		if (onboardingBatchRefreshTimer !== undefined) {
			window.clearTimeout(onboardingBatchRefreshTimer);
			onboardingBatchRefreshTimer = undefined;
		}
	}

	function scheduleOnboardingBatchRefresh(batchID = onboardingBatchID) {
		clearOnboardingBatchRefresh();
		if (
			!isSafeWorkflowID(batchID) ||
			batchID !== onboardingBatchID ||
			onboardingBatchStatus !== 'PENDING' ||
			onboardingBatchRefreshAttempts >= onboardingBatchAutoRefreshMaxAttempts
		) return;
		onboardingBatchRefreshTimer = window.setTimeout(() => {
			onboardingBatchRefreshTimer = undefined;
			if (batchID !== onboardingBatchID || onboardingBatchStatus !== 'PENDING') return;
			onboardingBatchRefreshAttempts += 1;
			void refreshOnboardingBatch(batchID).finally(() => {
				if (batchID === onboardingBatchID && onboardingBatchStatus === 'PENDING') scheduleOnboardingBatchRefresh(batchID);
			});
		}, onboardingBatchAutoRefreshDelayMs);
	}

	function newRelayReadinessNonce(): string {
		if (!globalThis.crypto?.getRandomValues || typeof globalThis.btoa !== 'function') return '';
		const bytes = new Uint8Array(32);
		globalThis.crypto.getRandomValues(bytes);
		let binary = '';
		for (const value of bytes) binary += String.fromCharCode(value);
		return globalThis.btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
	}

	function requestRelayReadiness() {
		if (typeof window === 'undefined') return;
		clearRelayReadinessTimer();
		const nonce = newRelayReadinessNonce();
		if (nonce.length !== 43) {
			relayReadinessNonce = '';
			relayReadiness = 'missing';
			return;
		}
		const issuedAt = new Date();
		const expiresAt = new Date(issuedAt.getTime() + 60_000);
		relayReadinessNonce = nonce;
		relayReadiness = 'checking';
		window.postMessage({
			source: 'open-accounting',
			type: 'open-accounting.smartaccounts-browser-readiness-ping.v1',
			version: 1,
			nonce,
			issued_at: issuedAt.toISOString(),
			expires_at: expiresAt.toISOString()
		}, window.location.origin);
		relayReadinessTimer = window.setTimeout(() => {
			if (relayReadinessNonce === nonce && relayReadiness === 'checking') relayReadiness = 'missing';
		}, relayReadinessTimeoutMs);
	}

	function requireReadyBrowserRelay(): boolean {
		if (browserRelayReady) return true;
		error = relayReadiness === 'signed_out'
			? 'Sign in to SmartAccounts in Brave, then check the Browser Relay before continuing.'
			: relayReadiness === 'stale'
				? 'Reload or update the Browser Relay before continuing.'
				: 'Wait for the Browser Relay readiness check before continuing.';
		return false;
	}

	function receiveRelayReadiness(value: unknown) {
		if (!value || typeof value !== 'object') return;
		const response = value as Record<string, unknown>;
		if (
			response.source !== 'smartaccounts-browser-relay' ||
			response.type !== 'smartaccounts-browser-relay.readiness.v1' ||
			response.version !== 1 ||
			typeof response.nonce !== 'string' ||
			response.nonce !== relayReadinessNonce
		) return;
		clearRelayReadinessTimer();
		if (
			Object.keys(response).length !== relayReadinessResponseFields.length ||
			!relayReadinessResponseFields.every((field) => Object.hasOwn(response, field)) ||
			response.relay_protocol_version !== relayReadinessProtocolVersion ||
			response.capture_manifest_version !== relayCaptureManifestVersion ||
			response.workflow_plan_version !== relayWorkflowPlanVersion
		) {
			relayReadiness = 'stale';
			return;
		}
		switch (response.smartaccounts_session_state) {
			case 'signed_in':
				relayReadiness = 'ready';
				return;
			case 'signed_out':
				relayReadiness = 'signed_out';
				return;
			case 'unknown':
				relayReadiness = 'unknown';
				return;
			default:
				relayReadiness = 'stale';
		}
	}

	async function configure() {
		if (!hasTenant || !canConfigure) return;
		saving = true;
		error = '';
		try {
			status = await api.configureSmartAccountsSync(tenantId.trim(), {
				api_key: apiKey.trim(),
				api_secret: apiSecret.trim(),
				smartaccounts_gl_authoritative: true,
				invoice_payment_mode: 'NON_POSTING'
			});
			persistSourceCompanyId(status);
			await startFullHistoryCapture();
		} catch (caught) {
			error = messageFrom(caught, 'Could not save SmartAccounts sync control.');
		} finally {
			// Never retain either raw credential after a bridge response, whether
			// configuration succeeded or failed.
			apiKey = '';
			apiSecret = '';
			saving = false;
		}
	}

	async function connectWithBrave() {
		if (!hasTenant || !requireReadyBrowserRelay() || saving || pairingPending) return;
		saving = true;
		error = '';
		pairingMessage = '';
		try {
			const issue = await api.createSmartAccountsBrowserPairing(tenantId.trim());
			pairingID = issue.pairing_id;
			pairingPending = true;
			// The token exists only long enough for the installed relay's isolated
			// content script to receive this same-window event. It is not kept in
			// component state, session storage, logs, or the rendered DOM.
			let pairingToken = issue.pairing_token;
			window.postMessage({
				source: 'open-accounting',
				type: 'open-accounting.smartaccounts-browser-pairing-issued.v1',
				version: 1,
				pairing_id: issue.pairing_id,
				pairing_token: pairingToken,
				api_base_url: new URL(getApiBase()).origin,
				expires_at: issue.expires_at
			}, window.location.origin);
			pairingToken = '';
			pairingMessage = 'Waiting for the installed SmartAccounts Browser Relay to pair the signed-in Brave session.';
		} catch (caught) {
			error = messageFrom(caught, 'Could not start Brave pairing.');
			pairingID = '';
			pairingPending = false;
		} finally {
			saving = false;
		}
	}

	async function refreshBrowserPairing() {
		if (!pairingID) return;
		try {
			const pairing = await api.getSmartAccountsBrowserPairing(tenantId.trim(), pairingID);
			if (pairing.status !== 'CLAIMED' || !isSafeSourceCompanyId(pairing.source_company_id)) return;
			status = await api.getSmartAccountsSyncStatus(tenantId.trim(), pairing.source_company_id);
			persistSourceCompanyId(status);
			pairingPending = false;
			pairingID = '';
			pairingMessage = 'Brave session paired. Choose the history start and confirm the journal-only transfer below before any CSV leaves Brave.';
		} catch (caught) {
			pairingPending = false;
			pairingID = '';
			error = messageFrom(caught, 'Brave pairing expired or could not be verified.');
		}
	}

	function browserCaptureIssueEvent(issue: SmartAccountsBrowserCaptureIssue) {
		return {
			source: 'open-accounting',
			type: 'open-accounting.smartaccounts-browser-capture-issued.v1',
			version: 1,
			run_id: issue.run_id,
			tenant_id: issue.tenant_id,
			capture_token: issue.capture_token,
			expires_at: issue.expires_at,
			source_company_id: issue.source_company_id,
			manifest_version: issue.manifest_version,
			scope: issue.scope,
			transfer_consent: issue.transfer_consent,
			api_base_url: new URL(getApiBase()).origin
		};
	}

	function browserMasterDetailIssueEvent(issue: SmartAccountsBrowserMasterDetailIssue) {
		return {
			source: 'open-accounting',
			type: 'open-accounting.smartaccounts-browser-master-detail-issued.v1',
			version: 1,
			run_id: issue.run_id,
			tenant_id: issue.tenant_id,
			source_company_id: issue.source_company_id,
			manifest_version: issue.manifest_version,
			resource_id: issue.resource_id,
			schema_id: issue.schema_id,
			source_schema: issue.source_schema,
			capture_token: issue.capture_token,
			expires_at: issue.expires_at,
			api_base_url: new URL(getApiBase()).origin,
			contract_sha256: issue.contract_sha256,
			approval_sha256: issue.approval_sha256,
			scope: issue.scope,
			transfer_consent: issue.transfer_consent,
			contract: issue.contract,
			workflow: {
				version: 'smartaccounts-browser-master-detail-workflow-v1',
				sequence: issue.sequence,
				resources: ['clients', 'vendors', 'articles']
			}
		};
	}

	async function refreshMasterDetailStatuses(runIDs = masterDetailStatuses.map((entry) => entry.run_id)) {
		if (!tenantId.trim() || runIDs.length === 0) return;
		masterDetailPending = true;
		try {
			// Owner status is safe to poll: it carries package receipt metadata but
			// never captures a relay capability, canonical row, or source path.
			masterDetailStatuses = await Promise.all(runIDs.map((runID) => api.getSmartAccountsBrowserMasterDetailStatus(tenantId.trim(), runID)));
			const staged = masterDetailStatuses.filter((entry) => entry.status === 'STAGED_REVIEW_REQUIRED').length;
			masterDetailMessage = staged > 0
				? `${staged} master-data package${staged === 1 ? '' : 's'} is checksum-staged for separate non-financial reference review.`
				: 'Master-data progress was refreshed. A sealed bridge package remains evidence-only until OA verifies its exact staged package digest.';
		} catch (caught) {
			error = messageFrom(caught, 'Could not refresh the safe master-detail status.');
		} finally {
			masterDetailPending = false;
		}
	}

	async function authorizeMasterDetails() {
		const sourceCompanyID = status?.source_company_id?.trim() ?? '';
		if (!requireReadyBrowserRelay() || masterDetailPending) return;
		if (!/^sa-browser-v1-\d{1,20}$/.test(sourceCompanyID)) {
			error = 'Pair this Brave source to the current tenant before authorizing master-data snapshots.';
			return;
		}
		if (!masterDetailTransferConsent) {
			error = 'Confirm the reviewed master-data transfer before issuing its three serial relay runs.';
			return;
		}
		masterDetailPending = true;
		error = '';
		try {
			const issued = await api.issueSmartAccountsBrowserMasterDetails(tenantId.trim(), {
				source_company_id: sourceCompanyID,
				transfer_consent_confirmed: true
			});
			if (issued.issues.length !== 3 || issued.issues.some((issue, index) => issue.sequence !== index + 1 || issue.tenant_id !== tenantId.trim() || issue.source_company_id !== sourceCompanyID)) throw new Error('Master-detail issue set binding mismatch');
			masterDetailBatchID = issued.batch_id;
			// Each high-entropy capability crosses once from this action response to
			// same-window relay memory. It is not retained in reactive state, the
			// DOM, storage, URLs, or safe owner status.
			for (const issue of issued.issues) {
				window.postMessage(browserMasterDetailIssueEvent(issue), window.location.origin);
				issue.capture_token = '';
			}
			masterDetailTransferConsent = false;
			masterDetailMessage = 'Clients, vendors, and articles were issued as one serial current-snapshot workflow. Articles remain review-only because no VAT mapping is approved.';
			await refreshMasterDetailStatuses(issued.issues.map((issue) => issue.run_id));
		} catch (caught) {
			error = messageFrom(caught, 'Could not authorize the master-data relay workflow.');
		} finally {
			masterDetailPending = false;
		}
	}

	async function resumeMasterDetail(status: SmartAccountsBrowserMasterDetailStatus) {
		if (!requireReadyBrowserRelay() || masterDetailPending || !masterDetailResumeConsent[status.run_id]) {
			error = 'Confirm renewed transfer consent before resuming this exact master-data resource run.';
			return;
		}
		masterDetailPending = true;
		error = '';
		try {
			const issue = await api.resumeSmartAccountsBrowserMasterDetail(tenantId.trim(), status.run_id, { transfer_consent_confirmed: true });
			if (issue.run_id !== status.run_id || issue.tenant_id !== tenantId.trim() || issue.source_company_id !== status.source_company_id || issue.resource_id !== status.resource_id) throw new Error('Master-detail resume binding mismatch');
			window.postMessage(browserMasterDetailIssueEvent(issue), window.location.origin);
			issue.capture_token = '';
			masterDetailResumeConsent = { ...masterDetailResumeConsent, [status.run_id]: false };
			masterDetailMessage = `A fresh short-lived capability was issued for the same ${status.resource_id} snapshot run. The prior capability is invalid.`;
			await refreshMasterDetailStatuses([status.run_id]);
		} catch (caught) {
			error = messageFrom(caught, 'Could not resume the exact master-data snapshot run.');
		} finally {
			masterDetailPending = false;
		}
	}

	function browserDiscoveryIssueEvent(issue: SmartAccountsBrowserDiscoveryIssue) {
		return {
			source: 'open-accounting',
			type: 'open-accounting.smartaccounts-browser-discovery-issued.v1',
			version: 1,
			discovery_id: issue.discovery_id,
			source_company_id: issue.source_company_id,
			manifest_version: issue.manifest_version,
			resource_ids: issue.resource_ids,
			expires_at: issue.expires_at,
			discovery_consent: issue.discovery_consent
		};
	}

	async function startBraveBrowserDiscovery() {
		if (!requireReadyBrowserRelay() || browserDiscoveryPending) return;
		const sourceCompanyID = status?.source_company_id?.trim() ?? '';
		if (!/^sa-browser-v1-\d{1,20}$/.test(sourceCompanyID)) {
			error = 'Pair this Brave source to the current tenant before authorizing metadata discovery.';
			return;
		}
		if (!browserDiscoveryMetadataConsent) {
			error = 'Confirm metadata-only discovery before the Brave relay may inspect the selected browser surfaces.';
			return;
		}
		browserDiscoveryPending = true;
		error = '';
		browserDiscoveryMessage = '';
		try {
			const issue = await api.createSmartAccountsBrowserDiscovery(tenantId.trim(), {
				source_company_id: sourceCompanyID,
				metadata_only_consent_confirmed: true,
				response_header_probe_confirmed: browserDiscoveryHeaderProbeConsent
			});
			browserDiscoveryID = issue.discovery_id;
			browserDiscoveryReceipt = null;
			browserCSVSchemaReviewConfirmed = false;
			browserCSVSchemaReview = null;
			// The issue has no capability. It moves once to same-window extension
			// memory, where its fresh consent and 10-minute bounded expiry are
			// independently rechecked. It is never kept in browser storage.
			window.postMessage(browserDiscoveryIssueEvent(issue), window.location.origin);
			browserDiscoveryMetadataConsent = false;
			browserDiscoveryHeaderProbeConsent = false;
			browserDiscoveryMessage = `Brave is checking the server-derived ${issue.resource_ids.length}-surface metadata manifest. This reads no source rows and does not start capture or accounting apply.`;
		} catch (caught) {
			error = messageFrom(caught, 'Could not authorize Brave metadata discovery.');
			browserDiscoveryID = '';
		} finally {
			browserDiscoveryPending = false;
		}
	}

	async function reviewGeneralLedgerCSVSchema() {
		if (!browserDiscoveryID || !browserDiscoveryReceipt || browserDiscoveryReceipt.status !== 'completed' || browserDiscoveryReceipt.capture_ready_count < 1 || browserCSVSchemaReviewPending) return;
		if (!browserCSVSchemaReviewConfirmed) {
			error = 'Confirm the reviewed General Ledger CSV schema before registering its authoritative-source adapter.';
			return;
		}
		browserCSVSchemaReviewPending = true;
		error = '';
		try {
			// The fixed adapter identity is public contract metadata. The request
			// contains no source selector, header, CSV data, cookie, credential,
			// audit ID, or bridge token; OA derives and persists those bindings.
			browserCSVSchemaReview = await api.reviewSmartAccountsBrowserCSVSchema(
				tenantId.trim(), browserDiscoveryID, 'general_ledger', 'general_ledger_csv_v1'
			);
			browserCSVSchemaReviewConfirmed = false;
			browserDiscoveryMessage = 'General Ledger CSV schema review registered. This approves only the authoritative source boundary; capture and accounting apply remain separate actions.';
		} catch (caught) {
			error = messageFrom(caught, 'Could not register the reviewed General Ledger CSV schema.');
		} finally {
			browserCSVSchemaReviewPending = false;
		}
	}

	function validBrowserDiscoveryRelayResult(value: unknown): value is SmartAccountsBrowserDiscoveryRelayResult {
		if (!value || typeof value !== 'object') return false;
		const result = value as Record<string, unknown>;
		const fields = ['source', 'type', 'version', 'discovery_id', 'manifest_version', 'contract_version', 'status', 'resources'];
		return Object.keys(result).length === fields.length && fields.every((field) => Object.hasOwn(result, field)) &&
			result.source === 'smartaccounts-browser-relay' &&
			result.type === 'smartaccounts-browser-relay.discovery-result.v1' &&
			result.version === 1 && result.discovery_id === browserDiscoveryID &&
			result.manifest_version === relayCaptureManifestVersion &&
			result.contract_version === 'smartaccounts-brave-discovery-contract-v1' &&
			['completed', 'awaiting_browser', 'company_binding_blocked', 'expired', 'discovery_failed'].includes(String(result.status)) &&
			Array.isArray(result.resources);
	}

	async function receiveBraveBrowserDiscoveryResult(result: SmartAccountsBrowserDiscoveryRelayResult) {
		if (!browserDiscoveryID || browserDiscoveryPending) return;
		browserDiscoveryPending = true;
		try {
			// Do not retain or render the relay result: it contains a redacted
			// private contract. OA's owner endpoint reduces it to a safe digest and
			// aggregate receipt before this component stores any result.
			const receipt = await api.submitSmartAccountsBrowserDiscoveryReceipt(tenantId.trim(), browserDiscoveryID, result);
			browserDiscoveryReceipt = receipt;
			browserDiscoveryMessage = receipt.status === 'completed'
				? 'Redacted discovery receipt recorded. It is metadata evidence only; capture, package review, and financial apply remain separate explicit actions.'
				: `Redacted discovery ended as ${receipt.status}; its safe partial progress is recorded. Start a new owner-consented discovery to retry.`;
		} catch (caught) {
			error = messageFrom(caught, 'Could not record the redacted Brave discovery receipt.');
		} finally {
			browserDiscoveryPending = false;
		}
	}

	async function refreshBraveBrowserDiscoveryReceipt() {
		if (!browserDiscoveryID || browserDiscoveryPending) return;
		browserDiscoveryPending = true;
		try {
			browserDiscoveryReceipt = await api.getSmartAccountsBrowserDiscoveryReceipt(tenantId.trim(), browserDiscoveryID);
			browserDiscoveryMessage = 'Refreshed the owner-safe redacted discovery receipt.';
		} catch (caught) {
			error = messageFrom(caught, 'Could not refresh the redacted Brave discovery receipt.');
		} finally {
			browserDiscoveryPending = false;
		}
	}

	function browserCaptureWorkflowIssueEvent(issue: SmartAccountsBrowserCaptureIssue, plan: SmartAccountsBrowserCapturePlan) {
		return {
			...browserCaptureIssueEvent(issue),
			type: 'open-accounting.smartaccounts-browser-workflow-issued.v1',
			workflow: {
				version: 'smartaccounts-browser-workflow-v1',
				operation: 'capture',
				plan: {
					version: plan.version,
					run_id: issue.run_id,
					tenant_id: plan.tenant_id,
					source_company_id: plan.source_company_id,
					manifest_version: plan.manifest_version,
					scope: plan.scope,
					eligible_resource_ids: plan.eligible_resource_ids
				}
			}
		};
	}

	function sendBrowserCaptureIssue(issue: SmartAccountsBrowserCaptureIssue, plan?: SmartAccountsBrowserCapturePlan) {
		// The token crosses directly from this one action response into the
		// extension's in-memory service worker. It is never put in component
		// state, storage, a URL, or rendered markup.
		let captureToken = issue.capture_token;
		const event = plan
			? browserCaptureWorkflowIssueEvent({ ...issue, capture_token: captureToken }, plan)
			: browserCaptureIssueEvent({ ...issue, capture_token: captureToken });
		window.postMessage(event, window.location.origin);
		captureToken = '';
	}

	async function startBraveBrowserCaptureWorkflow(sourceCompanyID = status?.source_company_id?.trim() ?? '', fromInclusive = browserCaptureFrom, consent = browserCaptureConsent) {
		if (!requireReadyBrowserRelay()) return;
		if (!/^sa-browser-v1-\d{1,20}$/.test(sourceCompanyID)) {
			error = 'Pair this Brave source to the current tenant before authorizing a capture.';
			return;
		}
		if (!fromInclusive || !consent) {
			error = 'Choose the history start date and confirm this transfer before starting the Brave journal capture.';
			return;
		}
		browserCapturePending = true;
		error = '';
		browserCaptureMessage = '';
		try {
			const workflow = await api.startSmartAccountsBrowserCaptureWorkflow(tenantId.trim(), {
				source_company_id: sourceCompanyID,
				from_inclusive: fromInclusive,
				transfer_consent_confirmed: true
			});
			browserCaptureWorkflow = workflow;
			persistBrowserCaptureWorkflow(workflow);
			const issue = workflow.capture;
			if (!issue) throw new Error('The server did not issue the requested Brave capture capability.');
			sendBrowserCaptureIssue(issue, workflow.plan);
			browserCaptureRunID = issue.run_id;
			browserCaptureResumeConsent = false;
			browserCaptureConsent = false;
			browserCaptureMessage = 'Journal CSV transfer authorized for the server-derived partial scope. The Brave relay must confirm consent and upload only that CSV.';
			await refreshBraveBrowserCaptureWorkflowStatus(workflow.workflow_id);
		} catch (caught) {
			error = messageFrom(caught, 'Could not start the Brave journal capture.');
		} finally {
			browserCapturePending = false;
		}
	}

	async function resumeBraveBrowserCapture() {
		if (!requireReadyBrowserRelay()) return;
		if (!browserCaptureRunID || !browserCaptureResumeConsent || browserCapturePending) {
			error = 'Confirm renewed transfer consent before resuming this Brave capture.';
			return;
		}
		browserCapturePending = true;
		error = '';
		try {
			const issue = await api.resumeSmartAccountsBrowserCapture(tenantId.trim(), browserCaptureRunID, { transfer_consent_confirmed: true });
			sendBrowserCaptureIssue(issue, browserCaptureWorkflow?.plan);
			browserCaptureMessage = 'A new ten-minute capability was issued for the same scope and run. The previous capability is invalid.';
			browserCaptureResumeConsent = false;
			if (browserCaptureWorkflow) await refreshBraveBrowserCaptureWorkflowStatus(browserCaptureWorkflow.workflow_id);
			else await refreshBraveBrowserCaptureStatus(issue.run_id);
		} catch (caught) {
			error = messageFrom(caught, 'Could not resume the Brave browser capture.');
		} finally {
			browserCapturePending = false;
		}
	}

	async function refreshBraveBrowserCaptureStatus(runID = browserCaptureRunID) {
		if (!runID || !tenantId.trim()) return;
		try {
			const next = await api.getSmartAccountsBrowserCaptureStatus(tenantId.trim(), runID);
			// The response is owner-authenticated and server-bound. Keep only its
			// safe immutable progress; it cannot include the relay capability.
			browserCaptureStatus = next;
			browserCaptureRunID = next.run_id;
			if (next.status === 'finalized_partial' && next.receipt?.status === 'partial_coverage_recorded') {
				browserCaptureMessage = next.staging?.status === 'staged_review_required'
					? 'Partial browser coverage is staged for explicit package review. It is not full-history coverage.'
					: 'Partial browser coverage receipt recorded. It is explicitly not full-history coverage.';
			}
			void queueBrowserCapturePreview(next);
		} catch {
			// The relay can still be running. Do not replace an existing safe status
			// with an error merely because the owner view is temporarily unavailable.
		}
	}

	async function refreshBraveBrowserCaptureWorkflowStatus(workflowID = browserCaptureWorkflow?.workflow_id ?? '') {
		if (!workflowID || !tenantId.trim()) return;
		try {
			const next = await api.getSmartAccountsBrowserCaptureWorkflowStatus(tenantId.trim(), workflowID);
			browserCaptureWorkflow = next;
			persistBrowserCaptureWorkflow(next);
			if (next.progress) {
				browserCaptureStatus = next.progress;
				browserCaptureRunID = next.progress.run_id;
				if (next.progress.status === 'finalized_partial' && next.progress.receipt?.status === 'partial_coverage_recorded') {
					browserCaptureMessage = next.progress.staging?.status === 'staged_review_required'
						? 'Partial journal coverage is staged for explicit package review. It is not full-history coverage.'
						: 'Partial journal coverage receipt recorded. It is explicitly not full-history coverage.';
				}
				void queueBrowserCapturePreview(next.progress);
			}
		} catch {
			// Owner status is advisory. Keep prior safe state during a transient
			// bridge poll failure rather than implying browser data was lost.
		}
	}

	function hasControlCharacter(value: string): boolean {
		for (const character of value) {
			const code = character.charCodeAt(0);
			if (code <= 0x1f || code === 0x7f) return true;
		}
		return false;
	}

	function sourceCompaniesFromBrowserMessage(value: unknown): BrowserSourceCompany[] {
		if (!Array.isArray(value) || value.length === 0 || value.length > browserOnboardingMaxSources) return [];
		const ids = new Set<string>();
		const companies: BrowserSourceCompany[] = [];
		for (const candidate of value) {
			if (!candidate || typeof candidate !== 'object') return [];
			const sourceCompanyID = String((candidate as { source_company_id?: unknown }).source_company_id ?? '').trim();
			const sourceCompanyName = String((candidate as { source_company_name?: unknown }).source_company_name ?? '').replace(/\s+/g, ' ').trim();
			if (!/^sa-browser-v1-\d{1,20}$/.test(sourceCompanyID) || new TextEncoder().encode(sourceCompanyName).length === 0 || new TextEncoder().encode(sourceCompanyName).length > 120 || sourceCompanyName !== sourceCompanyName.replace(/\s+/g, ' ').trim() || hasControlCharacter(sourceCompanyName) || ids.has(sourceCompanyID)) return [];
			ids.add(sourceCompanyID);
			companies.push({ source_company_id: sourceCompanyID, source_company_name: sourceCompanyName });
		}
		return companies.sort((left, right) => left.source_company_name.localeCompare(right.source_company_name) || left.source_company_id.localeCompare(right.source_company_id));
	}


	function browserCatalogIssueEvent(issue: SmartAccountsBrowserOnboardingCatalogIssue) {
		return {
			source: 'open-accounting',
			type: 'open-accounting.smartaccounts-browser-source-catalog-issued.v1',
			version: 1,
			catalog_id: issue.catalog_id,
			workflow_id: issue.workflow_id,
			catalog_token: issue.catalog_token,
			api_base_url: new URL(getApiBase()).origin,
			nonce: issue.nonce,
			issued_at: issue.issued_at,
			expires_at: issue.expires_at,
			catalog_digest_intent: issue.catalog_digest_intent,
			catalog_consent: issue.catalog_consent
		};
	}

	async function requestBrowserSourceDiscovery() {
		if (!requireReadyBrowserRelay() || sourceDiscoveryPending || onboardingBusy || !onboardingCatalogConsent) return;
		error = '';
		onboardingMessage = '';
		sourceDiscoveryPending = true;
		const generation = ++onboardingCatalogGeneration;
		try {
			const issued = await api.issueSmartAccountsBrowserOnboardingCatalog({
				catalog_consent: {
					version: 1,
					confirmed: true,
					confirmed_at: new Date().toISOString(),
					scope: 'visible_company_catalog'
				}
			});
			if (generation !== onboardingCatalogGeneration) return;
			onboardingCatalogReceiptID = issued.catalog_id;
			onboardingCatalogWorkflowID = issued.workflow_id;
			onboardingCatalogNonce = issued.nonce;
			window.postMessage(browserCatalogIssueEvent(issued), window.location.origin);
			// The raw capability is action-response-only: it crosses straight into
			// relay memory and must not remain in component state after dispatch.
			issued.catalog_token = '';
			onboardingMessage = 'Reading the relay-observed visible company catalog…';
		} catch (caught) {
			sourceDiscoveryPending = false;
			error = messageFrom(caught, 'Could not authorize the visible SmartAccounts company catalog.');
		}
	}

	function validBrowserCatalogResult(value: unknown): value is {
		source: 'smartaccounts-browser-relay';
		type: 'smartaccounts-browser-relay.source-catalog-result.v1';
		version: 1;
		catalog_id: string;
		workflow_id: string;
		nonce: string;
		status: 'accepted' | 'already_accepted' | 'awaiting_browser' | 'catalog_blocked' | 'expired';
		catalog_count?: number;
		catalog_sha256?: string;
	} {
		if (!value || typeof value !== 'object' || Array.isArray(value)) return false;
		const record = value as Record<string, unknown>;
		const allowed = new Set(['source', 'type', 'version', 'catalog_id', 'workflow_id', 'nonce', 'status', 'catalog_count', 'catalog_sha256']);
		if (Object.keys(record).some((key) => !allowed.has(key))) return false;
		if (record.source !== 'smartaccounts-browser-relay' || record.type !== 'smartaccounts-browser-relay.source-catalog-result.v1' || record.version !== 1 || typeof record.catalog_id !== 'string' || typeof record.workflow_id !== 'string' || typeof record.nonce !== 'string' || !['accepted', 'already_accepted', 'awaiting_browser', 'catalog_blocked', 'expired'].includes(String(record.status))) return false;
		const hasDigest = typeof record.catalog_sha256 === 'string' && /^[0-9a-f]{64}$/.test(record.catalog_sha256);
		const hasCount = Number.isInteger(record.catalog_count) && Number(record.catalog_count) >= 1 && Number(record.catalog_count) <= browserOnboardingMaxSources;
		return (record.status === 'accepted' || record.status === 'already_accepted') ? hasDigest && hasCount : record.catalog_sha256 === undefined && record.catalog_count === undefined;
	}

	function toggleSourceCompany(sourceCompanyID: string, checked: boolean) {
		selectedSourceCompanyIDs = checked
			? [...new Set([...selectedSourceCompanyIDs, sourceCompanyID])]
			: selectedSourceCompanyIDs.filter((id) => id !== sourceCompanyID);
	}

	function browserPairingEvent(pairing: { pairing_id: string; pairing_token: string; expires_at: string }, sourceCompanyID: string) {
		return {
			source: 'open-accounting',
			type: 'open-accounting.smartaccounts-browser-pairing-issued.v1',
			version: 1,
			pairing_id: pairing.pairing_id,
			pairing_token: pairing.pairing_token,
			api_base_url: new URL(getApiBase()).origin,
			expires_at: pairing.expires_at,
			source_company_id: sourceCompanyID
		};
	}

	function browserOnboardingBinding(result: SmartAccountsBrowserOnboardingResult): BrowserOnboardingBinding {
		return {
			sourceCompanyID: result.source_company_id,
			sourceCompanyName: result.source_company_name,
			tenantID: result.tenant_id,
			tenantName: result.tenant_name,
			pairingID: result.pairing_id,
			status: result.status === 'PAIRED'
				? 'claimed'
				: result.status === 'PAIRING_ISSUED'
					? 'waiting'
					: result.status === 'REVIEW_REQUIRED'
						? 'review_required'
						: 'failed',
			reasonCode: result.reason_code,
			tenantCreated: result.tenant_created,
			tenantReused: result.tenant_reused
		};
	}

	function browserOnboardingBindings(response: SmartAccountsBrowserOnboardingBatchResponse): BrowserOnboardingBinding[] {
		return response.outcomes.map(browserOnboardingBinding);
	}

	async function refreshOnboardingBatch(batchID = onboardingBatchID) {
		if (!batchID) return;
		try {
			const response = await api.getSmartAccountsBrowserOnboardingBatch(batchID);
			if (response.batch.batch_id !== batchID || batchID !== onboardingBatchID) throw new Error('batch status mismatch');
			onboardingBatchStatus = response.batch.status;
			onboardingBindings = browserOnboardingBindings(response);
			const workflowLoaded = await loadOnboardingBatchWorkflowIfReady(batchID, response.batch.status);
			if (response.batch.status !== 'PENDING') clearOnboardingBatchRefresh();
			onboardingMessage = response.batch.status === 'READY'
				? workflowLoaded
					? 'Every selected company is paired to its own Open Accounting tenant and the current safe workflow is loaded. Browser discovery and reviewed CSV-schema approval are required before any partial capture can be issued.'
					: 'Every selected company is paired to its own Open Accounting tenant. The safe workflow is not prepared yet; metadata discovery still requires its separate owner confirmation.'
				: 'Selected-company pairing progress was refreshed automatically. Each tenant remains isolated.';
		} catch {
			// Retain the last safe response during advisory status polling.
		}
	}

	async function onboardSelectedSourceCompanies() {
		if (!requireReadyBrowserRelay()) return;
		const allIDs = discoveredSourceCompanies.map((candidate) => candidate.source_company_id).sort();
		const selectedIDs = [...new Set(selectedSourceCompanyIDs)].sort();
		if (!onboardingCatalogReceiptID || !onboardingMode || onboardingBusy || !onboardingBatchConsent) return;
		if (onboardingMode === 'all' && (selectedIDs.length !== allIDs.length || selectedIDs.some((value, index) => value !== allIDs[index]))) return;
		if (onboardingMode === 'selected' && (selectedIDs.length === 0 || selectedIDs.length >= allIDs.length)) return;
		onboardingBusy = true;
		error = '';
		onboardingMessage = '';
		try {
			const response = await api.startSmartAccountsBrowserOnboardingBatch({
				catalog_receipt_id: onboardingCatalogReceiptID,
				mode: onboardingMode,
				selected_source_ids: selectedIDs,
				owner_confirmed: true
			});
			const batch: Array<ReturnType<typeof browserPairingEvent>> = [];
			const pendingBindings = browserOnboardingBindings(response);
			for (const issue of response.pairing_issues ?? []) {
				batch.push(browserPairingEvent(issue.pairing, issue.source_company_id));
				// Pairing tokens are action-response only. Do not keep one in the
				// response object after it has crossed into the relay's memory.
				issue.pairing.pairing_token = '';
			}
			onboardingBatchID = response.batch.batch_id;
			persistOnboardingBatch(response.batch.batch_id);
			onboardingBatchStatus = response.batch.status;
			onboardingBindings = pendingBindings;
			onboardingBatchRefreshAttempts = 0;
			await loadOnboardingBatchWorkflowIfReady(response.batch.batch_id, response.batch.status);
			if (response.batch.status === 'PENDING') scheduleOnboardingBatchRefresh(response.batch.batch_id);
			if (batch.length > 0) window.postMessage({ source: 'open-accounting', type: 'open-accounting.smartaccounts-browser-pairing-batch-issued.v1', version: 1, pairings: batch }, window.location.origin);
			// `batch` is function-local only and never enters component state or
			// browser storage. The relay receives the short-lived tokens directly
			// from this action response.
			const waiting = pendingBindings.filter((binding) => binding.status === 'waiting').length;
			const paired = pendingBindings.filter((binding) => binding.status === 'claimed');
			const needsReview = pendingBindings.filter((binding) => binding.status === 'review_required').length;
			const failed = pendingBindings.filter((binding) => binding.status === 'failed').length;
			onboardingMessage = `${waiting ? `Preparing ${waiting} isolated Brave pairing${waiting === 1 ? '' : 's'}. ` : ''}${paired.length ? `${paired.length} verified binding${paired.length === 1 ? '' : 's'} reused. ` : ''}${needsReview || failed ? `${needsReview + failed} selected source${needsReview + failed === 1 ? ' needs' : 's need'} review; other companies can continue.` : 'Each company remains tenant-isolated. Discovery receipt and reviewed schema approval are required before partial capture; no financial data has been posted.'}`;
		} catch (caught) {
			error = messageFrom(caught, 'Could not prepare the selected Open Accounting tenants.');
		} finally {
			onboardingBusy = false;
		}
	}

	async function resumeOnboardingBatch() {
		if (!onboardingBatchID || onboardingBusy || !onboardingResumeConfirmed || !requireReadyBrowserRelay()) return;
		onboardingBusy = true;
		error = '';
		try {
			const response = await api.resumeSmartAccountsBrowserOnboardingBatch(onboardingBatchID);
			if (response.batch.batch_id !== onboardingBatchID) throw new Error('batch resume binding mismatch');
			const relayPairings: Array<ReturnType<typeof browserPairingEvent>> = [];
			for (const issue of response.pairing_issues ?? []) {
				relayPairings.push(browserPairingEvent(issue.pairing, issue.source_company_id));
				issue.pairing.pairing_token = '';
			}
			onboardingBindings = browserOnboardingBindings(response);
			onboardingBatchStatus = response.batch.status;
			onboardingBatchRefreshAttempts = 0;
			const workflowLoaded = await loadOnboardingBatchWorkflowIfReady(response.batch.batch_id, response.batch.status);
			if (response.batch.status === 'PENDING') scheduleOnboardingBatchRefresh(response.batch.batch_id);
			if (relayPairings.length > 0) window.postMessage({ source: 'open-accounting', type: 'open-accounting.smartaccounts-browser-pairing-batch-issued.v1', version: 1, pairings: relayPairings }, window.location.origin);
			onboardingMessage = response.batch.status === 'READY'
				? workflowLoaded
					? 'The same immutable batch is paired and its safe workflow is loaded. No capture or financial action was started.'
					: 'The same immutable batch is paired. Metadata discovery still needs its separate owner confirmation; no capture or financial action was started.'
				: 'Continuing the same immutable pairing batch. Pairing status refreshes automatically; no capture or financial action was started.';
		} catch (caught) {
			error = messageFrom(caught, 'Could not resume the same SmartAccounts pairing batch.');
		} finally {
			onboardingBusy = false;
		}
	}

	function batchCompanyNames(): Record<string, string> {
		return Object.fromEntries(onboardingBindings.map((binding) => [binding.sourceCompanyID, binding.sourceCompanyName]));
	}

	async function getOnboardingBatchWorkflow(): Promise<SmartAccountsBrowserBatchWorkflowStatus> {
		if (!onboardingBatchID) throw new Error('A completed selected/all pairing batch is required before workflow status can be loaded.');
		const next = await api.getSmartAccountsBrowserOnboardingBatchWorkflow(onboardingBatchID);
		onboardingBatchWorkflow = next;
		return next;
	}

	async function prepareOnboardingBatchWorkflow(request: { history_from: string; owner_confirmed: boolean; metadata_discovery_consent_confirmed: boolean; header_probe_consent_confirmed: boolean; }): Promise<SmartAccountsBrowserBatchWorkflowStatus> {
		if (!onboardingBatchID || !requireReadyBrowserRelay()) throw new Error('The signed-in Brave relay and a completed selected/all pairing batch are required.');
		const next = await api.prepareSmartAccountsBrowserOnboardingBatchWorkflow(onboardingBatchID, request);
		onboardingBatchWorkflow = next;
		return next;
	}

	async function resumeOnboardingBatchWorkflow(): Promise<SmartAccountsBrowserBatchWorkflowStatus> {
		if (!onboardingBatchID) throw new Error('A selected/all pairing batch is required.');
		const next = await api.resumeSmartAccountsBrowserOnboardingBatchWorkflow(onboardingBatchID);
		onboardingBatchWorkflow = next;
		return next;
	}

	async function advanceOnboardingBatchSafe(current: SmartAccountsBrowserBatchWorkflowStatus): Promise<SmartAccountsBrowserBatchWorkflowStatus> {
		if (!onboardingBatchID || current.workflow.batch_id !== onboardingBatchID) throw new Error('The batch workflow status is no longer current. Refresh it before continuing.');
		if (!requireReadyBrowserRelay()) throw new Error('The signed-in Brave relay must be ready before a metadata-only action can start.');
		const nextDiscovery = current.sources.find((source) => source.phase === 'DISCOVERY_REQUIRED');
		if (nextDiscovery) {
			const issued = await api.acquireSmartAccountsBrowserOnboardingBatchDiscovery(onboardingBatchID, {
				metadata_only_consent_confirmed: true,
				response_header_probe_confirmed: current.workflow.header_probe_consent_confirmed
			});
			if (issued.source.source_company_id !== nextDiscovery.source_company_id || issued.source.tenant_id !== nextDiscovery.tenant_id || !issued.source.lease_id || issued.source.phase !== 'DISCOVERY_RUNNING' || issued.discovery.source_company_id !== nextDiscovery.source_company_id || issued.discovery.tenant_id !== nextDiscovery.tenant_id) {
				throw new Error('The server returned a discovery action for a different company binding.');
			}
			batchDiscoveryActions.set(issued.discovery.discovery_id, {
				batchID: onboardingBatchID,
				sourceCompanyID: issued.source.source_company_id,
				leaseID: issued.source.lease_id,
				phaseGeneration: issued.source.phase_generation,
				discoveryID: issued.discovery.discovery_id
			});
			// The issue is forwarded once to relay memory and then discarded. This
			// component does not retain browser contracts, headers, rows, or tokens.
			window.postMessage(browserDiscoveryIssueEvent(issued.discovery), window.location.origin);
			return getOnboardingBatchWorkflow();
		}
		const discoveryComplete = current.sources.find((source) => source.phase === 'DISCOVERY_COMPLETE');
		if (discoveryComplete) {
			await api.requireSmartAccountsBrowserOnboardingBatchSchema(onboardingBatchID, discoveryComplete.source_company_id, { phase_generation: discoveryComplete.phase_generation });
			return getOnboardingBatchWorkflow();
		}
		if (current.sources.length > 0 && current.sources.every((source) => source.phase === 'SCHEMA_APPROVED')) {
			const next = await api.openSmartAccountsBrowserOnboardingBatchTransfer(onboardingBatchID);
			onboardingBatchWorkflow = next;
			return next;
		}
		const staged = current.sources.find((source) => source.phase === 'STAGED');
		if (staged) {
			await api.previewSmartAccountsBrowserOnboardingBatchSource(onboardingBatchID, staged.source_company_id, { phase_generation: staged.phase_generation, use_source_chart: true });
			return getOnboardingBatchWorkflow();
		}
		return current;
	}

	async function reissueOnboardingBatchDiscovery(source: SmartAccountsBrowserBatchWorkflowSource): Promise<SmartAccountsBrowserBatchWorkflowStatus> {
		if (!onboardingBatchID || !requireReadyBrowserRelay() || source.batch_id !== onboardingBatchID || source.phase !== 'DISCOVERY_RUNNING') {
			throw new Error('Refresh the safe workflow status before reissuing a discovery action.');
		}
		const current = onboardingBatchWorkflow;
		if (!current || current.workflow.batch_id !== onboardingBatchID) throw new Error('The selected/all batch workflow is no longer current.');
		const issued = await api.reissueSmartAccountsBrowserOnboardingBatchDiscovery(onboardingBatchID, source.source_company_id, {
			metadata_only_consent_confirmed: true,
			response_header_probe_confirmed: current.workflow.header_probe_consent_confirmed
		});
		if (issued.source.source_company_id !== source.source_company_id || issued.source.tenant_id !== source.tenant_id || issued.source.phase !== 'DISCOVERY_RUNNING' || !issued.source.lease_id || issued.source.phase_generation <= source.phase_generation || issued.discovery.source_company_id !== source.source_company_id || issued.discovery.tenant_id !== source.tenant_id) {
			throw new Error('The server returned a discovery reissue for a different company binding.');
		}
		for (const [discoveryID, action] of batchDiscoveryActions) {
			if (action.batchID === onboardingBatchID && action.sourceCompanyID === source.source_company_id) batchDiscoveryActions.delete(discoveryID);
		}
		batchDiscoveryActions.set(issued.discovery.discovery_id, {
			batchID: onboardingBatchID,
			sourceCompanyID: issued.source.source_company_id,
			leaseID: issued.source.lease_id,
			phaseGeneration: issued.source.phase_generation,
			discoveryID: issued.discovery.discovery_id
		});
		window.postMessage(browserDiscoveryIssueEvent(issued.discovery), window.location.origin);
		return getOnboardingBatchWorkflow();
	}

	async function confirmOnboardingBatchSchema(source: SmartAccountsBrowserBatchWorkflowSource): Promise<SmartAccountsBrowserBatchWorkflowStatus> {
		if (!onboardingBatchID || source.batch_id !== onboardingBatchID || source.phase !== 'SCHEMA_REVIEW_REQUIRED') throw new Error('Refresh the safe workflow status before confirming this schema.');
		await api.confirmSmartAccountsBrowserOnboardingBatchSchema(onboardingBatchID, source.source_company_id, {
			phase_generation: source.phase_generation,
			review_confirmed: true
		});
		return getOnboardingBatchWorkflow();
	}

	async function openOnboardingBatchTransfer(): Promise<SmartAccountsBrowserBatchWorkflowStatus> {
		if (!onboardingBatchID) throw new Error('A selected/all pairing batch is required.');
		const next = await api.openSmartAccountsBrowserOnboardingBatchTransfer(onboardingBatchID);
		onboardingBatchWorkflow = next;
		return next;
	}

	async function confirmOnboardingBatchTransfer(request: { owner_confirmed: boolean; expected_schema_sha256: string; }): Promise<SmartAccountsBrowserBatchWorkflowStatus> {
		if (!onboardingBatchID) throw new Error('A selected/all pairing batch is required.');
		const next = await api.confirmSmartAccountsBrowserOnboardingBatchTransfer(onboardingBatchID, request);
		onboardingBatchWorkflow = next;
		return next;
	}

	async function advanceOnboardingBatchConfirmedTransfer(current: SmartAccountsBrowserBatchWorkflowStatus): Promise<SmartAccountsBrowserBatchWorkflowStatus> {
		if (!onboardingBatchID || current.workflow.batch_id !== onboardingBatchID || !current.workflow.transfer_confirmed_at || !requireReadyBrowserRelay()) throw new Error('Confirm the immutable source transfer while the signed-in Brave relay is ready.');
		const nextCapture = current.sources.find((source) => source.phase === 'TRANSFER_CONFIRMATION_REQUIRED');
		if (!nextCapture) return current;
		const issued = await api.acquireSmartAccountsBrowserOnboardingBatchCapture(onboardingBatchID, { transfer_consent_confirmed: true });
		if (issued.source.source_company_id !== nextCapture.source_company_id || issued.source.tenant_id !== nextCapture.tenant_id || !issued.source.lease_id || issued.source.phase !== 'CAPTURE_RUNNING' || issued.capture.source_company_id !== nextCapture.source_company_id || issued.capture.tenant_id !== nextCapture.tenant_id) {
			throw new Error('The server returned a capture action for a different company binding.');
		}
		batchCaptureActions.set(issued.capture.run_id, {
			batchID: onboardingBatchID,
			sourceCompanyID: issued.source.source_company_id,
			leaseID: issued.source.lease_id,
			phaseGeneration: issued.source.phase_generation,
			runID: issued.capture.run_id
		});
		// Only the direct same-window relay event carries the one-time token.
		// It is not copied into component state, DOM, storage, logs, or URLs.
		sendBrowserCaptureIssue(issued.capture);
		return getOnboardingBatchWorkflow();
	}

	function validBatchDiscoveryRelayResult(value: unknown, action: BatchDiscoveryAction): value is SmartAccountsBrowserDiscoveryRelayResult {
		if (!value || typeof value !== 'object') return false;
		const result = value as Record<string, unknown>;
		const fields = ['source', 'type', 'version', 'discovery_id', 'manifest_version', 'contract_version', 'status', 'resources'];
		return Object.keys(result).length === fields.length && fields.every((field) => Object.hasOwn(result, field)) &&
			result.source === 'smartaccounts-browser-relay' && result.type === 'smartaccounts-browser-relay.discovery-result.v1' && result.version === 1 &&
			result.discovery_id === action.discoveryID && result.manifest_version === relayCaptureManifestVersion &&
			result.contract_version === 'smartaccounts-brave-discovery-contract-v1' && Array.isArray(result.resources);
	}

	async function completeOnboardingBatchDiscovery(action: BatchDiscoveryAction, result: SmartAccountsBrowserDiscoveryRelayResult) {
		try {
			await api.completeSmartAccountsBrowserOnboardingBatchDiscovery(action.batchID, action.sourceCompanyID, {
				lease_id: action.leaseID,
				phase_generation: action.phaseGeneration,
				discovery_id: action.discoveryID,
				result
			});
			batchDiscoveryActions.delete(action.discoveryID);
			const refreshed = await getOnboardingBatchWorkflow();
			onboardingBatchWorkflow = await advanceOnboardingBatchSafe(refreshed);
		} catch (caught) {
			error = messageFrom(caught, 'Could not record the server-bound browser discovery result for this company.');
		}
	}

	function clearOnboardingBatchCapturePoll(runID: string) {
		const timer = batchCapturePollTimers.get(runID);
		if (timer !== undefined) window.clearTimeout(timer);
		batchCapturePollTimers.delete(runID);
	}

	function scheduleOnboardingBatchCapturePoll(action: BatchCaptureAction) {
		clearOnboardingBatchCapturePoll(action.runID);
		batchCapturePollTimers.set(action.runID, window.setTimeout(() => {
			batchCapturePollTimers.delete(action.runID);
			void completeOnboardingBatchCapture(action);
		}, 5_000));
	}

	async function completeOnboardingBatchCapture(action: BatchCaptureAction) {
		try {
			const completion = await api.completeSmartAccountsBrowserOnboardingBatchCapture(action.batchID, action.sourceCompanyID, {
				lease_id: action.leaseID,
				phase_generation: action.phaseGeneration
			});
			const refreshed = await getOnboardingBatchWorkflow();
			if (completion.source.phase === 'CAPTURE_RUNNING') {
				// A finalized relay upload can still be compiling/staging in OA. Keep
				// this exact safe run pollable; do not claim it is staged or ready for
				// preview until the server advances the source checkpoint.
				onboardingBatchWorkflow = refreshed;
				scheduleOnboardingBatchCapturePoll(action);
				return;
			}
			batchCaptureActions.delete(action.runID);
			clearOnboardingBatchCapturePoll(action.runID);
			onboardingBatchWorkflow = await advanceOnboardingBatchSafe(refreshed);
		} catch (caught) {
			clearOnboardingBatchCapturePoll(action.runID);
			error = messageFrom(caught, 'Could not refresh the staged package receipt for this company. Resume the exact approved transfer if its short-lived relay authorization expired.');
		}
	}

	async function startFullHistoryCapture(resumeRunId = '') {
		const sourceCompanyId = status?.source_company_id?.trim();
		if (!sourceCompanyId || startingCapture) return;
		startingCapture = true;
		error = '';
		try {
			status = await api.requestSmartAccountsSyncDryRun(tenantId.trim(), sourceCompanyId, {
				scope_mode: 'full_history',
				...(resumeRunId ? { resume_run_id: resumeRunId } : {})
			});
			persistSourceCompanyId(status);
		} catch (caught) {
			error = messageFrom(caught, 'SmartAccounts is connected, but the full-history capture did not start.');
		} finally {
			startingCapture = false;
		}
	}

	async function startDateWindowCapture() {
		const sourceCompanyId = status?.source_company_id?.trim();
		if (!sourceCompanyId || startingCapture || !dateWindowFrom || !dateWindowTo || dateWindowTo < dateWindowFrom || (requiredWindowEnd && dateWindowTo < requiredWindowEnd) || dateWindowResources.length === 0) return;
		startingCapture = true;
		error = '';
		try {
			status = await api.requestSmartAccountsSyncDryRun(tenantId.trim(), sourceCompanyId, {
				scope_mode: 'window',
				date_from: dateWindowFrom,
				date_to: dateWindowTo,
				resource_ids: dateWindowResources
			});
			persistSourceCompanyId(status);
		} catch (caught) {
			error = messageFrom(caught, 'The required SmartAccounts date-window capture did not start.');
		} finally {
			startingCapture = false;
		}
	}

	async function resumeCapture(run: SmartAccountsSyncStatus['capture_progress']) {
		const sourceCompanyId = status?.source_company_id?.trim();
		if (!sourceCompanyId || !run || startingCapture) return;
		startingCapture = true;
		error = '';
		try {
			status = await api.requestSmartAccountsSyncDryRun(tenantId.trim(), sourceCompanyId, {
				scope_mode: run.scope_mode,
				...(run.date_from ? { date_from: run.date_from } : {}),
				...(run.date_to ? { date_to: run.date_to } : {}),
				...(run.resource_ids?.length ? { resource_ids: run.resource_ids } : {}),
				resume_run_id: run.run_id
			});
			persistSourceCompanyId(status);
		} catch (caught) {
			error = messageFrom(caught, 'The SmartAccounts capture could not resume.');
		} finally {
			startingCapture = false;
		}
	}

	async function refreshCaptureProgress() {
		const sourceCompanyId = status?.source_company_id?.trim();
		if (!sourceCompanyId) return;
		try {
			status = await api.getSmartAccountsSyncStatus(tenantId.trim(), sourceCompanyId);
			persistSourceCompanyId(status);
		} catch {
			// Progress polling is advisory. Preserve the last safe server result and
			// avoid replacing it with a transport error while capture continues.
		}
	}

	$effect(() => {
		if (!captureRuns.some((run) => run.status === 'running') || !status?.source_company_id) return;
		const timer = window.setInterval(() => void refreshCaptureProgress(), 5000);
		return () => window.clearInterval(timer);
	});

	onMount(() => {
		relayListenerMounted = true;
		const onPairingResult = (event: MessageEvent<unknown>) => {
			if (event.source !== window || event.origin !== window.location.origin) return;
			const data = event.data as { source?: unknown; type?: unknown; pairing_id?: unknown; run_id?: unknown; discovery_id?: unknown; status?: unknown; state?: unknown; receipt?: unknown } | null;
			if (data?.source !== 'smartaccounts-browser-relay') return;
			if (data.type === 'smartaccounts-browser-relay.readiness.v1') {
				receiveRelayReadiness(data);
				return;
			}
			if (data.type === 'smartaccounts-browser-relay.source-catalog-result.v1') {
				if (!validBrowserCatalogResult(data) || data.catalog_id !== onboardingCatalogReceiptID || data.workflow_id !== onboardingCatalogWorkflowID || data.nonce !== onboardingCatalogNonce) return;
				const catalog = data;
				if (catalog.status !== 'accepted' && catalog.status !== 'already_accepted') {
					if (catalog.status === 'awaiting_browser') {
						onboardingMessage = 'The relay is waiting for the visible SmartAccounts company picker.';
						return;
					}
					sourceDiscoveryPending = false;
					onboardingCatalogWorkflowID = '';
					onboardingCatalogNonce = '';
					error = catalog.status === 'expired'
						? 'The visible-company catalog capability expired. Confirm again to issue a new, bounded catalog handoff.'
						: 'The Brave relay could not hand off the visible SmartAccounts company catalog.';
					return;
				}
				sourceDiscoveryPending = false;
				const acceptedGeneration = onboardingCatalogGeneration;
				const acceptedCatalogID = catalog.catalog_id;
				const acceptedWorkflowID = catalog.workflow_id;
				onboardingCatalogWorkflowID = '';
				onboardingCatalogNonce = '';
				void (async () => {
					try {
						const receipt = await api.getSmartAccountsBrowserOnboardingCatalog(onboardingCatalogReceiptID);
						if (onboardingCatalogGeneration !== acceptedGeneration || onboardingCatalogReceiptID !== acceptedCatalogID) return;
						if (receipt.catalog_id !== acceptedCatalogID || receipt.workflow_id !== acceptedWorkflowID || receipt.catalog_count !== catalog.catalog_count || receipt.catalog_sha256 !== catalog.catalog_sha256) throw new Error('catalog receipt mismatch');
						const companies = sourceCompaniesFromBrowserMessage(receipt.companies.map((company) => ({ source_company_id: company.source_company_id, source_company_name: company.display_name })));
						if (companies.length !== receipt.catalog_count) throw new Error('invalid receipt');
						if (onboardingCatalogGeneration !== acceptedGeneration || onboardingCatalogReceiptID !== acceptedCatalogID) return;
						discoveredSourceCompanies = companies;
						selectedSourceCompanyIDs = [];
						onboardingMode = '';
						onboardingBatchConsent = false;
						onboardingMessage = `The relay-observed catalog has ${companies.length} company option${companies.length === 1 ? '' : 's'}. Choose All or a strict subset; nothing is selected implicitly.`;
					} catch (caught) {
						error = messageFrom(caught, 'The visible-company catalog was accepted but could not be loaded for selection.');
					}
				})();
				return;
			}
			if (data.type === 'smartaccounts-browser-relay.discovery-result.v1') {
				const discoveryID = typeof data.discovery_id === 'string' ? data.discovery_id : '';
				const batchAction = batchDiscoveryActions.get(discoveryID);
				if (batchAction && validBatchDiscoveryRelayResult(data, batchAction)) {
					void completeOnboardingBatchDiscovery(batchAction, data);
					return;
				}
				if (validBrowserDiscoveryRelayResult(data)) void receiveBraveBrowserDiscoveryResult(data);
				return;
			}
			if ((data.type === 'smartaccounts-browser-relay.capture-result.v1' || data.type === 'smartaccounts-browser-relay.workflow-state.v1') && typeof data.run_id === 'string') {
				const batchAction = batchCaptureActions.get(data.run_id);
				if (batchAction) {
					// A resume-required notification retains the exact same safe run
					// checkpoint; it must not be marked complete until the relay emits
					// the terminal capture result and OA verifies the staged receipt.
					if (data.type === 'smartaccounts-browser-relay.capture-result.v1') void completeOnboardingBatchCapture(batchAction);
					return;
				}
				if (data.run_id === browserCaptureRunID) {
					browserCaptureMessage = data.type === 'smartaccounts-browser-relay.workflow-state.v1' && data.state === 'resume_required'
						? 'The Brave capability expired. Confirm transfer again to resume this same run.'
						: 'Brave capture progress was updated. Refreshing the owner-safe status; source rows remain in the relay.';
					if (browserCaptureWorkflow) void refreshBraveBrowserCaptureWorkflowStatus(browserCaptureWorkflow.workflow_id);
					else void refreshBraveBrowserCaptureStatus(data.run_id);
				}
				return;
			}
			if (data.type !== 'smartaccounts-browser-relay.pairing-result.v1' || typeof data.pairing_id !== 'string') return;
			const binding = onboardingBindings.find((candidate) => candidate.pairingID === data.pairing_id);
			if (binding) {
				const nextStatus: BrowserOnboardingBinding['status'] = data.status === 'claimed' ? 'claimed' : 'failed';
				const nextBindings: BrowserOnboardingBinding[] = onboardingBindings.map((candidate) => candidate.pairingID === data.pairing_id ? { ...candidate, status: nextStatus } : candidate);
				onboardingBindings = nextBindings;
				if (nextBindings.length > 0 && nextBindings.every((candidate) => candidate.status === 'claimed')) {
					onboardingMessage = 'Every selected SmartAccounts company is bound to its own Open Accounting tenant. Loading the current safe workflow automatically.';
					// A terminal relay claim is only a local progress signal. The server
					// owns the immutable batch state, so automatically re-read that safe
					// state once and load its workflow only after it says READY.
					onboardingBatchRefreshAttempts = 0;
					clearOnboardingBatchRefresh();
					void refreshOnboardingBatch();
				} else {
					onboardingMessage = 'Brave is continuing the selected-company pairing queue.';
				}
				return;
			}
			if (data.pairing_id === pairingID && data.status === 'claimed') void refreshBrowserPairing();
		};
		window.addEventListener('message', onPairingResult);
		if (!accountantReviewRequested && !ownerContinuationRequested) requestRelayReadiness();
		return () => {
			relayListenerMounted = false;
			clearRelayReadinessTimer();
			clearOnboardingBatchRefresh();
			for (const timer of batchCapturePollTimers.values()) window.clearTimeout(timer);
			batchCapturePollTimers.clear();
			batchDiscoveryActions.clear();
			batchCaptureActions.clear();
			window.removeEventListener('message', onPairingResult);
		};
	});

	async function preparePreview(run: SmartAccountsSyncStatus['capture_progress']) {
		const staging = run?.staging;
		if (!staging || staging.status !== 'staged_review_required' || !staging.finalized || run.scope_mode !== 'full_history' || coverageBlocked) return;
		preparingPreview = true;
		error = '';
		try {
			preview = await api.previewSmartAccountsPackage(tenantId.trim(), staging.package_id, { use_source_chart: true });
			previewPackageID = staging.package_id;
			applyConfirmed = false;
		}
		catch (caught) { error = messageFrom(caught, 'Could not prepare the SmartAccounts preview.'); }
		finally { preparingPreview = false; }
	}

	async function prepareBrowserCapturePreview() {
		const staging = browserCaptureStatus?.staging;
		if (!browserCaptureStatus || browserCaptureStatus.scope.mode !== 'partial' || browserCaptureStatus.receipt?.status !== 'partial_coverage_recorded' || !staging || staging.status !== 'staged_review_required' || !staging.finalized) return;
		preparingPreview = true;
		error = '';
		try {
			preview = await api.previewSmartAccountsPackage(tenantId.trim(), staging.package_id, { use_source_chart: true });
			previewPackageID = staging.package_id;
			applyConfirmed = false;
		} catch (caught) {
			error = messageFrom(caught, 'Could not prepare the partial browser package preview.');
		} finally {
			preparingPreview = false;
		}
	}

	async function queueBrowserCapturePreview(next: SmartAccountsBrowserCaptureStatus) {
		const staging = next.staging;
		if (next.scope.mode !== 'partial' || next.receipt?.status !== 'partial_coverage_recorded' || !staging || staging.status !== 'staged_review_required' || !staging.finalized) return;
		const key = `${staging.package_id}:${staging.package_sha256}`;
		if (browserPreviewAutoRequestedKey === key || (previewPackageID === staging.package_id && preview)) return;
		browserPreviewAutoRequestedKey = key;
		await prepareBrowserCapturePreview();
	}

	async function applyPreview() {
		if (!preview || !applyConfirmed || preview.status !== 'PREVIEW_READY') return;
		const continuedSource = ownerContinuationSource;
		const sourceCompanyID = continuedSource?.source_company_id ?? status?.source_company_id?.trim() ?? '';
		if (continuedSource && (!ownerContinuationSourceReady(continuedSource) || preview.id !== continuedSource.preview_id || preview.package_id !== continuedSource.package_id || preview.preview_sha256 !== continuedSource.preview_sha256 || preview.source_company_id !== continuedSource.source_company_id)) {
			error = 'Financial GL apply stopped because the selected/all continuation preview no longer matches its server-bound tenant/source checkpoint.';
			return;
		}
		if (!/^sa-browser-v1-\d{1,20}$/.test(sourceCompanyID)) {
			error = 'Financial GL apply requires the exact paired SmartAccounts source binding.';
			return;
		}
		applyingPreview = true;
		error = '';
		try {
			const policy = await api.resolveSmartAccountsTolerancePolicy(tenantId.trim(), sourceCompanyID, {
				package_id: preview.package_id,
				preview_id: preview.id
			});
			preview = await api.applySmartAccountsPackage(tenantId.trim(), {
				confirm: true,
				preview_id: preview.id,
				preview_sha256: preview.preview_sha256,
				tolerance_policy_id: policy.policy_id
			});
		}
		catch (caught) { error = messageFrom(caught, 'Financial GL apply needs a current independently approved accountant policy for this exact preview.'); }
		finally { applyingPreview = false; }
	}

	async function prepareReferencePreview(packageID: string, entityTypes: Array<'account' | 'customer' | 'vendor' | 'item'> = []) {
		if (!packageID || preparingReferencePreview || applyingReferencePreview) return;
		preparingReferencePreview = true;
		error = '';
		try {
			referencePreview = await api.previewSmartAccountsReferenceMasters(tenantId.trim(), packageID, entityTypes.length ? { entity_types: entityTypes } : {});
			referencePreviewPackageID = packageID;
			referenceApplyConfirmed = false;
		} catch (caught) {
			error = messageFrom(caught, 'Could not prepare the non-financial SmartAccounts master preview.');
		} finally {
			preparingReferencePreview = false;
		}
	}

	async function applyReferencePreview() {
		if (!referencePreview || !referenceApplyConfirmed || referencePreview.status !== 'PREVIEW_READY') return;
		applyingReferencePreview = true;
		error = '';
		try {
			referencePreview = await api.applySmartAccountsReferenceMasters(tenantId.trim(), {
				confirm: true,
				preview_id: referencePreview.id,
				preview_sha256: referencePreview.preview_sha256
			});
		} catch (caught) {
			error = messageFrom(caught, 'Reference-master apply needs review.');
		} finally {
			applyingReferencePreview = false;
		}
	}

	function messageFrom(caught: unknown, fallback: string): string {
		return caught instanceof Error && caught.message ? caught.message : fallback;
	}
</script>

<section class="smartaccounts-control card panel" aria-labelledby="smartaccounts-sync-heading">
	<div class="panel-heading">
		<div>
			<p class="eyebrow">SmartAccounts</p>
			<h2 id="smartaccounts-sync-heading">Guided sync preparation</h2>
		</div>
		<span class="status-pill">Read-only capture</span>
	</div>
	<p class="intro">
		{#if hasTenant}
			Connect the signed-in SmartAccounts Brave session to this tenant without copying an API key. The pairing binds only the selected opaque source identifier; SmartAccounts remains GL-authoritative, while invoices and payments remain non-posting.
		{:else}
			Read the visible SmartAccounts company catalog, choose an explicit selected/all scope, then create or reuse isolated Open Accounting tenants. No records, credentials, cookies, capture, or financial write occurs in this step.
		{/if}
	</p>

	{#if error}
		<div class="alert alert-error" role="alert">{error}</div>
	{/if}

	{#if accountantReviewRequested}
		<SmartAccountsAccountantReconciliationReview
			tenantId={tenantId.trim()}
			batchId={accountantReviewBatchId.trim()}
			sourceCompanyId={accountantReviewSourceCompanyId.trim()}
		/>
	{/if}

	{#if ownerContinuationRequested}
		<div class="package-review" aria-labelledby="smartaccounts-owner-continuation-heading" aria-live="polite">
			<h3 id="smartaccounts-owner-continuation-heading">Selected/all source review and GL apply</h3>
			<p class="help">This continuation contains only the tenant, immutable batch, and opaque source IDs. Open Accounting re-reads the current owner-safe workflow before allowing any review or apply action.</p>
			{#if ownerContinuationLoading}
				<p>Loading the server-bound source checkpoint…</p>
			{:else if ownerContinuationSource}
				<p><strong>{ownerContinuationSource.phase.replaceAll('_', ' ').toLowerCase()}</strong> — {ownerContinuationMessage}</p>
				{#if ownerContinuationSourceReady(ownerContinuationSource)}
					{#if ownerContinuationCoverage}
						<p class="help">Count-only archive coverage: {ownerContinuationCoverage.observed_record_count}/{ownerContinuationCoverage.declared_record_count} records, {ownerContinuationCoverage.artifact_count} artifacts, {ownerContinuationCoverage.review_required_record_count} review-required, {ownerContinuationCoverage.unconsumed_record_count} unconsumed; integrity {ownerContinuationCoverage.integrity_ok ? 'verified' : 'needs review'}.</p>
					{:else if ownerContinuationCoverageMessage}
						<p class="help">{ownerContinuationCoverageMessage}</p>
					{/if}
					{#if !preview || previewPackageID !== ownerContinuationSource.package_id}
						<button class="btn btn-secondary" type="button" disabled={ownerContinuationPreviewLoading || applyingPreview} onclick={() => void loadOwnerContinuationPreview()}>{ownerContinuationPreviewLoading ? 'Loading server-bound GL preview…' : 'Load current GL preview for review'}</button>
					{:else}
						<p><strong>{preview.status}</strong>: {preview.journals?.length ?? 0} journals, {preview.account_imports?.length ?? 0} chart proposals, {preview.non_posting_record_count} non-posting archive records.</p>
						{#if preview.issues?.length}<ul>{#each preview.issues as issue (issue.code)}<li>{issue.code}: {issue.message}</li>{/each}</ul>{/if}
						{#if preview.status === 'PREVIEW_READY'}
							<label class="confirm"><input type="checkbox" bind:checked={applyConfirmed} /> I reviewed this server-bound partial, GL-authoritative plan and want to create and post these journals once. Accountant policy approval remains separate.</label>
							<button class="btn btn-primary" type="button" disabled={!applyConfirmed || applyingPreview} onclick={() => void applyPreview()}>{applyingPreview ? 'Applying…' : 'Confirm and apply reviewed GL plan'}</button>
						{/if}
					{/if}
				{:else}
					<p class="help">This source has no current server-bound preview. Return to the selected/all runner and complete the blocked or pending phase there.</p>
				{/if}
			{:else if ownerContinuationMessage}
				<p class="help">{ownerContinuationMessage}</p>
			{/if}
		</div>
	{/if}

	{#if !accountantReviewRequested && !ownerContinuationRequested}
	{#if hasTenant}
		<div class="form-grid">
			<div class="form-group">
				<p class="label">Source company</p>
				<p class="input read-only" aria-live="polite">
					{status?.configured ? (status.source_company_name ?? 'Verified SmartAccounts source') : 'Derived from the signed-in Brave session'}
				</p>
				<p class="help">The browser relay shares no API key, cookie, or ledger data at pairing time.</p>
			</div>
		</div>
	{/if}

	<div class="package-review" aria-live="polite" aria-label="Brave Browser Relay readiness">
		<p><strong>{relayReadinessMessage}</strong></p>
		<p class="help">This zero-data check confirms only the installed relay protocol and whether the visible SmartAccounts session is signed in. It never returns a company, credential, cookie, or source record.</p>
		<button class="btn btn-secondary" type="button" disabled={relayReadiness === 'checking'} onclick={requestRelayReadiness}>
			{relayReadiness === 'checking' ? 'Checking Browser Relay…' : 'Check Browser Relay again'}
		</button>
	</div>

	{#if !accountantReviewRequested && !ownerContinuationRequested}
	<div class="actions">
		{#if hasTenant}
			<button class="btn btn-primary" type="button" disabled={!browserRelayReady || saving || pairingPending} onclick={() => void connectWithBrave()}>
				{saving ? 'Starting Brave pairing…' : pairingPending ? 'Waiting for Brave…' : 'Connect with Brave (no API key)'}
			</button>
		{/if}
		<button class="btn btn-secondary" type="button" disabled={!browserRelayReady || sourceDiscoveryPending || onboardingBusy || !onboardingCatalogConsent} onclick={() => void requestBrowserSourceDiscovery()}>
			{sourceDiscoveryPending ? 'Reading visible company catalog…' : 'Read visible SmartAccounts company catalog'}
		</button>
	</div>
	<label class="company-choice"><input type="checkbox" bind:checked={onboardingCatalogConsent} /> I approve this metadata-only read of the visible SmartAccounts company picker. It transfers no records, cookies, credentials, or financial data.</label>
	{/if}
	{#if pairingMessage}
		<p class="help" aria-live="polite">{pairingMessage}</p>
	{/if}
	{#if /^sa-browser-v1-\d{1,20}$/.test(status?.source_company_id ?? '')}
		<div class="package-review" aria-labelledby="smartaccounts-browser-discovery-heading">
			<h3 id="smartaccounts-browser-discovery-heading">Discover browser contract coverage</h3>
			<p>Inspect the server-derived 31-surface SmartAccounts browser manifest for this paired source. This is metadata-only: it never transfers source rows, cookies, credentials, CSV bodies, or accounting entries. Journal entries alone never represent complete full-sync discovery.</p>
			<label class="company-choice"><input type="checkbox" bind:checked={browserDiscoveryMetadataConsent} /> I approve this metadata-only browser discovery for this tenant and paired source.</label>
			<label class="company-choice"><input type="checkbox" disabled={!browserDiscoveryMetadataConsent} bind:checked={browserDiscoveryHeaderProbeConsent} /> I separately approve bounded CSV header-name probing where the relay supports it. No header values or CSV body are retained.</label>
			<button class="btn btn-secondary" type="button" disabled={!browserRelayReady || browserDiscoveryPending || !browserDiscoveryMetadataConsent} onclick={() => void startBraveBrowserDiscovery()}>{browserDiscoveryPending ? 'Authorizing discovery…' : 'Discover browser contracts'}</button>
			{#if browserDiscoveryID}
				<button class="btn btn-secondary" type="button" disabled={browserDiscoveryPending} onclick={() => void refreshBraveBrowserDiscoveryReceipt()}>Refresh safe discovery status</button>
			{/if}
			{#if browserDiscoveryMessage}<p class="help" aria-live="polite">{browserDiscoveryMessage}</p>{/if}
			{#if browserDiscoveryReceipt}
				<div class="package-review" aria-live="polite">
					<h3>Redacted discovery receipt</h3>
					<p><strong>{browserDiscoveryReceipt.status}</strong>: {browserDiscoveryReceipt.resource_count}/31 surfaces recorded — {browserDiscoveryReceipt.capture_ready_count} capture-ready, {browserDiscoveryReceipt.filter_contract_required_count} filter review, {browserDiscoveryReceipt.page_only_contract_required_count} page-only, {browserDiscoveryReceipt.private_endpoint_required_count} private-endpoint, and {browserDiscoveryReceipt.binding_blocked_count} binding-blocked.</p>
					<p class="help">Receipt digest: {browserDiscoveryReceipt.contract_sha256}. This integrity handle is not source data or permission to capture or post.</p>
					{#if browserDiscoveryReceipt.status === 'completed' && browserDiscoveryReceipt.capture_ready_count > 0}
						<label class="company-choice"><input type="checkbox" bind:checked={browserCSVSchemaReviewConfirmed} /> I confirm the reviewed General Ledger CSV schema for this discovery. The journal summary grid remains archive-only evidence.</label>
						<button class="btn btn-secondary" type="button" disabled={browserCSVSchemaReviewPending || !browserCSVSchemaReviewConfirmed} onclick={() => void reviewGeneralLedgerCSVSchema()}>{browserCSVSchemaReviewPending ? 'Registering reviewed schema…' : 'Register reviewed General Ledger CSV schema'}</button>
						{#if browserCSVSchemaReview}
							<p class="help" aria-live="polite">Schema adapter <strong>{browserCSVSchemaReview.status}</strong>; approval digest: {browserCSVSchemaReview.approval_sha256}. This is not permission to capture or post.</p>
						{/if}
					{/if}
				</div>
			{/if}
		</div>
		<div class="package-review" aria-labelledby="smartaccounts-browser-capture-heading">
			<h3 id="smartaccounts-browser-capture-heading">Capture General Ledger from Brave</h3>
			<p>Open Accounting derives the end date and cutoff when you start. v2 transfers only the reviewed <code>general_ledger</code> CSV source as an explicitly partial, review-gated capture; it cannot post accounting entries. The <code>journal_entries</code> summary grid remains archive-only evidence.</p>
			<div class="form-grid">
				<div class="form-group"><label class="label" for="smartaccounts-browser-from">History starts</label><input id="smartaccounts-browser-from" class="input" type="date" bind:value={browserCaptureFrom} /></div>
			</div>
			<label class="company-choice"><input type="checkbox" bind:checked={browserCaptureConsent} /> I confirm this partial General Ledger CSV transfer to this tenant and source. SmartAccounts remains GL-authoritative; package review and any financial apply stay explicit.</label>
			<button class="btn btn-secondary" type="button" disabled={!browserRelayReady || browserCapturePending || !browserCaptureFrom || !browserCaptureConsent} onclick={() => void startBraveBrowserCaptureWorkflow()}>{browserCapturePending ? 'Starting General Ledger capture…' : 'Start General Ledger CSV capture'}</button>
			{#if browserCaptureRunID}
				<label class="company-choice"><input type="checkbox" bind:checked={browserCaptureResumeConsent} /> I confirm transfer for the same tenant, source, run, and scope again.</label>
				<button class="btn btn-secondary" type="button" disabled={!browserRelayReady || browserCapturePending || !browserCaptureResumeConsent} onclick={() => void resumeBraveBrowserCapture()}>Resume same Brave capture</button>
				<button class="btn btn-secondary" type="button" disabled={browserCapturePending} onclick={() => void refreshBraveBrowserCaptureWorkflowStatus()}>Refresh safe capture status</button>
			{/if}
			{#if browserCaptureMessage}<p class="help" aria-live="polite">{browserCaptureMessage}</p>{/if}
			{#if browserCaptureStatus}
				<div class="package-review" aria-live="polite">
					<h3>Brave capture status</h3>
					<p><strong>Scope: {browserCaptureStatus.scope.mode === 'partial' ? 'partial browser export' : 'full browser export'}</strong> — {browserCaptureStatus.status}. This scope is immutable and is not automatically treated as all history.</p>
					{#if browserCaptureStatus.receipt}
						<p>Coverage receipt: {browserCaptureStatus.receipt.status} ({browserCaptureStatus.receipt.completed_export_count}/{browserCaptureStatus.receipt.required_export_count} selected CSV exports complete{browserCaptureStatus.receipt.blocked_page_only_count ? `; ${browserCaptureStatus.receipt.blocked_page_only_count} page-only blockers` : ''}).</p>
					{/if}
					{#if browserCaptureStatus.staging?.status === 'review_required'}
						<p class="help">The bridge marked this partial CSV schema or journal for review. No package preview or accounting apply is available.</p>
					{:else if browserCaptureStatus.staging?.status === 'staged_review_required' && browserCaptureStatus.staging.finalized}
						<p>Partial browser package {browserCaptureStatus.staging.package_id} is staged ({browserCaptureStatus.staging.record_chunks_acknowledged} record and {browserCaptureStatus.staging.artifact_chunks_acknowledged} artifact chunks acknowledged). Review is explicit; invoices and payments remain non-posting evidence.</p>
						<button class="btn btn-primary" type="button" disabled={preparingPreview || applyingPreview} onclick={() => void prepareBrowserCapturePreview()}>{preparingPreview ? 'Preparing preview…' : 'Prepare partial package preview'}</button>
						{#if preview && previewPackageID === browserCaptureStatus.staging.package_id}
							<p><strong>{preview.status}</strong>: {preview.journals?.length ?? 0} journals, {preview.account_imports?.length ?? 0} chart proposals, {preview.non_posting_record_count} non-posting archive records.</p>
							{#if preview.issues?.length}<ul>{#each preview.issues as issue (issue.code)}<li>{issue.code}: {issue.message}</li>{/each}</ul>{/if}
							{#if preview.status === 'PREVIEW_READY'}
								<label class="confirm"><input type="checkbox" bind:checked={applyConfirmed} /> I reviewed this partial, GL-authoritative plan and want to create and post these journals once.</label>
								<button class="btn btn-primary" type="button" disabled={!applyConfirmed || applyingPreview} onclick={() => void applyPreview()}>{applyingPreview ? 'Applying…' : 'Confirm and apply reviewed GL plan'}</button>
							{/if}
						{/if}
						<button class="btn btn-secondary" type="button" disabled={preparingReferencePreview || applyingReferencePreview} onclick={() => void prepareReferencePreview(browserCaptureStatus?.staging?.package_id ?? '')}>{preparingReferencePreview ? 'Preparing reference masters…' : 'Prepare non-financial master preview'}</button>
						{#if referencePreview && referencePreviewPackageID === browserCaptureStatus?.staging?.package_id}
							<p><strong>{referencePreview.status}</strong>: {referencePreview.actions?.length ?? 0} account, contact, or item actions. This preview never posts a journal, invoice, or payment.</p>
							<ul class="resource-progress">{#each referencePreview.reconciliation as line (line.entity_type)}<li>{line.entity_type}: {line.source_records} source, {line.create_planned} planned, {line.already_applied} already applied, {line.review_required} review{line.tombstones ? ` (${line.tombstones} tombstones)` : ''}</li>{/each}</ul>
							{#if referencePreview.issues?.length}<ul>{#each referencePreview.issues as issue (issue.code)}<li>{issue.code}: {issue.message}</li>{/each}</ul>{/if}
							{#if referencePreview.status === 'PREVIEW_READY'}
								<label class="confirm"><input type="checkbox" bind:checked={referenceApplyConfirmed} /> I reviewed this non-financial account, contact, and item master plan. No financial posting will be made.</label>
								<button class="btn btn-primary" type="button" disabled={!referenceApplyConfirmed || applyingReferencePreview} onclick={() => void applyReferencePreview()}>{applyingReferencePreview ? 'Applying reference masters…' : 'Confirm and apply reference masters'}</button>
							{/if}
						{/if}
					{:else if browserCaptureStatus.staging}
						<p class="help">Package handoff: {browserCaptureStatus.staging.status}. The bridge will expose a package review only after its checksum-verified staging finalizes.</p>
					{/if}
				</div>
			{/if}
		</div>
		<div class="package-review" aria-labelledby="smartaccounts-browser-master-detail-heading">
			<h3 id="smartaccounts-browser-master-detail-heading">Current client and vendor snapshots</h3>
			<p>One owner confirmation queues reviewed current snapshots for clients, vendors, and articles. Each resource remains separately resumable and tenant/source bound. This never posts a journal, invoice, or payment. Articles are deliberately review-only until an explicit VAT-rate mapping exists.</p>
			<label class="company-choice"><input type="checkbox" bind:checked={masterDetailTransferConsent} /> I authorize this tenant’s paired source to transfer the reviewed current client/vendor/article snapshots. I understand article records cannot be applied yet and contact apply needs a separate final confirmation.</label>
			<button class="btn btn-secondary" type="button" disabled={!browserRelayReady || masterDetailPending || !masterDetailTransferConsent} onclick={() => void authorizeMasterDetails()}>{masterDetailPending ? 'Authorizing master snapshots…' : 'Authorize current master snapshots'}</button>
			{#if masterDetailBatchID}
				<button class="btn btn-secondary" type="button" disabled={masterDetailPending} onclick={() => void refreshMasterDetailStatuses()}>{masterDetailPending ? 'Refreshing…' : 'Refresh safe master-data status'}</button>
			{/if}
			{#if masterDetailMessage}<p class="help" aria-live="polite">{masterDetailMessage}</p>{/if}
			{#if masterDetailStatuses.length > 0}
				<ul class="resource-progress" aria-label="Master-detail snapshot status">
					{#each masterDetailStatuses as master (master.run_id)}
						<li>
							<strong>{master.resource_id}</strong>: {master.status} — snapshot date {master.snapshot_date}.
							{#if master.status === 'STAGED_REVIEW_REQUIRED' && master.package_id && (master.resource_id === 'clients' || master.resource_id === 'vendors')}
								<p class="help">OA verified the exact tenant/source/package digest as staged. Prepare a separate non-financial {master.resource_id === 'clients' ? 'customer' : 'vendor'} preview; no finance is affected.</p>
								<button class="btn btn-secondary" type="button" disabled={preparingReferencePreview || applyingReferencePreview} onclick={() => void prepareReferencePreview(master.package_id!, [master.resource_id === 'clients' ? 'customer' : 'vendor'])}>{preparingReferencePreview ? 'Preparing reference preview…' : `Prepare ${master.resource_id} reference preview`}</button>
								{#if referencePreview && referencePreviewPackageID === master.package_id}
									<p><strong>{referencePreview.status}</strong>: {referencePreview.actions?.length ?? 0} deterministic contact actions. Canonical rows remain archive-only.</p>
									<ul class="resource-progress">{#each referencePreview.reconciliation as line (line.entity_type)}<li>{line.entity_type}: {line.source_records} source, {line.create_planned} planned, {line.already_applied} already applied, {line.review_required} review{line.tombstones ? ` (${line.tombstones} tombstones)` : ''}</li>{/each}</ul>
									{#if referencePreview.issues?.length}<ul>{#each referencePreview.issues as issue (issue.code)}<li>{issue.code}: {issue.message}</li>{/each}</ul>{/if}
									{#if referencePreview.status === 'PREVIEW_READY'}
										<label class="confirm"><input type="checkbox" bind:checked={referenceApplyConfirmed} /> I reviewed this non-financial contact plan and want to create only its exact contacts. No financial posting will be made.</label>
										<button class="btn btn-primary" type="button" disabled={!referenceApplyConfirmed || applyingReferencePreview} onclick={() => void applyReferencePreview()}>{applyingReferencePreview ? 'Applying contacts…' : 'Confirm and apply contacts'}</button>
									{/if}
								{/if}
							{:else if master.resource_id === 'articles'}
								<p class="help">Articles remain archival evidence/review-only: no proven browser VAT-rate mapping exists, so OA cannot infer or default a VAT rate.</p>
							{:else if master.status === 'finalized_archived_evidence'}
								<p class="help">The bridge sealed archive evidence. OA will not preview it until its own receiver verifies this exact tenant/source/package digest as staged.</p>
							{/if}
							{#if master.status === 'open'}
								<label class="confirm"><input type="checkbox" bind:checked={masterDetailResumeConsent[master.run_id]} /> I reconfirm this exact tenant, source, resource, and current-snapshot scope before resuming.</label>
								<button class="btn btn-secondary" type="button" disabled={masterDetailPending || !browserRelayReady || !masterDetailResumeConsent[master.run_id]} onclick={() => void resumeMasterDetail(master)}>{masterDetailPending ? 'Resuming…' : `Resume ${master.resource_id} snapshot`}</button>
							{/if}
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	{/if}
	{#if onboardingMessage}
		<p class="help" aria-live="polite">{onboardingMessage}</p>
	{/if}

	{#if discoveredSourceCompanies.length > 0}
		<div class="package-review" aria-labelledby="smartaccounts-company-onboarding-heading">
			<h3 id="smartaccounts-company-onboarding-heading">Create or reuse isolated company tenants</h3>
			<p>Choose an explicit scope from the relay-observed catalog. The server records an immutable owner-confirmed batch, creates only missing exact-name tenants, and returns expected-source pairings. This stops at paired/review-ready: per-source discovery and reviewed CSV schema approval must happen before any partial capture. It never posts financial data.</p>
			<div class="company-picker" role="radiogroup" aria-label="SmartAccounts company onboarding scope">
				<label class="company-choice">
					<input type="radio" name="smartaccounts-onboarding-mode" value="all" checked={onboardingMode === 'all'} onchange={() => { onboardingMode = 'all'; selectedSourceCompanyIDs = discoveredSourceCompanies.map((company) => company.source_company_id); }} />
					<span>All {discoveredSourceCompanies.length} relay-observed companies</span>
				</label>
				<label class="company-choice">
					<input type="radio" name="smartaccounts-onboarding-mode" value="selected" disabled={discoveredSourceCompanies.length === 1} checked={onboardingMode === 'selected'} onchange={() => { onboardingMode = 'selected'; selectedSourceCompanyIDs = []; }} />
					<span>Choose a strict subset</span>
				</label>
			</div>
			{#if onboardingMode === 'selected'}
				<div class="company-picker" role="group" aria-label="SmartAccounts companies to onboard">
					{#each discoveredSourceCompanies as company (company.source_company_id)}
						<label class="company-choice">
							<input type="checkbox" checked={selectedSourceCompanyIDs.includes(company.source_company_id)} onchange={(event) => toggleSourceCompany(company.source_company_id, (event.currentTarget as HTMLInputElement).checked)} />
							<span>{company.source_company_name}</span>
						</label>
					{/each}
				</div>
			{/if}
			<label class="company-choice"><input type="checkbox" bind:checked={onboardingBatchConsent} /> As owner, I authorize only the selected/all tenant create-or-reuse and expected-source pairing action. I understand capture and any financial apply require separate later confirmation.</label>
			<button class="btn btn-primary" type="button" disabled={!browserRelayReady || onboardingBusy || !onboardingMode || !onboardingBatchConsent || (onboardingMode === 'selected' && (selectedSourceCompanyIDs.length === 0 || selectedSourceCompanyIDs.length >= discoveredSourceCompanies.length))} onclick={() => void onboardSelectedSourceCompanies()}>
				{onboardingBusy ? 'Creating tenants and pairing…' : onboardingMode === 'all' ? `Create/reuse and pair all ${discoveredSourceCompanies.length} companies` : `Create/reuse and pair ${selectedSourceCompanyIDs.length} selected ${selectedSourceCompanyIDs.length === 1 ? 'company' : 'companies'}`}
			</button>
		</div>
	{/if}

	{#if onboardingBatchID}
		<div class="package-review" aria-labelledby="smartaccounts-restored-batch-heading">
			<h3 id="smartaccounts-restored-batch-heading">Selected/all company batch</h3>
			<p class="help">This is a durable server-owned batch checkpoint. Reloading restores only its opaque ID and safe owner status; browser capabilities, catalog entries, credentials, and source records are never stored.</p>
			{#if onboardingBatchStatus === 'PENDING' || onboardingBatchStatus === 'REVIEW_REQUIRED'}
				<p class="help">Pairing progress refreshes automatically for a bounded interval after a relay action or restore. If Brave was interrupted, reconfirm before continuing this exact server-owned pairing batch.</p>
				<label class="company-choice"><input type="checkbox" bind:checked={onboardingResumeConfirmed} /> I confirm continuing the same immutable selected/all pairing batch.</label>
				<button class="btn btn-secondary" type="button" disabled={onboardingBusy || !onboardingResumeConfirmed} onclick={() => void resumeOnboardingBatch()}>{onboardingBusy ? 'Continuing…' : 'Continue same pairing batch'}</button>
			{/if}
			{#if onboardingBindings.length > 0}
				<ul class="resource-progress" aria-live="polite">
				{#each onboardingBindings as binding (binding.sourceCompanyID)}
					<li>
						{binding.sourceCompanyName}{binding.tenantName ? ` → ${binding.tenantName}` : ''}: {binding.status}{binding.reasonCode ? ` (${binding.reasonCode})` : ''}
						{#if binding.tenantID}
							<a class="link-button" href={`/migration?tenant=${encodeURIComponent(binding.tenantID)}`}>Continue with this tenant</a>
						{/if}
					</li>
					{/each}
				</ul>
			{/if}
			{#if onboardingBatchStatus === 'READY'}
				<SmartAccountsBatchRunner
					batchId={onboardingBatchID}
					workflow={onboardingBatchWorkflow}
					companyNames={batchCompanyNames()}
					onRefresh={getOnboardingBatchWorkflow}
					onPrepare={prepareOnboardingBatchWorkflow}
					onResume={resumeOnboardingBatchWorkflow}
					onAdvanceSafe={advanceOnboardingBatchSafe}
					onAdvanceConfirmedTransfer={advanceOnboardingBatchConfirmedTransfer}
					onConfirmSchema={confirmOnboardingBatchSchema}
					onReissueDiscovery={reissueOnboardingBatchDiscovery}
					onOpenTransfer={openOnboardingBatchTransfer}
					onConfirmTransfer={confirmOnboardingBatchTransfer}
					onWorkflowChange={(next) => { onboardingBatchWorkflow = next; }}
				/>
			{/if}
		</div>
	{/if}

	{#if hasTenant}
	<details class="api-fallback">
		<summary>Use API-key connection instead</summary>
		<p class="help">Use this fallback only when the signed-in Brave relay is unavailable.</p>
		<div class="form-grid">
			<div class="form-group">
				<label class="label" for="smartaccounts-api-key">SmartAccounts API key</label>
				<input id="smartaccounts-api-key" class="input" type="password" autocomplete="off" bind:value={apiKey} />
				<p class="help">Sent only to the private NUC bridge; Open Accounting never stores or returns it.</p>
			</div>
			<div class="form-group">
				<label class="label" for="smartaccounts-api-secret">SmartAccounts API secret</label>
				<input id="smartaccounts-api-secret" class="input" type="password" autocomplete="off" bind:value={apiSecret} />
				<p class="help">The bridge validates one safe accounts request before Open Accounting saves an opaque reference.</p>
			</div>
		</div>
		<div class="actions">
			<button class="btn btn-secondary" type="button" disabled={!canConfigure} onclick={() => void configure()}>
				{saving ? 'Connecting…' : 'Connect, validate & start full-history capture'}
			</button>
		</div>
	</details>
	{/if}

	<div class="actions">
		{#if status?.configured && captureRuns.length === 0}
			<button class="btn btn-secondary" type="button" disabled={startingCapture || saving} onclick={() => void startFullHistoryCapture()}>
				{startingCapture ? 'Starting capture…' : 'Start full-history capture'}
			</button>
		{/if}
		{#each captureRuns.filter((run) => run.status === 'rate_limited' || run.status === 'interrupted') as run (run.run_id)}
			<button class="btn btn-secondary" type="button" disabled={startingCapture || saving} onclick={() => void resumeCapture(run)}>
				{startingCapture ? 'Resuming capture…' : `Resume ${run.scope_mode === 'full_history' ? 'full-history' : 'date-window'} capture`}
			</button>
		{/each}
	</div>

	{#if status}
		<div class="progress" aria-live="polite">
			<h3>Progress and reconciliation</h3>
			<dl>
				<div><dt>Capture</dt><dd>{status.capture_status}</dd></div>
				<div><dt>Plan</dt><dd>{status.plan_status}</dd></div>
				<div><dt>Reconciliation</dt><dd>{status.reconciliation_status}</dd></div>
				<div><dt>Financial apply</dt><dd>{status.financial_apply_eligible ? 'Eligible after explicit confirmation' : 'Blocked pending review'}</dd></div>
			</dl>
			{#each captureRuns as run (run.run_id)}
				<section class="capture-run">
					<p>Capture run {run.run_id}: {run.summary.completed}/{run.summary.total} resources complete ({run.scope_mode === 'full_history' ? `full history through ${run.source_as_of_date}` : `${run.date_from} to ${run.date_to}`}).</p>
					<ul class="resource-progress">
						{#each run.resources as resource (resource.resource_id)}
							<li>{resource.resource_id}: {resource.status}{resource.reason_code ? ` (${resource.reason_code})` : ''}{resource.next_eligible_at ? ` (eligible ${resource.next_eligible_at})` : ''}</li>
						{/each}
					</ul>
				</section>
			{/each}
			{#if dateWindowResources.length > 0}
				<div class="package-review">
					<h3>Complete required date-window data</h3>
					<p>SmartAccounts requires a date range for {dateWindowResources.join(', ')}. Choose the agreed historical start and an end no earlier than the full capture’s source-as-of date ({requiredWindowEnd}); this follow-up captures only these missing services and will not repeat the full history.</p>
					<div class="form-grid compact-grid">
						<div class="form-group"><label class="label" for="smartaccounts-window-from">From</label><input id="smartaccounts-window-from" class="input" type="date" bind:value={dateWindowFrom} /></div>
						<div class="form-group"><label class="label" for="smartaccounts-window-to">To</label><input id="smartaccounts-window-to" class="input" type="date" bind:value={dateWindowTo} /></div>
					</div>
					<button class="btn btn-secondary" type="button" disabled={startingCapture || !dateWindowFrom || !dateWindowTo || dateWindowTo < dateWindowFrom || Boolean(requiredWindowEnd && dateWindowTo < requiredWindowEnd)} onclick={() => void startDateWindowCapture()}>{startingCapture ? 'Starting date-window capture…' : 'Capture missing date-window services'}</button>
				</div>
			{/if}
			{#if braveDiscoveryResources.length > 0}
				<div class="alert alert-error" role="status">Full capture is waiting for verified Brave request discovery for {braveDiscoveryResources.join(', ')}. No undocumented endpoint will be guessed or called.</div>
			{/if}
			<p>{status.next_action}</p>
			{#each captureRuns.filter((run) => run.scope_mode === 'full_history' && run.staging?.status === 'staged_review_required' && run.staging.finalized) as run (run.run_id)}
				<div class="package-review">
					<h3>Staged package ready for review</h3>
					<p>Package {run.staging?.package_id} is finalized ({run.staging?.record_chunks_acknowledged} record and {run.staging?.artifact_chunks_acknowledged} artifact chunks acknowledged). A chart proposal is prepared from source accounts; unknown types and collisions remain review-required.</p>
					{#if coverageBlocked}
						<p class="help">GL preview remains blocked until the required date-window capture and verified Brave endpoint discovery above are complete.</p>
					{:else}
						<button class="btn btn-primary" type="button" disabled={preparingPreview || applyingPreview} onclick={() => void preparePreview(run)}>{preparingPreview ? 'Preparing preview…' : 'Prepare chart and GL preview'}</button>
					{/if}
					{#if preview && previewPackageID === run.staging?.package_id}
						<p><strong>{preview.status}</strong>: {preview.journals?.length ?? 0} journals, {preview.account_imports?.length ?? 0} chart proposals, {preview.non_posting_record_count} non-posting archive records.</p>
						{#if preview.issues?.length}<ul>{#each preview.issues as issue (issue.code)}<li>{issue.code}: {issue.message}</li>{/each}</ul>{/if}
						{#if preview.status === 'PREVIEW_READY'}
							<label class="confirm"><input type="checkbox" bind:checked={applyConfirmed} /> I reviewed this GL-authoritative plan and want to create and post these journals once.</label>
							<button class="btn btn-primary" type="button" disabled={!applyConfirmed || applyingPreview} onclick={() => void applyPreview()}>{applyingPreview ? 'Applying…' : 'Confirm and apply GL plan'}</button>
						{/if}
					{/if}
					<button class="btn btn-secondary" type="button" disabled={preparingReferencePreview || applyingReferencePreview} onclick={() => void prepareReferencePreview(run.staging!.package_id)}>{preparingReferencePreview ? 'Preparing reference masters…' : 'Prepare non-financial master preview'}</button>
					{#if referencePreview && referencePreviewPackageID === run.staging?.package_id}
						<p><strong>{referencePreview.status}</strong>: {referencePreview.actions?.length ?? 0} account, contact, or item actions. This preview never posts a journal, invoice, or payment.</p>
						<ul class="resource-progress">{#each referencePreview.reconciliation as line (line.entity_type)}<li>{line.entity_type}: {line.source_records} source, {line.create_planned} planned, {line.already_applied} already applied, {line.review_required} review{line.tombstones ? ` (${line.tombstones} tombstones)` : ''}</li>{/each}</ul>
						{#if referencePreview.issues?.length}<ul>{#each referencePreview.issues as issue (issue.code)}<li>{issue.code}: {issue.message}</li>{/each}</ul>{/if}
						{#if referencePreview.status === 'PREVIEW_READY'}
							<label class="confirm"><input type="checkbox" bind:checked={referenceApplyConfirmed} /> I reviewed this non-financial account, contact, and item master plan. No financial posting will be made.</label>
							<button class="btn btn-primary" type="button" disabled={!referenceApplyConfirmed || applyingReferencePreview} onclick={() => void applyReferencePreview()}>{applyingReferencePreview ? 'Applying reference masters…' : 'Confirm and apply reference masters'}</button>
						{/if}
					{/if}
				</div>
			{/each}
		</div>
	{/if}
	{/if}
</section>

<style>
	.smartaccounts-control { margin: 1rem 0 1.5rem; }
	.panel-heading, .actions, .progress dl { display: flex; gap: 1rem; }
	.panel-heading { align-items: start; justify-content: space-between; }
	.panel-heading h2, .progress h3 { margin: 0; }
	.eyebrow, .help { color: var(--color-text-muted, #64748b); }
	.eyebrow { margin: 0 0 0.25rem; font-size: 0.75rem; font-weight: 700; letter-spacing: 0.08em; text-transform: uppercase; }
	.intro { max-width: 76ch; }
	.status-pill { border-radius: 999px; background: #e0e7ff; color: #3730a3; font-size: 0.8rem; font-weight: 700; padding: 0.25rem 0.6rem; white-space: nowrap; }
	.help { margin: 0.4rem 0 0; font-size: 0.85rem; }
	.read-only { background: var(--color-surface-subtle, #f8fafc); min-height: 2.4rem; padding: 0.6rem 0.75rem; }
	.api-fallback { margin-top: 1rem; }
	.api-fallback summary { cursor: pointer; font-weight: 600; }
	.actions { flex-wrap: wrap; margin-top: 1rem; }
	.progress { border-top: 1px solid var(--color-border, #e2e8f0); margin-top: 1rem; padding-top: 1rem; }
	.progress dl { flex-wrap: wrap; margin: 0.75rem 0; }
	.progress dl div { min-width: 10rem; }
	.progress dt { color: var(--color-text-muted, #64748b); font-size: 0.8rem; }
	.progress dd { font-weight: 600; margin: 0.2rem 0 0; }
	.resource-progress { margin: 0.75rem 0; padding-left: 1.25rem; }
	.capture-run { border-top: 1px solid var(--color-border, #e2e8f0); margin-top: 1rem; padding-top: 0.75rem; }
	.compact-grid { max-width: 36rem; }
	.package-review { border-top: 1px solid var(--color-border, #e2e8f0); margin-top: 1rem; padding-top: 1rem; }
	.confirm { display: block; margin: 0.75rem 0; }
	.company-picker { display: grid; gap: 0.5rem; margin: 0.75rem 0; }
	.company-choice { align-items: center; display: flex; gap: 0.55rem; }
</style>
