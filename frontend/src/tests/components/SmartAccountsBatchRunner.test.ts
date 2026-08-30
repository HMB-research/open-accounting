import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import type { SmartAccountsBrowserBatchWorkflowStatus } from '$lib/api';
import SmartAccountsBatchRunner from '$lib/components/SmartAccountsBatchRunner.svelte';

const digest = 'a'.repeat(64);

function workflowStatus(
	phase: SmartAccountsBrowserBatchWorkflowStatus['sources'][number]['phase'],
	overrides: Partial<SmartAccountsBrowserBatchWorkflowStatus> = {}
): SmartAccountsBrowserBatchWorkflowStatus {
	return {
		workflow: {
			batch_id: 'd436c224-5df5-4b4d-a772-1897f9147400',
			schema_version: 'smartaccounts-browser-batch-workflow-v1',
			history_from: '2024-01-01',
			header_probe_consent_confirmed: false,
			preparatory_manifest_sha256: digest,
			preparatory_consented_at: '2026-08-28T10:00:00Z',
			created_at: '2026-08-28T10:00:00Z',
			updated_at: '2026-08-28T10:00:00Z'
		},
		status: phase,
		sources: [{
			batch_id: 'd436c224-5df5-4b4d-a772-1897f9147400',
			source_company_id: 'sa-browser-v1-1234',
			tenant_id: 'tenant-1',
			ordinal: 0,
			phase,
			phase_generation: 1,
			attempt_count: 0,
			created_at: '2026-08-28T10:00:00Z',
			updated_at: '2026-08-28T10:00:00Z'
		}],
		...overrides
	};
}

function actions() {
	return {
		onRefresh: vi.fn().mockResolvedValue(workflowStatus('DISCOVERY_REQUIRED')),
		onPrepare: vi.fn().mockResolvedValue(workflowStatus('DISCOVERY_REQUIRED')),
		onResume: vi.fn().mockResolvedValue(workflowStatus('DISCOVERY_REQUIRED')),
		onAdvanceSafe: vi.fn().mockResolvedValue(workflowStatus('DISCOVERY_RUNNING')),
		 onAdvanceConfirmedTransfer: vi.fn().mockResolvedValue(workflowStatus('CAPTURE_RUNNING')),
		onReissueDiscovery: vi.fn().mockResolvedValue(workflowStatus('DISCOVERY_RUNNING')),
		onOpenTransfer: vi.fn().mockResolvedValue(workflowStatus('TRANSFER_CONFIRMATION_REQUIRED')),
		onConfirmTransfer: vi.fn().mockResolvedValue(workflowStatus('TRANSFER_CONFIRMATION_REQUIRED'))
	};
}

