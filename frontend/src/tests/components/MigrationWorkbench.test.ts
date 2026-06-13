import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import type {
	BundleValidationReport,
	MigrationExecutionPlan,
	MigrationExecutionRun
} from '$lib/api';

const { apiMock } = vi.hoisted(() => ({
	apiMock: {
		validateMigrationBundle: vi.fn(),
		planMigrationExecution: vi.fn(),
		executeMigration: vi.fn(),
		listMigrationExecutionRuns: vi.fn(),
		getMigrationExecutionRun: vi.fn()
	}
}));

vi.mock('$lib/api', async () => {
	const actual = await vi.importActual<typeof import('$lib/api')>('$lib/api');
	return {
		...actual,
		api: apiMock
	};
});

import MigrationWorkbench from '$lib/components/MigrationWorkbench.svelte';

function validationReport(overrides: Partial<BundleValidationReport> = {}): BundleValidationReport {
	return {
		summary: {
			files_validated: 1,
			rows_validated: 2,
			error_count: 0,
			warning_count: 0,
			ready: true
		},
		files: [
			{
				kind: 'contacts',
				file_name: 'contacts.csv',
				rows: 2
			}
		],
		issues: [],
		remediation_actions: [],
		...overrides
	};
}

function executionPlan(): MigrationExecutionPlan {
	return {
		summary: {
			validation_ready: true,
			ready: true,
			step_count: 1,
			ready_step_count: 1,
			needs_context_count: 0,
			blocked_step_count: 0
		},
		validation: validationReport(),
		steps: [
			{
				step_number: 1,
				kind: 'contacts',
				file_name: 'contacts.csv',
				status: 'READY',
				message: 'Ready to import.',
				action: 'Import contacts.',
				api_method: 'POST',
				api_path: '/api/v1/tenants/tenant-1/contacts/import',
				cli_command: 'oa contacts import --file contacts.csv'
			}
		],
		remediation_actions: []
	};
}

function executionRun(overrides: Partial<MigrationExecutionRun> = {}): MigrationExecutionRun {
	return {
		id: 'run-1',
		tenant_id: 'tenant-1',
		created_at: '2026-06-14T09:00:00Z',
		updated_at: '2026-06-14T09:01:00Z',
		summary: {
			status: 'succeeded',
			confirmed: true,
			resumed: false,
			plan_ready: true,
			validation_ready: true,
			step_count: 1,
			running_step_count: 0,
			succeeded_step_count: 1,
			failed_step_count: 0,
			skipped_step_count: 0,
			planned_step_count: 0,
			resumed_step_count: 0,
			completed_step_count: 1,
			remaining_step_count: 0,
			progress_percent: 100,
			needs_context_count: 0,
			blocked_step_count: 0
		},
		plan: executionPlan(),
		steps: [
			{
				step_number: 1,
				kind: 'contacts',
				file_name: 'contacts.csv',
				status: 'SUCCEEDED',
				message: 'Imported contacts.',
				api_path: '/api/v1/tenants/tenant-1/contacts/import',
				cli_command: 'oa contacts import --file contacts.csv'
			}
		],
		remediation_actions: [],
		...overrides
	};
}

function runningExecutionRun(): MigrationExecutionRun {
	const base = executionRun();
	return {
		...base,
		summary: {
			...base.summary,
			status: 'running',
			running_step_count: 1,
			succeeded_step_count: 1,
			completed_step_count: 1,
			remaining_step_count: 1,
			progress_percent: 50,
			active_step_number: 2,
			active_step_kind: 'contacts',
			active_step_file_name: 'contacts-next.csv',
			active_step_status: 'RUNNING',
			step_count: 2
		},
		steps: [
			...(base.steps ?? []),
			{
				step_number: 2,
				kind: 'contacts',
				file_name: 'contacts-next.csv',
				status: 'RUNNING',
				message: 'Import running.',
				api_path: '/api/v1/tenants/tenant-1/contacts/import',
				cli_command: 'oa contacts import --file contacts-next.csv'
			}
		]
	};
}

