<script lang="ts">
	import Decimal from 'decimal.js';
	import {
		api,
		type BankTransaction,
		type DocumentAttachment,
		type FollowUpStatus,
		type JournalEntry,
		type OverdueInvoice,
		type OverdueInvoicesSummary,
		type PeriodCloseEvent,
		type Tenant
	} from '$lib/api';
	import * as m from '$lib/paraglide/messages.js';
	import {
		flattenUnmatchedTransactions,
		getSuggestedCloseDate,
		loadTenantReviewSnapshot,
		toDecimal,
		type BankExceptionGroup,
		type WorkspaceAssignmentAction,
		type WorkspaceAssignmentSource
	} from '$lib/review/workspace';

	let { tenant }: { tenant: Tenant } = $props();

	let isLoading = $state(true);
	let error = $state('');
	let overdueSummary = $state<OverdueInvoicesSummary | null>(null);
	let bankExceptions = $state<BankExceptionGroup[]>([]);
	let periodCloseEvents = $state<PeriodCloseEvent[]>([]);
	let journalEntries = $state<JournalEntry[]>([]);
	let assignmentActions = $state<WorkspaceAssignmentAction[]>([]);
	let assignmentErrorCount = $state(0);
	let reviewDrafts = $state<Record<string, { followUpStatus: FollowUpStatus; reviewNote: string }>>({});
	let reviewSavingId = $state('');
	let reviewSavedId = $state('');
	let reviewErrorId = $state('');
	let reviewError = $state('');
	let reminderSendingId = $state('');
	let reminderSentId = $state('');
	let reminderErrorId = $state('');
	let reminderError = $state('');
	let assignmentCompletingId = $state('');
	let assignmentCompletedMessage = $state('');
	let assignmentCompletionErrorId = $state('');
	let assignmentCompletionError = $state('');
	let assignmentRetentionDrafts = $state<Record<string, string>>({});
	let assignmentUploadDrafts = $state<Record<string, File | undefined>>({});
	let loadedTenantKey = '';

	type AssignmentEvidenceUploadTarget = {
		entityType: DocumentAttachment['entity_type'];
		entityId: string;
		documentType: DocumentAttachment['document_type'];
		notes: string;
		replacement: boolean;
	};

	const documentUploadEntityTypes: DocumentAttachment['entity_type'][] = [
		'invoice',
		'journal_entry',
		'payment',
		'bank_transaction',
		'asset',
		'expense',
		'quote',
		'order',
		'leave_record',
		'year_end_close'
	];
	const documentUploadTypes: DocumentAttachment['document_type'][] = [
		'supporting_document',
		'receipt',
		'reconciliation_evidence',
		'contract',
		'asset_record',
		'tax_support',
		'close_pack',
		'other'
	];

	$effect(() => {
		const tenantKey = buildTenantKey(tenant);
		if (!tenant.id || tenantKey === loadedTenantKey) {
			return;
		}

		loadedTenantKey = tenantKey;
		void loadReviewWorkspace(tenant);
	});

	async function loadReviewWorkspace(reviewTenant: Tenant) {
		isLoading = true;
		error = '';

		const snapshot = await loadTenantReviewSnapshot(reviewTenant);
		overdueSummary = snapshot.overdueSummary;
		periodCloseEvents = snapshot.periodCloseEvents;
		journalEntries = snapshot.journalEntries;
		bankExceptions = snapshot.bankExceptions;
		assignmentActions = snapshot.assignmentActions;
		assignmentErrorCount = snapshot.assignmentErrorCount;
		reviewDrafts = buildReviewDrafts(snapshot.bankExceptions);
		reviewSavedId = '';
		reviewErrorId = '';
		reviewError = '';
		reminderSentId = '';
		reminderErrorId = '';
		reminderError = '';

		if (snapshot.errorCount === 4) {
			error = m.errors_loadFailed();
		}

		isLoading = false;
	}

	function buildTenantKey(value: Tenant): string {
		return `${value.id}:${value.updated_at}:${value.settings?.period_lock_date ?? ''}`;
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

	function getEntryTotal(entry: JournalEntry): Decimal {
		return entry.lines.reduce((sum, line) => sum.add(toDecimal(line.base_debit)), new Decimal(0));
	}

	function getCloseActionLabel(event: PeriodCloseEvent): string {
		return event.action === 'reopen' ? m.settings_periodHistoryReopened() : m.settings_periodHistoryClosed();
	}

	function getCloseKindLabel(event: PeriodCloseEvent): string {
		return event.close_kind === 'year_end' ? m.settings_periodYearEnd() : m.settings_periodMonthEnd();
	}

	const unmatchedTransactions = $derived(flattenUnmatchedTransactions(bankExceptions));
	const unmatchedAmount = $derived(
		unmatchedTransactions.reduce((sum, item) => sum.add(toDecimal(item.transaction.amount).abs()), new Decimal(0))
	);
	const missingEvidenceCount = $derived(
		unmatchedTransactions.filter((item) => item.documentSummary.missing_evidence).length
	);
	const pendingEvidenceCount = $derived(
		unmatchedTransactions.filter((item) => item.documentSummary.has_pending_review).length
	);
	const topOverdueInvoices = $derived(overdueSummary?.invoices.slice(0, 4) ?? []);
	const topUnmatchedTransactions = $derived(unmatchedTransactions.slice(0, 4));
	const topAssignmentActions = $derived(assignmentActions.slice(0, 6));
	const highPriorityAssignmentCount = $derived(
		assignmentActions.filter((action) => action.priority.toLowerCase() === 'high').length
	);
	const dueNowAssignmentCount = $derived(
		assignmentActions.filter((action) => action.dueInDays <= 0).length
	);
	const cliReadyAssignmentCount = $derived(
		assignmentActions.filter((action) => Boolean(action.cliCommand)).length
	);
	const topJournalEntries = $derived(journalEntries.slice(0, 4));
	const journalDraftCount = $derived(journalEntries.filter((entry) => entry.status === 'DRAFT').length);
	const journalPostedCount = $derived(journalEntries.filter((entry) => entry.status === 'POSTED').length);
	const currentLockDate = $derived(tenant.settings?.period_lock_date ?? null);
	const suggestedCloseDate = $derived(getSuggestedCloseDate(currentLockDate));
	const latestCloseEvent = $derived(periodCloseEvents[0] ?? null);

	function getEvidenceLabel(missingEvidence: boolean, hasPendingReview: boolean): string {
		if (missingEvidence) {
			return m.dashboard_reviewEvidenceMissing();
		}
		if (hasPendingReview) {
			return m.dashboard_reviewEvidencePending();
		}
		return m.dashboard_reviewEvidenceReviewed();
	}

	function buildReviewDrafts(groups: BankExceptionGroup[]) {
		const drafts: Record<string, { followUpStatus: FollowUpStatus; reviewNote: string }> = {};
		for (const item of flattenUnmatchedTransactions(groups)) {
			drafts[item.transaction.id] = {
				followUpStatus: normalizeFollowUpStatus(item.transaction.follow_up_status),
				reviewNote: item.transaction.review_note ?? ''
			};
		}
		return drafts;
	}

	function normalizeFollowUpStatus(value: FollowUpStatus | string | undefined): FollowUpStatus {
		if (value === 'EVIDENCE_REQUIRED' || value === 'READY_TO_MATCH') {
			return value;
		}
		return 'NONE';
	}

	function getFollowUpLabel(value: FollowUpStatus | string | undefined): string {
		switch (normalizeFollowUpStatus(value)) {
			case 'EVIDENCE_REQUIRED':
				return m.dashboard_reviewFollowUpEvidenceRequired();
			case 'READY_TO_MATCH':
				return m.dashboard_reviewFollowUpReadyToMatch();
			default:
				return m.dashboard_reviewFollowUpNone();
		}
	}

	function getAssignmentSourceLabel(source: WorkspaceAssignmentSource): string {
		switch (source) {
			case 'close':
				return m.dashboard_reviewAssignmentSourceClose();
			case 'banking':
				return m.dashboard_reviewAssignmentSourceBanking();
			case 'documents':
				return m.dashboard_reviewAssignmentSourceDocuments();
			case 'expenses':
				return m.dashboard_reviewAssignmentSourceExpenses();
			case 'payroll':
				return m.dashboard_reviewAssignmentSourcePayroll();
			case 'tsd':
				return m.dashboard_reviewAssignmentSourceTsd();
			case 'kmd':
				return m.dashboard_reviewAssignmentSourceKmd();
			case 'migration':
				return m.dashboard_reviewAssignmentSourceMigration();
		}
	}

	function getAssignmentDueLabel(days: number): string {
		if (days < 0) {
			return m.dashboard_reviewAssignmentOverdue();
		}
		if (days === 0) {
			return m.dashboard_reviewAssignmentDueToday();
		}
		return m.dashboard_reviewAssignmentDueDays({ days });
	}

	function buildTenantScopedHref(path: string | undefined): string {
		if (!path) {
			return `/dashboard?tenant=${tenant.id}`;
		}

		const url = new URL(path.startsWith('/') ? path : `/${path}`, 'http://open-accounting.local');
		url.searchParams.set('tenant', tenant.id);
		return `${url.pathname}${url.search}${url.hash}`;
	}

	function canApproveAssignmentDocument(action: WorkspaceAssignmentAction): boolean {
		return (
			action.source === 'documents' &&
			['document_review_pending', 'document_evidence_unapproved'].includes(action.code) &&
			Boolean(action.documentId)
		);
	}

	function canSetAssignmentDocumentRetention(action: WorkspaceAssignmentAction): boolean {
		return (
			action.source === 'documents' &&
			['document_retention_missing', 'document_retention_due_soon', 'document_retention_expired'].includes(
				action.code
			) &&
			Boolean(action.documentId)
		);
	}

	function getAssignmentEvidenceUploadTarget(action: WorkspaceAssignmentAction): AssignmentEvidenceUploadTarget | null {
		if (action.source === 'banking' && action.code === 'bank_evidence_required' && action.entityId) {
			return {
				entityType: 'bank_transaction',
				entityId: action.entityId,
				documentType: 'reconciliation_evidence',
				notes: m.dashboard_reviewAssignmentEvidenceUploadNote(),
				replacement: false
			};
		}

		if (
			action.source !== 'documents' ||
			!['document_evidence_missing', 'document_review_rejected'].includes(action.code) ||
			!action.entityId
		) {
			return null;
		}

		const entityType = getDocumentUploadEntityType(action.entityType);
		const documentType = getDocumentUploadType(action.documentType);
		if (!entityType || !documentType) {
			return null;
		}

		const replacement = action.code === 'document_review_rejected';
		return {
			entityType,
			entityId: action.entityId,
			documentType,
			notes: replacement
				? m.dashboard_reviewAssignmentReplacementUploadNote()
				: m.dashboard_reviewAssignmentEvidenceUploadNote(),
			replacement
		};
	}

	function getDocumentUploadEntityType(value: string | undefined): DocumentAttachment['entity_type'] | null {
		return documentUploadEntityTypes.includes(value as DocumentAttachment['entity_type'])
			? (value as DocumentAttachment['entity_type'])
			: null;
	}

	function getDocumentUploadType(value: string | undefined): DocumentAttachment['document_type'] | null {
		return documentUploadTypes.includes(value as DocumentAttachment['document_type'])
			? (value as DocumentAttachment['document_type'])
			: null;
	}

	function updateAssignmentUploadDraft(action: WorkspaceAssignmentAction, event: Event) {
		const target = event.currentTarget as HTMLInputElement;
		assignmentUploadDrafts = {
			...assignmentUploadDrafts,
			[action.id]: target.files?.[0]
		};
	}

	function canUploadAssignmentEvidence(action: WorkspaceAssignmentAction): boolean {
		return getAssignmentEvidenceUploadTarget(action) !== null;
	}

	function getAssignmentUploadFieldLabel(action: WorkspaceAssignmentAction): string {
		return getAssignmentEvidenceUploadTarget(action)?.replacement
			? m.dashboard_reviewAssignmentsReplacementFile()
			: m.dashboard_reviewAssignmentsEvidenceFile();
	}

	function getAssignmentUploadButtonLabel(action: WorkspaceAssignmentAction): string {
		return getAssignmentEvidenceUploadTarget(action)?.replacement
			? m.dashboard_reviewAssignmentsUploadReplacement()
			: m.dashboard_reviewAssignmentsUploadEvidence();
	}

	function getDefaultAssignmentRetentionDate(action: WorkspaceAssignmentAction): string {
		if (!action.dueDate?.match(/^\d{4}-\d{2}-\d{2}$/)) {
			return '';
		}

		const dueYear = Number(action.dueDate.slice(0, 4));
		if (!dueYear) {
			return '';
		}
		return `${dueYear + 1}${action.dueDate.slice(4)}`;
	}

	function getAssignmentRetentionDate(action: WorkspaceAssignmentAction): string {
		return assignmentRetentionDrafts[action.id] ?? getDefaultAssignmentRetentionDate(action);
	}

	function updateAssignmentRetentionDraft(action: WorkspaceAssignmentAction, event: Event) {
		const target = event.currentTarget as HTMLInputElement;
		assignmentRetentionDrafts = {
			...assignmentRetentionDrafts,
			[action.id]: target.value
		};
	}

	function canApproveAssignmentPayroll(action: WorkspaceAssignmentAction): boolean {
		return action.source === 'payroll' && action.code === 'payroll_run_approve' && Boolean(action.entityId);
	}

	function canSetAssignmentPayrollPaymentDate(action: WorkspaceAssignmentAction): boolean {
		return (
			action.source === 'payroll' &&
			action.code === 'payroll_payment_date_missing' &&
			Boolean(action.entityId) &&
			parseAssignmentPeriod(action) !== null
		);
	}

	function canGenerateAssignmentTSD(action: WorkspaceAssignmentAction): boolean {
		return (
			action.source === 'payroll' &&
			['payroll_generate_tsd', 'payroll_paid_tsd_followup'].includes(action.code) &&
			Boolean(action.entityId)
		);
	}

	function canExportAssignmentTSD(action: WorkspaceAssignmentAction): boolean {
		if (parseAssignmentPeriod(action) === null) {
			return false;
		}

		if (action.source === 'tsd') {
			return ['tsd_export_and_submit', 'tsd_accepted_archive'].includes(action.code);
		}

		return action.source === 'payroll' && action.code === 'payroll_declared_archive';
	}

	function canAcceptAssignmentTSD(action: WorkspaceAssignmentAction): boolean {
		return (
			action.source === 'tsd' &&
			action.code === 'tsd_awaiting_authority_acceptance' &&
			parseAssignmentPeriod(action) !== null
		);
	}

	function canExecuteAssignmentMigration(action: WorkspaceAssignmentAction): boolean {
		return (
			action.source === 'migration' &&
			action.code === 'migration_execution_needs_confirmation' &&
			Boolean(action.entityId)
		);
	}

	function canSubmitAssignmentExpense(action: WorkspaceAssignmentAction): boolean {
		return action.source === 'expenses' && action.code === 'expense_submit_for_approval' && Boolean(action.entityId);
	}

	function canApproveAssignmentExpense(action: WorkspaceAssignmentAction): boolean {
		return action.source === 'expenses' && action.code === 'expense_approve_or_reject' && Boolean(action.entityId);
	}

	function canPostAssignmentExpense(action: WorkspaceAssignmentAction): boolean {
		return action.source === 'expenses' && action.code === 'expense_post_to_ledger' && Boolean(action.entityId);
	}

	function canCloseAssignmentFiscalYear(action: WorkspaceAssignmentAction): boolean {
		return action.source === 'close' && action.code === 'fiscal_year_not_closed' && Boolean(action.periodEndDate);
	}

	function canPostAssignmentCarryForward(action: WorkspaceAssignmentAction): boolean {
		return action.source === 'close' && action.code === 'ready_to_post_carry_forward' && Boolean(action.periodEndDate);
	}

	type AssignmentPeriod = {
		year: number;
		month: number;
	};

	function parseAssignmentPeriod(action: WorkspaceAssignmentAction): AssignmentPeriod | null {
		const match = action.period?.match(/^(\d{4})-(\d{2})$/);
		if (!match) {
			return null;
		}

		const year = Number(match[1]);
		const month = Number(match[2]);
		if (!year || month < 1 || month > 12) {
			return null;
		}

		return { year, month };
	}

	function getMonthEndDate(period: AssignmentPeriod): string {
		const date = new Date(Date.UTC(period.year, period.month, 0));
		const year = date.getUTCFullYear();
		const month = String(date.getUTCMonth() + 1).padStart(2, '0');
		const day = String(date.getUTCDate()).padStart(2, '0');
		return `${year}-${month}-${day}`;
	}

	function canRegenerateAssignmentKMD(action: WorkspaceAssignmentAction): boolean {
		return action.source === 'kmd' && action.code === 'kmd_no_vat_rows' && parseAssignmentPeriod(action) !== null;
	}

	function canExportAssignmentKMD(action: WorkspaceAssignmentAction): boolean {
		if (action.source !== 'kmd' || parseAssignmentPeriod(action) === null) {
			return false;
		}

		return [
			'kmd_payable_review',
			'kmd_refund_review',
			'kmd_zero_payable_review',
			'kmd_awaiting_authority_acceptance',
			'kmd_accepted_archive'
		].includes(action.code);
	}

	function canAcceptAssignmentKMD(action: WorkspaceAssignmentAction): boolean {
		return (
			action.source === 'kmd' &&
			action.code === 'kmd_awaiting_authority_acceptance' &&
			parseAssignmentPeriod(action) !== null
		);
	}

	function isReviewDirty(transaction: BankTransaction): boolean {
		const draft = reviewDrafts[transaction.id];
		if (!draft) {
			return false;
		}
		return (
			draft.followUpStatus !== normalizeFollowUpStatus(transaction.follow_up_status) ||
			draft.reviewNote.trim() !== (transaction.review_note ?? '').trim()
		);
	}

	async function saveTransactionReview(transaction: BankTransaction) {
		const draft = reviewDrafts[transaction.id];
		if (!draft) {
			return;
		}

		reviewSavingId = transaction.id;
		reviewSavedId = '';
		reviewErrorId = '';
		reviewError = '';

		try {
			await api.reviewBankTransaction(tenant.id, transaction.id, {
				follow_up_status: draft.followUpStatus,
				review_note: draft.reviewNote.trim()
			});
			await loadReviewWorkspace(tenant);
			reviewSavedId = transaction.id;
		} catch (err) {
			reviewErrorId = transaction.id;
			reviewError = err instanceof Error ? err.message : m.dashboard_reviewFollowUpSaveError();
		} finally {
			reviewSavingId = '';
		}
	}

	async function approveAssignmentDocument(action: WorkspaceAssignmentAction) {
		if (!action.documentId) {
			return;
		}

		assignmentCompletingId = action.id;
		assignmentCompletedMessage = '';
		assignmentCompletionErrorId = '';
		assignmentCompletionError = '';

		try {
			await api.reviewDocument(tenant.id, action.documentId, {
				review_status: 'APPROVED'
			});
			await loadReviewWorkspace(tenant);
			assignmentCompletedMessage = m.dashboard_reviewAssignmentDocumentApproved();
		} catch (err) {
			assignmentCompletionErrorId = action.id;
			assignmentCompletionError =
				err instanceof Error ? err.message : m.dashboard_reviewAssignmentDocumentApproveError();
		} finally {
			assignmentCompletingId = '';
		}
	}

	async function setAssignmentDocumentRetention(action: WorkspaceAssignmentAction) {
		if (!action.documentId) {
			return;
		}

		const retentionUntil = getAssignmentRetentionDate(action).trim();
		if (!retentionUntil.match(/^\d{4}-\d{2}-\d{2}$/)) {
			assignmentCompletionErrorId = action.id;
			assignmentCompletionError = m.dashboard_reviewAssignmentDocumentRetentionDateRequired();
			return;
		}

		assignmentCompletingId = action.id;
		assignmentCompletedMessage = '';
		assignmentCompletionErrorId = '';
		assignmentCompletionError = '';

		try {
			await api.updateDocumentRetention(tenant.id, action.documentId, {
				retention_until: retentionUntil
			});
			await loadReviewWorkspace(tenant);
			assignmentCompletedMessage = m.dashboard_reviewAssignmentDocumentRetentionSet();
		} catch (err) {
			assignmentCompletionErrorId = action.id;
			assignmentCompletionError =
				err instanceof Error ? err.message : m.dashboard_reviewAssignmentDocumentRetentionSetError();
		} finally {
			assignmentCompletingId = '';
		}
	}

	async function uploadAssignmentEvidence(action: WorkspaceAssignmentAction) {
		const target = getAssignmentEvidenceUploadTarget(action);
		if (!target) {
			return;
		}

		const file = assignmentUploadDrafts[action.id];
		if (!file) {
			assignmentCompletionErrorId = action.id;
			assignmentCompletionError = m.dashboard_reviewAssignmentEvidenceFileRequired();
			return;
		}

		assignmentCompletingId = action.id;
		assignmentCompletedMessage = '';
		assignmentCompletionErrorId = '';
		assignmentCompletionError = '';

		try {
			await api.uploadDocument(tenant.id, target.entityType, target.entityId, file, {
				document_type: target.documentType,
				notes: target.notes
			});
			await loadReviewWorkspace(tenant);
			assignmentCompletedMessage = m.dashboard_reviewAssignmentEvidenceUploaded();
		} catch (err) {
			assignmentCompletionErrorId = action.id;
			assignmentCompletionError =
				err instanceof Error ? err.message : m.dashboard_reviewAssignmentEvidenceUploadError();
		} finally {
			assignmentCompletingId = '';
		}
	}

	async function approveAssignmentPayroll(action: WorkspaceAssignmentAction) {
		if (!action.entityId) {
			return;
		}

		assignmentCompletingId = action.id;
		assignmentCompletedMessage = '';
		assignmentCompletionErrorId = '';
		assignmentCompletionError = '';

		try {
			await api.approvePayroll(tenant.id, action.entityId);
			await loadReviewWorkspace(tenant);
			assignmentCompletedMessage = m.dashboard_reviewAssignmentPayrollApproved();
		} catch (err) {
			assignmentCompletionErrorId = action.id;
			assignmentCompletionError =
				err instanceof Error ? err.message : m.dashboard_reviewAssignmentPayrollApproveError();
		} finally {
			assignmentCompletingId = '';
		}
	}

	async function setAssignmentPayrollPaymentDate(action: WorkspaceAssignmentAction) {
		const period = parseAssignmentPeriod(action);
		if (!action.entityId || !period) {
			return;
		}

		assignmentCompletingId = action.id;
		assignmentCompletedMessage = '';
		assignmentCompletionErrorId = '';
		assignmentCompletionError = '';

		try {
			await api.updatePayrollPaymentDate(tenant.id, action.entityId, {
				payment_date: getMonthEndDate(period)
			});
			await loadReviewWorkspace(tenant);
			assignmentCompletedMessage = m.dashboard_reviewAssignmentPayrollPaymentDateSet();
		} catch (err) {
			assignmentCompletionErrorId = action.id;
			assignmentCompletionError =
				err instanceof Error ? err.message : m.dashboard_reviewAssignmentPayrollPaymentDateError();
		} finally {
			assignmentCompletingId = '';
		}
	}

	async function generateAssignmentTSD(action: WorkspaceAssignmentAction) {
		if (!action.entityId) {
			return;
		}

		assignmentCompletingId = action.id;
		assignmentCompletedMessage = '';
		assignmentCompletionErrorId = '';
		assignmentCompletionError = '';

		try {
			await api.generateTSD(tenant.id, action.entityId);
			await loadReviewWorkspace(tenant);
			assignmentCompletedMessage = m.dashboard_reviewAssignmentTsdGenerated();
		} catch (err) {
			assignmentCompletionErrorId = action.id;
			assignmentCompletionError =
				err instanceof Error ? err.message : m.dashboard_reviewAssignmentTsdGenerateError();
		} finally {
			assignmentCompletingId = '';
		}
	}

	async function exportAssignmentTSD(action: WorkspaceAssignmentAction) {
		const period = parseAssignmentPeriod(action);
		if (!period) {
			return;
		}

		assignmentCompletingId = action.id;
		assignmentCompletedMessage = '';
		assignmentCompletionErrorId = '';
		assignmentCompletionError = '';

		try {
			await api.downloadTSDXml(tenant.id, period.year, period.month);
			await loadReviewWorkspace(tenant);
			assignmentCompletedMessage = m.dashboard_reviewAssignmentTsdExported();
		} catch (err) {
			assignmentCompletionErrorId = action.id;
			assignmentCompletionError =
				err instanceof Error ? err.message : m.dashboard_reviewAssignmentTsdExportError();
		} finally {
			assignmentCompletingId = '';
		}
	}

	async function acceptAssignmentTSD(action: WorkspaceAssignmentAction) {
		const period = parseAssignmentPeriod(action);
		if (!period) {
			return;
		}

		assignmentCompletingId = action.id;
		assignmentCompletedMessage = '';
		assignmentCompletionErrorId = '';
		assignmentCompletionError = '';

		try {
			await api.markTSDAccepted(tenant.id, period.year, period.month);
			await loadReviewWorkspace(tenant);
			assignmentCompletedMessage = m.dashboard_reviewAssignmentTsdAccepted();
		} catch (err) {
			assignmentCompletionErrorId = action.id;
			assignmentCompletionError =
				err instanceof Error ? err.message : m.dashboard_reviewAssignmentTsdAcceptError();
		} finally {
			assignmentCompletingId = '';
		}
	}

	async function executeAssignmentMigration(action: WorkspaceAssignmentAction) {
		if (!action.entityId) {
			return;
		}

		assignmentCompletingId = action.id;
		assignmentCompletedMessage = '';
		assignmentCompletionErrorId = '';
		assignmentCompletionError = '';

		try {
			await api.executeMigration(tenant.id, {
				files: [],
				confirm: true,
				resume_from_run_id: action.entityId
			});
			await loadReviewWorkspace(tenant);
			assignmentCompletedMessage = m.dashboard_reviewAssignmentMigrationExecuted();
		} catch (err) {
			assignmentCompletionErrorId = action.id;
			assignmentCompletionError =
				err instanceof Error ? err.message : m.dashboard_reviewAssignmentMigrationExecuteError();
		} finally {
			assignmentCompletingId = '';
		}
	}

	async function submitAssignmentExpense(action: WorkspaceAssignmentAction) {
		if (!action.entityId) {
			return;
		}

		assignmentCompletingId = action.id;
		assignmentCompletedMessage = '';
		assignmentCompletionErrorId = '';
		assignmentCompletionError = '';

		try {
			await api.submitExpense(tenant.id, action.entityId);
			await loadReviewWorkspace(tenant);
			assignmentCompletedMessage = m.dashboard_reviewAssignmentExpenseSubmitted();
		} catch (err) {
			assignmentCompletionErrorId = action.id;
			assignmentCompletionError =
				err instanceof Error ? err.message : m.dashboard_reviewAssignmentExpenseSubmitError();
		} finally {
			assignmentCompletingId = '';
		}
	}

	async function approveAssignmentExpense(action: WorkspaceAssignmentAction) {
		if (!action.entityId) {
			return;
		}

		assignmentCompletingId = action.id;
		assignmentCompletedMessage = '';
		assignmentCompletionErrorId = '';
		assignmentCompletionError = '';

		try {
			await api.approveExpense(tenant.id, action.entityId);
			await loadReviewWorkspace(tenant);
			assignmentCompletedMessage = m.dashboard_reviewAssignmentExpenseApproved();
		} catch (err) {
			assignmentCompletionErrorId = action.id;
			assignmentCompletionError =
				err instanceof Error ? err.message : m.dashboard_reviewAssignmentExpenseApproveError();
		} finally {
			assignmentCompletingId = '';
		}
	}

	async function postAssignmentExpense(action: WorkspaceAssignmentAction) {
		if (!action.entityId) {
			return;
		}

		assignmentCompletingId = action.id;
		assignmentCompletedMessage = '';
		assignmentCompletionErrorId = '';
		assignmentCompletionError = '';

		try {
			await api.postExpense(tenant.id, action.entityId);
			await loadReviewWorkspace(tenant);
			assignmentCompletedMessage = m.dashboard_reviewAssignmentExpensePosted();
		} catch (err) {
			assignmentCompletionErrorId = action.id;
			assignmentCompletionError =
				err instanceof Error ? err.message : m.dashboard_reviewAssignmentExpensePostError();
		} finally {
			assignmentCompletingId = '';
		}
	}

	async function closeAssignmentFiscalYear(action: WorkspaceAssignmentAction) {
		if (!action.periodEndDate) {
			return;
		}

		assignmentCompletingId = action.id;
		assignmentCompletedMessage = '';
		assignmentCompletionErrorId = '';
		assignmentCompletionError = '';

		try {
			await api.closePeriod(tenant.id, {
				period_end_date: action.periodEndDate,
				note: m.dashboard_reviewAssignmentCloseYearNote(),
				reviewer_sign_off: true,
				...(tenant.settings?.inventory_valuation_method
					? { inventory_valuation_method: tenant.settings.inventory_valuation_method }
					: {})
			});
			await loadReviewWorkspace(tenant);
			assignmentCompletedMessage = m.dashboard_reviewAssignmentCloseYearClosed();
		} catch (err) {
			assignmentCompletionErrorId = action.id;
			assignmentCompletionError =
				err instanceof Error ? err.message : m.dashboard_reviewAssignmentCloseYearError();
		} finally {
			assignmentCompletingId = '';
		}
	}

	async function postAssignmentCarryForward(action: WorkspaceAssignmentAction) {
		if (!action.periodEndDate) {
			return;
		}

		assignmentCompletingId = action.id;
		assignmentCompletedMessage = '';
		assignmentCompletionErrorId = '';
		assignmentCompletionError = '';

		try {
			await api.createYearEndCarryForward(tenant.id, {
				period_end_date: action.periodEndDate,
				...(tenant.settings?.inventory_valuation_method
					? { inventory_valuation_method: tenant.settings.inventory_valuation_method }
					: {})
			});
			await loadReviewWorkspace(tenant);
			assignmentCompletedMessage = m.dashboard_reviewAssignmentCarryForwardPosted();
		} catch (err) {
			assignmentCompletionErrorId = action.id;
			assignmentCompletionError =
				err instanceof Error ? err.message : m.dashboard_reviewAssignmentCarryForwardPostError();
		} finally {
			assignmentCompletingId = '';
		}
	}

	async function regenerateAssignmentKMD(action: WorkspaceAssignmentAction) {
		const period = parseAssignmentPeriod(action);
		if (!period) {
			return;
		}

		assignmentCompletingId = action.id;
		assignmentCompletedMessage = '';
		assignmentCompletionErrorId = '';
		assignmentCompletionError = '';

		try {
			await api.generateKMD(tenant.id, period);
			await loadReviewWorkspace(tenant);
			assignmentCompletedMessage = m.dashboard_reviewAssignmentKmdRegenerated();
		} catch (err) {
			assignmentCompletionErrorId = action.id;
			assignmentCompletionError =
				err instanceof Error ? err.message : m.dashboard_reviewAssignmentKmdGenerateError();
		} finally {
			assignmentCompletingId = '';
		}
	}

	async function exportAssignmentKMD(action: WorkspaceAssignmentAction) {
		const period = parseAssignmentPeriod(action);
		if (!period) {
			return;
		}

		assignmentCompletingId = action.id;
		assignmentCompletedMessage = '';
		assignmentCompletionErrorId = '';
		assignmentCompletionError = '';

		try {
			await api.downloadKMDXml(tenant.id, period.year, period.month);
			await loadReviewWorkspace(tenant);
			assignmentCompletedMessage = m.dashboard_reviewAssignmentKmdExported();
		} catch (err) {
			assignmentCompletionErrorId = action.id;
			assignmentCompletionError =
				err instanceof Error ? err.message : m.dashboard_reviewAssignmentKmdExportError();
		} finally {
			assignmentCompletingId = '';
		}
	}

	async function acceptAssignmentKMD(action: WorkspaceAssignmentAction) {
		const period = parseAssignmentPeriod(action);
		if (!period) {
			return;
		}

		assignmentCompletingId = action.id;
		assignmentCompletedMessage = '';
		assignmentCompletionErrorId = '';
		assignmentCompletionError = '';

		try {
			await api.markKMDAccepted(tenant.id, period.year, period.month);
			await loadReviewWorkspace(tenant);
			assignmentCompletedMessage = m.dashboard_reviewAssignmentKmdAccepted();
		} catch (err) {
			assignmentCompletionErrorId = action.id;
			assignmentCompletionError =
				err instanceof Error ? err.message : m.dashboard_reviewAssignmentKmdAcceptError();
		} finally {
			assignmentCompletingId = '';
		}
	}

	async function sendInvoiceReminder(invoice: OverdueInvoice) {
		if (!invoice.contact_email) {
			return;
		}

		reminderSendingId = invoice.id;
		reminderSentId = '';
		reminderErrorId = '';
		reminderError = '';

		try {
			const result = await api.sendPaymentReminder(tenant.id, invoice.id, undefined);
			if (!result.success) {
				throw new Error(result.message || m.dashboard_reviewReminderSendError());
			}

			await loadReviewWorkspace(tenant);
			reminderSentId = invoice.id;
		} catch (err) {
			reminderErrorId = invoice.id;
			reminderError = err instanceof Error ? err.message : m.dashboard_reviewReminderSendError();
		} finally {
			reminderSendingId = '';
		}
	}
</script>

<section class="review-board card">
	<div class="review-board-header">
		<div>
			<div class="review-kicker">{m.dashboard_reviewQueue()}</div>
			<h3>{m.dashboard_reviewQueueTitle()}</h3>
			<p>{m.dashboard_reviewQueueDesc()}</p>
		</div>
		<button class="btn btn-secondary review-refresh" type="button" onclick={() => loadReviewWorkspace(tenant)} disabled={isLoading}>
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
						{#each topOverdueInvoices as invoice (invoice.id)}
							<li class="review-list-item-invoice">
								<div class="review-list-main">
									<strong>{invoice.invoice_number}</strong>
									<span>{invoice.contact_name}</span>
									{#if invoice.contact_email}
										<span>{invoice.contact_email}</span>
									{:else}
										<span>{m.reminder_no_email()}</span>
									{/if}
								</div>
								<div class="review-list-meta">
									<strong>{formatCurrency(invoice.outstanding_amount)}</strong>
									<span>{invoice.days_overdue} {m.dashboard_reviewDaysShort()}</span>
								</div>
								<div class="review-invoice-actions">
									<button
										class="btn btn-secondary review-reminder-button"
										type="button"
										onclick={() => sendInvoiceReminder(invoice)}
										disabled={reminderSendingId === invoice.id || !invoice.contact_email}
									>
										{reminderSendingId === invoice.id ? m.reminder_sending() : m.reminder_send_now()}
									</button>
									{#if reminderSentId === invoice.id}
										<span class="review-feedback review-feedback-success">
											{m.reminder_sent_success({ invoice: invoice.invoice_number })}
										</span>
									{/if}
									{#if reminderErrorId === invoice.id}
										<span class="review-feedback review-feedback-error">{reminderError}</span>
									{/if}
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
						<strong>{missingEvidenceCount}</strong>
						<span>{m.dashboard_reviewEvidenceMissing()}</span>
					</div>
					<div>
						<strong>{pendingEvidenceCount}</strong>
						<span>{m.dashboard_reviewEvidencePending()}</span>
					</div>
				</div>

				{#if topUnmatchedTransactions.length > 0}
					<ul class="review-list">
						{#each topUnmatchedTransactions as item (item.transaction.id)}
							<li class="review-list-item-banking">
								<div class="review-list-main">
									<strong>{item.account.name}</strong>
									<span>{item.transaction.description || item.transaction.counterparty_name || m.common_noData()}</span>
									<span>{getEvidenceLabel(item.documentSummary.missing_evidence, item.documentSummary.has_pending_review)}</span>
									<span>{getFollowUpLabel(item.transaction.follow_up_status)}</span>
									{#if item.transaction.review_note}
										<span class="review-note-preview">{item.transaction.review_note}</span>
									{/if}
								</div>
								<div class="review-list-meta review-list-meta-banking">
									<strong>{formatCurrency(toDecimal(item.transaction.amount).abs())}</strong>
									<span>{formatDate(item.transaction.transaction_date)}</span>
								</div>
								<div class="review-transaction-review">
									<label class="review-field">
										<span>{m.dashboard_reviewFollowUpLabel()}</span>
										<select
											aria-label={m.dashboard_reviewFollowUpLabel()}
											bind:value={reviewDrafts[item.transaction.id].followUpStatus}
										>
											<option value="NONE">{m.dashboard_reviewFollowUpNone()}</option>
											<option value="EVIDENCE_REQUIRED">{m.dashboard_reviewFollowUpEvidenceRequired()}</option>
											<option value="READY_TO_MATCH">{m.dashboard_reviewFollowUpReadyToMatch()}</option>
										</select>
									</label>
									<label class="review-field review-field-note">
										<span>{m.dashboard_reviewFollowUpNoteLabel()}</span>
										<textarea
											aria-label={m.dashboard_reviewFollowUpNoteLabel()}
											rows="2"
											bind:value={reviewDrafts[item.transaction.id].reviewNote}
										></textarea>
									</label>
									<div class="review-inline-actions">
										<button
											class="btn btn-secondary review-inline-save"
											type="button"
											onclick={() => saveTransactionReview(item.transaction)}
											disabled={reviewSavingId === item.transaction.id || !isReviewDirty(item.transaction)}
										>
											{reviewSavingId === item.transaction.id ? m.common_loading() : m.dashboard_reviewFollowUpSave()}
										</button>
										{#if reviewSavedId === item.transaction.id}
											<span class="review-feedback review-feedback-success">{m.dashboard_reviewFollowUpSaved()}</span>
										{/if}
										{#if reviewErrorId === item.transaction.id}
											<span class="review-feedback review-feedback-error">{reviewError}</span>
										{/if}
									</div>
								</div>
							</li>
						{/each}
					</ul>
				{:else}
					<p class="review-empty">{m.dashboard_reviewNoBankingExceptions()}</p>
				{/if}
			</article>

			<article id="assignment-queue" class="review-card">
				<div class="review-card-topline">
					<span class="review-card-kicker">{m.dashboard_reviewAssignmentsTitle()}</span>
					<a href="/documents?tenant={tenant.id}&review_status=PENDING" class="review-action">{m.dashboard_reviewAssignmentsOpenDocuments()}</a>
				</div>
				<div class="review-figure">
					<strong>{assignmentActions.length}</strong>
					<span>{m.dashboard_reviewAssignmentsCount()}</span>
				</div>
				<div class="review-metrics">
					<div>
						<strong>{highPriorityAssignmentCount}</strong>
						<span>{m.dashboard_reviewAssignmentsHighPriority()}</span>
					</div>
					<div>
						<strong>{dueNowAssignmentCount}</strong>
						<span>{m.dashboard_reviewAssignmentsDueNow()}</span>
					</div>
					<div>
						<strong>{cliReadyAssignmentCount}</strong>
						<span>{m.dashboard_reviewAssignmentsCliReady()}</span>
					</div>
				</div>

				{#if assignmentErrorCount > 0}
					<p class="review-feedback review-feedback-error">{m.dashboard_reviewAssignmentsPartial()}</p>
				{/if}
				{#if assignmentCompletedMessage}
					<p class="review-feedback review-feedback-success">{assignmentCompletedMessage}</p>
				{/if}

				{#if topAssignmentActions.length > 0}
					<ul class="review-list review-assignment-list">
						{#each topAssignmentActions as action (action.id)}
							<li class="review-list-item-assignment">
								<div class="review-list-main">
									<strong>{action.message}</strong>
									<span>
										{getAssignmentSourceLabel(action.source)} · {action.queue} · {action.severity}
									</span>
									<span>{action.action}</span>
									{#if action.cliCommand}
										<code>{m.dashboard_reviewAssignmentCommand()}: {action.cliCommand}</code>
									{/if}
								</div>
								<div class="review-list-meta review-list-meta-assignment">
									<strong>{action.priority}</strong>
									<span>{getAssignmentDueLabel(action.dueInDays)}</span>
									<a class="review-action" href={buildTenantScopedHref(action.uiPath)}>
										{m.dashboard_reviewAssignmentsOpenAction()}
									</a>
									{#if canCloseAssignmentFiscalYear(action)}
										<button
											class="review-action review-action-button"
											type="button"
											onclick={() => closeAssignmentFiscalYear(action)}
											disabled={assignmentCompletingId === action.id}
										>
											{assignmentCompletingId === action.id
												? m.common_loading()
												: m.dashboard_reviewAssignmentsCloseYear()}
										</button>
									{/if}
									{#if canPostAssignmentCarryForward(action)}
										<button
											class="review-action review-action-button"
											type="button"
											onclick={() => postAssignmentCarryForward(action)}
											disabled={assignmentCompletingId === action.id}
										>
											{assignmentCompletingId === action.id
												? m.common_loading()
												: m.dashboard_reviewAssignmentsPostCarryForward()}
										</button>
									{/if}
									{#if canApproveAssignmentDocument(action)}
										<button
											class="review-action review-action-button"
											type="button"
											onclick={() => approveAssignmentDocument(action)}
											disabled={assignmentCompletingId === action.id}
										>
											{assignmentCompletingId === action.id
												? m.common_loading()
												: m.dashboard_reviewAssignmentsApproveDocument()}
										</button>
									{/if}
									{#if canSetAssignmentDocumentRetention(action)}
										<label class="review-assignment-inline-field">
											<span>{m.dashboard_reviewAssignmentsRetentionDate()}</span>
											<input
												type="date"
												value={getAssignmentRetentionDate(action)}
												oninput={(event) => updateAssignmentRetentionDraft(action, event)}
											/>
										</label>
										<button
											class="review-action review-action-button"
											type="button"
											onclick={() => setAssignmentDocumentRetention(action)}
											disabled={assignmentCompletingId === action.id || !getAssignmentRetentionDate(action)}
										>
											{assignmentCompletingId === action.id
												? m.common_loading()
												: m.dashboard_reviewAssignmentsSetRetention()}
										</button>
									{/if}
									{#if canUploadAssignmentEvidence(action)}
										<label class="review-assignment-upload-field">
											<span>{getAssignmentUploadFieldLabel(action)}</span>
											<input
												type="file"
												onchange={(event) => updateAssignmentUploadDraft(action, event)}
											/>
										</label>
										<button
											class="review-action review-action-button"
											type="button"
											onclick={() => uploadAssignmentEvidence(action)}
											disabled={assignmentCompletingId === action.id || !assignmentUploadDrafts[action.id]}
										>
											{assignmentCompletingId === action.id
												? m.common_loading()
												: getAssignmentUploadButtonLabel(action)}
										</button>
									{/if}
									{#if canApproveAssignmentPayroll(action)}
										<button
											class="review-action review-action-button"
											type="button"
											onclick={() => approveAssignmentPayroll(action)}
											disabled={assignmentCompletingId === action.id}
										>
											{assignmentCompletingId === action.id
												? m.common_loading()
												: m.dashboard_reviewAssignmentsApprovePayroll()}
										</button>
									{/if}
									{#if canSetAssignmentPayrollPaymentDate(action)}
										<button
											class="review-action review-action-button"
											type="button"
											onclick={() => setAssignmentPayrollPaymentDate(action)}
											disabled={assignmentCompletingId === action.id}
										>
											{assignmentCompletingId === action.id
												? m.common_loading()
												: m.dashboard_reviewAssignmentsSetPayrollPaymentDate()}
										</button>
									{/if}
									{#if canGenerateAssignmentTSD(action)}
										<button
											class="review-action review-action-button"
											type="button"
											onclick={() => generateAssignmentTSD(action)}
											disabled={assignmentCompletingId === action.id}
										>
											{assignmentCompletingId === action.id
												? m.common_loading()
												: m.dashboard_reviewAssignmentsGenerateTsd()}
										</button>
									{/if}
									{#if canExportAssignmentTSD(action)}
										<button
											class="review-action review-action-button"
											type="button"
											onclick={() => exportAssignmentTSD(action)}
											disabled={assignmentCompletingId === action.id}
										>
											{assignmentCompletingId === action.id
												? m.common_loading()
												: m.dashboard_reviewAssignmentsExportTsdXml()}
										</button>
									{/if}
									{#if canAcceptAssignmentTSD(action)}
										<button
											class="review-action review-action-button"
											type="button"
											onclick={() => acceptAssignmentTSD(action)}
											disabled={assignmentCompletingId === action.id}
										>
											{assignmentCompletingId === action.id
												? m.common_loading()
												: m.dashboard_reviewAssignmentsAcceptTsd()}
										</button>
									{/if}
									{#if canExecuteAssignmentMigration(action)}
										<button
											class="review-action review-action-button"
											type="button"
											onclick={() => executeAssignmentMigration(action)}
											disabled={assignmentCompletingId === action.id}
										>
											{assignmentCompletingId === action.id
												? m.common_loading()
												: m.dashboard_reviewAssignmentsExecuteMigration()}
										</button>
									{/if}
									{#if canSubmitAssignmentExpense(action)}
										<button
											class="review-action review-action-button"
											type="button"
											onclick={() => submitAssignmentExpense(action)}
											disabled={assignmentCompletingId === action.id}
										>
											{assignmentCompletingId === action.id
												? m.common_loading()
												: m.dashboard_reviewAssignmentsSubmitExpense()}
										</button>
									{/if}
									{#if canApproveAssignmentExpense(action)}
										<button
											class="review-action review-action-button"
											type="button"
											onclick={() => approveAssignmentExpense(action)}
											disabled={assignmentCompletingId === action.id}
										>
											{assignmentCompletingId === action.id
												? m.common_loading()
												: m.dashboard_reviewAssignmentsApproveExpense()}
										</button>
									{/if}
									{#if canPostAssignmentExpense(action)}
										<button
											class="review-action review-action-button"
											type="button"
											onclick={() => postAssignmentExpense(action)}
											disabled={assignmentCompletingId === action.id}
										>
											{assignmentCompletingId === action.id
												? m.common_loading()
												: m.dashboard_reviewAssignmentsPostExpense()}
										</button>
									{/if}
									{#if canRegenerateAssignmentKMD(action)}
										<button
											class="review-action review-action-button"
											type="button"
											onclick={() => regenerateAssignmentKMD(action)}
											disabled={assignmentCompletingId === action.id}
										>
											{assignmentCompletingId === action.id
												? m.common_loading()
												: m.dashboard_reviewAssignmentsRegenerateKmd()}
										</button>
									{/if}
									{#if canExportAssignmentKMD(action)}
										<button
											class="review-action review-action-button"
											type="button"
											onclick={() => exportAssignmentKMD(action)}
											disabled={assignmentCompletingId === action.id}
										>
											{assignmentCompletingId === action.id
												? m.common_loading()
												: m.dashboard_reviewAssignmentsExportKmdXml()}
										</button>
									{/if}
									{#if canAcceptAssignmentKMD(action)}
										<button
											class="review-action review-action-button"
											type="button"
											onclick={() => acceptAssignmentKMD(action)}
											disabled={assignmentCompletingId === action.id}
										>
											{assignmentCompletingId === action.id
												? m.common_loading()
												: m.dashboard_reviewAssignmentsAcceptKmd()}
										</button>
									{/if}
									{#if assignmentCompletionErrorId === action.id}
										<span class="review-feedback review-feedback-error">{assignmentCompletionError}</span>
									{/if}
								</div>
							</li>
						{/each}
					</ul>
				{:else}
					<p class="review-empty">{m.dashboard_reviewAssignmentsEmpty()}</p>
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
						{#each periodCloseEvents.slice(0, 4) as event (event.id)}
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
						{#each topJournalEntries as entry (entry.id)}
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

	.review-action-button {
		border: none;
		background: transparent;
		padding: 0;
		cursor: pointer;
	}

	.review-action-button:disabled {
		cursor: wait;
		opacity: 0.65;
		text-decoration: none;
	}

	.review-assignment-inline-field {
		display: flex;
		align-items: center;
		justify-content: flex-end;
		gap: 0.35rem;
		font-size: 0.78rem;
		color: var(--color-text-muted);
	}

	.review-assignment-inline-field input {
		width: 8.7rem;
		min-height: 1.75rem;
		border: 1px solid var(--color-border);
		border-radius: 6px;
		padding: 0.15rem 0.35rem;
		color: var(--color-text);
		background: var(--color-surface);
		font: inherit;
	}

	.review-assignment-upload-field {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		gap: 0.25rem;
		max-width: 12rem;
		font-size: 0.78rem;
		color: var(--color-text-muted);
	}

	.review-assignment-upload-field input {
		width: 100%;
		max-width: 12rem;
		font-size: 0.75rem;
		color: var(--color-text);
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

	.review-list li.review-list-item-banking {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
	}

	.review-list li.review-list-item-invoice {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto minmax(9rem, auto);
		align-items: flex-start;
	}

	.review-list li.review-list-item-assignment {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(9rem, auto);
	}

	.review-list-main {
		min-width: 0;
	}

	.review-list-main code {
		display: block;
		max-width: 100%;
		overflow-wrap: anywhere;
		color: var(--color-text);
		background: rgba(15, 23, 42, 0.05);
		border-radius: 0.5rem;
		padding: 0.4rem 0.5rem;
		font-size: 0.75rem;
	}

	.review-list-meta {
		text-align: right;
	}

	.review-list-meta-banking {
		align-items: flex-end;
	}

	.review-list-meta-assignment {
		align-items: flex-end;
	}

	.review-note-preview {
		font-style: italic;
	}

	.review-invoice-actions {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		gap: 0.45rem;
		min-width: 9rem;
	}

	.review-reminder-button {
		justify-content: center;
		width: 100%;
	}

	.review-card-emphasis .review-reminder-button {
		background: rgba(255, 255, 255, 0.12);
		border-color: rgba(226, 232, 240, 0.24);
		color: rgba(248, 250, 252, 0.96);
	}

	.review-card-emphasis .review-reminder-button:hover:not(:disabled) {
		background: rgba(255, 255, 255, 0.2);
	}

	.review-card-emphasis .review-reminder-button:disabled {
		color: rgba(226, 232, 240, 0.52);
	}

	.review-transaction-review {
		grid-column: 1 / -1;
		display: grid;
		grid-template-columns: minmax(11rem, 13rem) minmax(0, 1fr) auto;
		gap: 0.75rem;
		padding-top: 0.75rem;
		margin-top: 0.15rem;
		border-top: 1px solid rgba(148, 163, 184, 0.12);
	}

	.review-field {
		display: flex;
		flex-direction: column;
		gap: 0.35rem;
		font-size: 0.82rem;
		color: var(--color-text-muted);
	}

	.review-field select,
	.review-field textarea {
		width: 100%;
		border: 1px solid rgba(148, 163, 184, 0.28);
		border-radius: 0.85rem;
		background: rgba(255, 255, 255, 0.92);
		color: var(--color-text);
		padding: 0.7rem 0.8rem;
		font: inherit;
	}

	.review-field textarea {
		min-height: 5.1rem;
		resize: vertical;
	}

	.review-field-note {
		min-width: 0;
	}

	.review-inline-actions {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		gap: 0.45rem;
	}

	.review-inline-save {
		min-width: 9rem;
		justify-content: center;
	}

	.review-feedback {
		font-size: 0.78rem;
		text-align: right;
	}

	.review-feedback-success {
		color: #166534;
	}

	.review-feedback-error {
		color: #b91c1c;
		max-width: 16rem;
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

		.review-list li.review-list-item-banking {
			grid-template-columns: 1fr;
		}

		.review-list li.review-list-item-invoice {
			grid-template-columns: 1fr;
		}

		.review-list li.review-list-item-assignment {
			grid-template-columns: 1fr;
		}

		.review-invoice-actions {
			align-items: stretch;
			width: 100%;
		}

		.review-list-meta-banking {
			align-items: flex-start;
			text-align: left;
		}

		.review-list-meta-assignment {
			align-items: flex-start;
			text-align: left;
		}

		.review-assignment-inline-field {
			justify-content: flex-start;
		}

		.review-assignment-upload-field {
			align-items: flex-start;
			max-width: 100%;
		}

		.review-transaction-review {
			grid-template-columns: 1fr;
		}

		.review-inline-actions {
			align-items: stretch;
		}

		.review-list li {
			flex-direction: column;
		}

		.review-list-meta {
			text-align: left;
		}
	}
</style>