describe('SmartAccountsBatchRunner', () => {
	afterEach(cleanup);

	it('prepares once with the three explicit non-destructive consents, then lets the server advance one safe step', async () => {
		const runner = actions();
		render(SmartAccountsBatchRunner, {
			batchId: 'd436c224-5df5-4b4d-a772-1897f9147400',
			companyNames: { 'sa-browser-v1-1234': 'Hold My Beer OÜ' },
			...runner
		});

		await fireEvent.input(screen.getByLabelText('History starts'), { target: { value: '2024-01-01' } });
		await fireEvent.click(screen.getByLabelText(/I approve metadata-only discovery/));
		await fireEvent.click(screen.getByLabelText(/I separately approve bounded CSV header-name probing/));
		await fireEvent.click(screen.getByLabelText(/I confirm this selected\/all batch is ready/));
		await fireEvent.click(screen.getByRole('button', { name: 'Prepare safe batch workflow' }));

		await waitFor(() => expect(runner.onPrepare).toHaveBeenCalledWith({
			history_from: '2024-01-01',
			owner_confirmed: true,
			metadata_discovery_consent_confirmed: true,
			header_probe_consent_confirmed: true
		}));
		expect(runner.onAdvanceSafe).toHaveBeenCalledTimes(1);
		expect(screen.getAllByText('Hold My Beer OÜ').length).toBeGreaterThan(0);
		expect(screen.getByText('discovery running')).toBeInTheDocument();
	});

	it('requires renewed consent before reissuing a lost running discovery action', async () => {
		const runner = actions();
		const running = workflowStatus('DISCOVERY_RUNNING', { sources: [{ ...workflowStatus('DISCOVERY_RUNNING').sources[0], lease_id: '317f6fec-1994-4cfe-8ea6-bb7281d3050f', lease_expires_at: '2026-08-28T10:10:00Z' }] });
		render(SmartAccountsBatchRunner, { batchId: running.workflow.batch_id, workflow: running, ...runner });
		const button = screen.getByRole('button', { name: 'Reissue lost discovery action' });
		expect(button).toBeDisabled();
		await fireEvent.click(screen.getByLabelText(/I reconfirm metadata-only discovery for this exact paired company/));
		await fireEvent.click(button);
		await waitFor(() => expect(runner.onReissueDiscovery).toHaveBeenCalledWith(expect.objectContaining({ phase: 'DISCOVERY_RUNNING', source_company_id: 'sa-browser-v1-1234' })));
	});

	it('shows source-level review blockers and does not invent a schema or skip a selected company', () => {
		const runner = actions();
		const status = workflowStatus('SCHEMA_REVIEW_REQUIRED', {
			status: 'REVIEW_REQUIRED',
			sources: [
				{ ...workflowStatus('SCHEMA_REVIEW_REQUIRED').sources[0], reason_code: 'schema_not_registered' },
				{ ...workflowStatus('SCHEMA_REVIEW_REQUIRED').sources[0], source_company_id: 'sa-browser-v1-5678', tenant_id: 'tenant-2', ordinal: 1, phase: 'SCHEMA_APPROVED', phase_generation: 3 }
			]
		});
		render(SmartAccountsBatchRunner, {
			batchId: status.workflow.batch_id,
			workflow: status,
			companyNames: { 'sa-browser-v1-1234': 'Hold My Beer OÜ', 'sa-browser-v1-5678': 'Second Company OÜ' },
			...runner
		});

		expect(screen.getByText(/1 company needs review/)).toBeInTheDocument();
		expect(screen.getByText(/schema_not_registered/)).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: /Confirm transfer/ })).not.toBeInTheDocument();
		const continuation = screen.getAllByRole('link', { name: 'Open tenant review & apply' })[0];
		expect(continuation).toHaveAttribute('href', '/migration?tenant=tenant-1&workflow_batch=d436c224-5df5-4b4d-a772-1897f9147400&workflow_source=sa-browser-v1-1234');
		expect(continuation.getAttribute('href')).not.toContain('Hold');
		expect(continuation.getAttribute('href')).not.toContain(digest);
		const accountantReview = screen.getAllByRole('link', { name: 'Open accountant review' })[0];
		expect(accountantReview).toHaveAttribute('href', '/migration?tenant=tenant-1&reconciliation_batch=d436c224-5df5-4b4d-a772-1897f9147400&reconciliation_source=sa-browser-v1-1234');
		expect(accountantReview.getAttribute('href')).not.toContain('Hold');
		expect(accountantReview.getAttribute('href')).not.toContain(digest);
		expect(screen.getAllByRole('button', { name: 'Copy accountant review link' })).toHaveLength(2);
	});

	it('requires confirmation immediately before the immutable source transfer and never renders ephemeral capabilities', async () => {
		const runner = actions();
		const status = workflowStatus('TRANSFER_CONFIRMATION_REQUIRED', { schema_readiness_sha256: digest });
		render(SmartAccountsBatchRunner, {
			batchId: status.workflow.batch_id,
			workflow: status,
			companyNames: { 'sa-browser-v1-1234': 'Hold My Beer OÜ' },
			...runner
		});

		const confirm = screen.getByRole('button', { name: 'Confirm transfer for 1 companies' });
		expect(confirm).toBeDisabled();
		await fireEvent.click(screen.getByLabelText(/I confirm the exact reviewed schema set/));
		await fireEvent.click(confirm);
		await waitFor(() => expect(runner.onConfirmTransfer).toHaveBeenCalledWith({ owner_confirmed: true, expected_schema_sha256: digest }));
		expect(runner.onAdvanceConfirmedTransfer).toHaveBeenCalledTimes(1);
		expect(screen.getByText(/package is compiling/)).toBeInTheDocument();
		expect(screen.queryByText(/capture-token|relay-token|cookie/i)).not.toBeInTheDocument();
	});

	it('offers a server-serialized retry and a safe zero-tenant route without making a source transfer', async () => {
		const runner = actions();
		const retryable = workflowStatus('FAILED_RETRYABLE');
		render(SmartAccountsBatchRunner, { batchId: retryable.workflow.batch_id, workflow: retryable, ...runner });
		await fireEvent.click(screen.getByRole('button', { name: 'Continue safe workflow' }));
		await waitFor(() => expect(runner.onResume).toHaveBeenCalledTimes(1));
		expect(runner.onAdvanceConfirmedTransfer).not.toHaveBeenCalled();
		cleanup();

		const empty = workflowStatus('PAIRED', { sources: [] });
		render(SmartAccountsBatchRunner, { batchId: empty.workflow.batch_id, workflow: empty, ...actions() });
		expect(screen.getByText(/No paired Open Accounting tenants are available/)).toBeInTheDocument();
		expect(screen.getByRole('link', { name: 'Return to selected/all company onboarding' })).toHaveAttribute('href', '/migration');
	});

	it('uses the same Continue action to read and safely advance a fresh metadata-only step', async () => {
		const runner = actions();
		const fresh = workflowStatus('DISCOVERY_REQUIRED');
		render(SmartAccountsBatchRunner, { batchId: fresh.workflow.batch_id, workflow: fresh, ...runner });

		expect(screen.queryByRole('button', { name: 'Refresh safe batch status' })).not.toBeInTheDocument();
		await fireEvent.click(screen.getByRole('button', { name: 'Continue safe workflow' }));

		await waitFor(() => expect(runner.onRefresh).toHaveBeenCalledTimes(1));
		expect(runner.onResume).not.toHaveBeenCalled();
		expect(runner.onAdvanceSafe).toHaveBeenCalledTimes(1);
		expect(runner.onAdvanceConfirmedTransfer).not.toHaveBeenCalled();
	});
});