async function addContactsFile() {
	await fireEvent.change(screen.getByLabelText('File kind'), {
		target: { value: 'contacts' }
	});
	await fireEvent.input(screen.getByLabelText('CSV or XML content'), {
		target: { value: 'name\nAcme OU\n' }
	});
	await fireEvent.click(screen.getByRole('button', { name: 'Add text file' }));
	await waitFor(() => expect(screen.getByText('contacts.csv')).toBeInTheDocument());
}

describe('MigrationWorkbench', () => {
	afterEach(() => {
		cleanup();
	});

	beforeEach(() => {
		vi.clearAllMocks();
		apiMock.listMigrationExecutionRuns.mockResolvedValue([]);
		apiMock.validateMigrationBundle.mockResolvedValue(validationReport());
		apiMock.planMigrationExecution.mockResolvedValue(executionPlan());
		apiMock.executeMigration.mockResolvedValue(executionRun());
		apiMock.getMigrationExecutionRun.mockResolvedValue(executionRun());
	});

	it('builds an execution plan from a pasted migration bundle', async () => {
		render(MigrationWorkbench, { tenantId: 'tenant-1' });

		await waitFor(() =>
			expect(apiMock.listMigrationExecutionRuns).toHaveBeenCalledWith('tenant-1', {
				status: undefined,
				limit: 10
			})
		);
		await addContactsFile();
		await fireEvent.click(screen.getByRole('button', { name: 'Build plan' }));

		await waitFor(() => expect(apiMock.planMigrationExecution).toHaveBeenCalledTimes(1));
		expect(apiMock.planMigrationExecution).toHaveBeenCalledWith('tenant-1', {
			files: [
				{
					kind: 'contacts',
					file_name: 'contacts.csv',
					csv_content: 'name\nAcme OU\n'
				}
			],
			provider_preset: 'generic',
			e_invoice_contact_mode: 'supplier'
		});
		expect(await screen.findByText('Migration execution plan is ready.')).toBeInTheDocument();
		expect(screen.getByText('oa contacts import --file contacts.csv')).toBeInTheDocument();
	});

	it('opens saved execution runs for monitoring', async () => {
		const monitoringRun = runningExecutionRun();
		apiMock.listMigrationExecutionRuns.mockResolvedValue([monitoringRun]);
		apiMock.getMigrationExecutionRun.mockResolvedValue(monitoringRun);
		render(MigrationWorkbench, { tenantId: 'tenant-1' });

		expect(await screen.findByText('run-1')).toBeInTheDocument();
		await fireEvent.click(screen.getByRole('button', { name: 'Open' }));

		await waitFor(() =>
			expect(apiMock.getMigrationExecutionRun).toHaveBeenCalledWith('tenant-1', 'run-1')
		);
		expect(await screen.findByText('Saved migration run loaded.')).toBeInTheDocument();
		expect(screen.getByText('Import running.')).toBeInTheDocument();
		expect(screen.getAllByText('50%').length).toBeGreaterThan(0);
		expect(screen.getByText('Active step: #2 RUNNING contacts contacts-next.csv')).toBeInTheDocument();
	});

	it('executes a confirmed cutover with a selected resume run id', async () => {
		apiMock.listMigrationExecutionRuns.mockResolvedValue([executionRun()]);
		render(MigrationWorkbench, { tenantId: 'tenant-1' });

		expect(await screen.findByText('run-1')).toBeInTheDocument();
		await addContactsFile();
		await fireEvent.click(screen.getByRole('button', { name: 'Resume' }));
		expect(screen.getByLabelText('Resume run ID')).toHaveValue('run-1');
		await fireEvent.click(screen.getByLabelText('Confirm execution'));
		await fireEvent.click(screen.getByRole('button', { name: 'Execute confirmed cutover' }));

		await waitFor(() => expect(apiMock.executeMigration).toHaveBeenCalledTimes(1));
		expect(apiMock.executeMigration).toHaveBeenCalledWith(
			'tenant-1',
			expect.objectContaining({
				confirm: true,
				resume_from_run_id: 'run-1',
				files: [
					{
						kind: 'contacts',
						file_name: 'contacts.csv',
						csv_content: 'name\nAcme OU\n'
					}
				]
			})
		);
		expect(await screen.findByText('Migration execution run completed.')).toBeInTheDocument();
	});
});
