<script lang="ts">
	import { onMount } from 'svelte';
	import {
		api,
		type SMTPConfig,
		type EmailTemplate,
		type EmailLog,
		type EmailStatus,
		type TemplateType,
		type UpdateTemplateRequest
	} from '$lib/api';
	import * as m from '$lib/paraglide/messages.js';
	import StatusBadge, { type StatusConfig } from '$lib/components/StatusBadge.svelte';

	let selectedTenantId = $state('');
	let isLoading = $state(true);
	let error = $state('');
	let success = $state('');
	let activeTab = $state<'smtp' | 'templates' | 'log'>('smtp');

	// SMTP Config
	let smtpConfig = $state<SMTPConfig>({
		smtp_host: '',
		smtp_port: 587,
		smtp_username: '',
		smtp_password: '',
		smtp_from_email: '',
		smtp_from_name: '',
		smtp_use_tls: true
	});
	let isSavingSMTP = $state(false);
	let isTestingSMTP = $state(false);
	let testEmail = $state('');

	// Templates
	let templates = $state<EmailTemplate[]>([]);
	let selectedTemplate = $state<EmailTemplate | null>(null);
	let editingTemplate = $state<UpdateTemplateRequest | null>(null);
	let isSavingTemplate = $state(false);

	// Email Log
	let emailLog = $state<EmailLog[]>([]);

	onMount(async () => {
		const urlParams = new URLSearchParams(window.location.search);
		selectedTenantId = urlParams.get('tenant') || '';

		if (!selectedTenantId) {
			const tenants = await api.getMyTenants();
			if (tenants.length > 0) {
				selectedTenantId = tenants.find((t) => t.is_default)?.tenant.id || tenants[0].tenant.id;
			}
		}

		if (selectedTenantId) {
			await loadData();
		}
		isLoading = false;
	});

	async function loadData() {
		try {
			const [config, templateList, log] = await Promise.all([
				api.getSMTPConfig(selectedTenantId),
				api.listEmailTemplates(selectedTenantId),
				api.getEmailLog(selectedTenantId, 50)
			]);
			smtpConfig = config;
			templates = templateList;
			emailLog = log;
		} catch (err) {
			error = err instanceof Error ? err.message : m.errors_loadFailed();
		}
	}

	async function saveSMTPConfig(e: Event) {
		e.preventDefault();
		error = '';
		success = '';
		isSavingSMTP = true;

		try {
			await api.updateSMTPConfig(selectedTenantId, smtpConfig);
			success = m.email_smtpSaved();
		} catch (err) {
			error = err instanceof Error ? err.message : m.errors_saveFailed();
		} finally {
			isSavingSMTP = false;
		}
	}

	async function testSMTPConfig() {
		if (!testEmail) {
			error = m.email_enterTestEmail();
			return;
		}

		error = '';
		success = '';
		isTestingSMTP = true;

		try {
			const result = await api.testSMTP(selectedTenantId, testEmail);
			if (result.success) {
				success = result.message;
			} else {
				error = result.message;
			}
		} catch (err) {
			error = err instanceof Error ? err.message : m.errors_saveFailed();
		} finally {
			isTestingSMTP = false;
		}
	}

	function selectTemplate(template: EmailTemplate) {
		selectedTemplate = template;
		editingTemplate = {
			subject: template.subject,
			body_html: template.body_html,
			body_text: template.body_text || '',
			is_active: template.is_active
		};
	}

	async function saveTemplate(e: Event) {
		e.preventDefault();
		if (!selectedTemplate || !editingTemplate) return;

		error = '';
		success = '';
		isSavingTemplate = true;

		try {
			const updated = await api.updateEmailTemplate(
				selectedTenantId,
				selectedTemplate.template_type,
				editingTemplate
			);
			templates = templates.map((t) =>
				t.template_type === selectedTemplate!.template_type ? updated : t
			);
			selectedTemplate = updated;
			success = m.email_templateSaved();
		} catch (err) {
			error = err instanceof Error ? err.message : m.errors_saveFailed();
		} finally {
			isSavingTemplate = false;
		}
	}

	function templateTypeName(type: TemplateType): string {
		switch (type) {
			case 'INVOICE_SEND': return m.email_invoiceEmail();
			case 'PAYMENT_RECEIPT': return m.email_paymentReceipt();
			case 'OVERDUE_REMINDER': return m.email_overdueReminder();
			default: return type;
		}
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleString('en-GB');
	}

	const statusConfig: Record<EmailStatus, StatusConfig> = {
		PENDING: { class: 'badge-muted', label: 'PENDING' },
		SENT: { class: 'badge-success', label: 'SENT' },
		FAILED: { class: 'badge-danger', label: 'FAILED' }
	};
</script>

<svelte:head>
	<title>{m.email_title()} - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="header">
		<h1>{m.email_title()}</h1>
		<a href="/dashboard?tenant={selectedTenantId}" class="btn btn-secondary">{m.email_backToDashboard()}</a>
	</div>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if success}
		<div class="alert alert-success">{success}</div>
	{/if}

	{#if isLoading}
		<p>{m.common_loading()}</p>
	{:else}
		<div class="tabs">
			<button
				class="tab"
				class:active={activeTab === 'smtp'}
				onclick={() => (activeTab = 'smtp')}
			>
				{m.email_smtpConfig()}
			</button>
			<button
				class="tab"
				class:active={activeTab === 'templates'}
				onclick={() => (activeTab = 'templates')}
			>
				{m.email_templates()}
			</button>
			<button
				class="tab"
				class:active={activeTab === 'log'}
				onclick={() => (activeTab = 'log')}
			>
				{m.email_log()}
			</button>
		</div>

		{#if activeTab === 'smtp'}
			<div class="card">
				<h2>{m.email_smtpConfig()}</h2>
				<p class="text-muted">{m.email_smtpConfigDesc()}</p>

				<form onsubmit={saveSMTPConfig}>
					<div class="form-row">
						<div class="form-group">
							<label class="label" for="smtp_host">{m.email_smtpHost()}</label>
							<input
								class="input"
								type="text"
								id="smtp_host"
								bind:value={smtpConfig.smtp_host}
								placeholder="smtp.example.com"
							/>
						</div>
						<div class="form-group">
							<label class="label" for="smtp_port">{m.email_port()}</label>
							<input
								class="input"
								type="number"
								id="smtp_port"
								bind:value={smtpConfig.smtp_port}
								placeholder="587"
							/>
						</div>
					</div>

					<div class="form-row">
						<div class="form-group">
							<label class="label" for="smtp_username">{m.email_username()}</label>
							<input
								class="input"
								type="text"
								id="smtp_username"
								bind:value={smtpConfig.smtp_username}
								placeholder="user@example.com"
							/>
						</div>
						<div class="form-group">
							<label class="label" for="smtp_password">{m.email_password()}</label>
							<input
								class="input"
								type="password"
								id="smtp_password"
								bind:value={smtpConfig.smtp_password}
								placeholder=""
							/>
						</div>
					</div>

					<div class="form-row">
						<div class="form-group">
							<label class="label" for="smtp_from_email">{m.email_fromEmail()}</label>
							<input
								class="input"
								type="email"
								id="smtp_from_email"
								bind:value={smtpConfig.smtp_from_email}
								placeholder="invoices@example.com"
							/>
						</div>
						<div class="form-group">
							<label class="label" for="smtp_from_name">{m.email_fromName()}</label>
							<input
								class="input"
								type="text"
								id="smtp_from_name"
								bind:value={smtpConfig.smtp_from_name}
								placeholder="My Company"
							/>
						</div>
					</div>

					<div class="form-group">
						<label class="checkbox-label">
							<input type="checkbox" bind:checked={smtpConfig.smtp_use_tls} />
							{m.email_useTls()}
						</label>
					</div>

					<div class="form-actions">
						<button type="submit" class="btn btn-primary" disabled={isSavingSMTP}>
							{isSavingSMTP ? m.settings_saving() : m.settings_saveSettings()}
						</button>
					</div>
				</form>

				<hr />

				<h3>{m.email_testConfig()}</h3>
				<div class="test-form">
					<div class="form-group">
						<label class="label" for="test_email">{m.email_sendTestTo()}</label>
						<input
							class="input"
							type="email"
							id="test_email"
							bind:value={testEmail}
							placeholder="your@email.com"
						/>
					</div>
					<button
						type="button"
						class="btn btn-secondary"
						onclick={testSMTPConfig}
						disabled={isTestingSMTP}
					>
						{isTestingSMTP ? m.email_sending() : m.email_sendTest()}
					</button>
				</div>
			</div>
		{/if}

		{#if activeTab === 'templates'}
			<div class="templates-layout">
				<div class="template-list card">
					<h3>{m.email_templates()}</h3>
					{#if templates.length === 0}
						<p class="text-muted">{m.email_noTemplates()}</p>
					{/if}
					{#each templates as template}
						<button
							class="template-item"
							class:active={selectedTemplate?.template_type === template.template_type}
							onclick={() => selectTemplate(template)}
						>
							<div class="template-name">{templateTypeName(template.template_type)}</div>
							<span class="badge" class:badge-success={template.is_active} class:badge-muted={!template.is_active}>
								{template.is_active ? m.email_active() : m.email_inactive()}
							</span>
						</button>
					{/each}
				</div>

				<div class="template-editor card">
					{#if selectedTemplate && editingTemplate}
						<h3>{m.common_edit()} {templateTypeName(selectedTemplate.template_type)}</h3>
						<form onsubmit={saveTemplate}>
							<div class="form-group">
								<label class="label" for="template_subject">{m.email_subject()}</label>
								<input
									class="input"
									type="text"
									id="template_subject"
									bind:value={editingTemplate.subject}
								/>
								<small class="text-muted">
									{m.email_availableVars()}: {'{{.CompanyName}}'}, {'{{.ContactName}}'}, {'{{.InvoiceNumber}}'}, {'{{.TotalAmount}}'}, {'{{.Currency}}'}, {'{{.DueDate}}'}
								</small>
							</div>

							<div class="form-group">
								<label class="label" for="template_body">{m.email_htmlBody()}</label>
								<textarea
									class="input textarea"
									id="template_body"
									bind:value={editingTemplate.body_html}
									rows="15"
								></textarea>
							</div>

							<div class="form-group">
								<label class="checkbox-label">
									<input type="checkbox" bind:checked={editingTemplate.is_active} />
									{m.email_templateActive()}
								</label>
							</div>

							<div class="form-actions">
								<button type="submit" class="btn btn-primary" disabled={isSavingTemplate}>
									{isSavingTemplate ? m.settings_saving() : m.email_saveTemplate()}
								</button>
							</div>
						</form>
					{:else}
						<p class="text-muted">{m.email_selectTemplate()}</p>
					{/if}
				</div>
			</div>
		{/if}

		{#if activeTab === 'log'}
			<div class="card">
				<h2>{m.email_log()}</h2>
				{#if emailLog.length === 0}
					<p class="text-muted">{m.email_noEmailsSent()}</p>
				{:else}
					<div class="table-container">
						<table class="table table-mobile-cards">
							<thead>
								<tr>
									<th>{m.common_date()}</th>
									<th>{m.email_type()}</th>
									<th>{m.email_recipient()}</th>
									<th>{m.email_subject()}</th>
									<th>{m.common_status()}</th>
								</tr>
							</thead>
							<tbody>
								{#each emailLog as log}
									<tr>
										<td data-label={m.common_date()}>{formatDate(log.created_at)}</td>
										<td data-label={m.email_type()}>{templateTypeName(log.email_type as TemplateType)}</td>
										<td data-label={m.email_recipient()}>
											{log.recipient_name || log.recipient_email}
											{#if log.recipient_name}
												<br /><small class="text-muted">{log.recipient_email}</small>
											{/if}
										</td>
										<td data-label={m.email_subject()}>{log.subject}</td>
										<td data-label={m.common_status()}>
											<StatusBadge status={log.status} config={statusConfig} />
											{#if log.error_message}
												<br /><small class="text-muted">{log.error_message}</small>
											{/if}
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{/if}
			</div>
		{/if}
	{/if}
</div>

<style>
	.header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 1.5rem;
	}

	h1 {
		font-size: 1.75rem;
	}

	h2 {
		font-size: 1.25rem;
		margin-bottom: 0.5rem;
	}

	h3 {
		font-size: 1rem;
		margin-bottom: 1rem;
	}

	.text-muted {
		color: var(--color-text-muted);
	}

	.tabs {
		display: flex;
		gap: 0.5rem;
		margin-bottom: 1.5rem;
		border-bottom: 1px solid var(--color-border);
		padding-bottom: 0.5rem;
	}

	.tab {
		padding: 0.5rem 1rem;
		border: none;
		background: none;
		cursor: pointer;
		border-radius: var(--radius-sm);
		color: var(--color-text-muted);
	}

	.tab:hover {
		background: var(--color-border);
	}

	.tab.active {
		background: var(--color-primary);
		color: white;
	}

	.form-row {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 1rem;
	}

	.form-actions {
		margin-top: 1.5rem;
	}

	.checkbox-label {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		cursor: pointer;
	}

	hr {
		border: none;
		border-top: 1px solid var(--color-border);
		margin: 1.5rem 0;
	}

	.test-form {
		display: flex;
		gap: 1rem;
		align-items: flex-end;
	}

	.test-form .form-group {
		flex: 1;
		margin-bottom: 0;
	}

	.templates-layout {
		display: grid;
		grid-template-columns: 250px 1fr;
		gap: 1.5rem;
	}

	.template-list h3 {
		margin-bottom: 0.75rem;
	}

	.template-item {
		display: flex;
		justify-content: space-between;
		align-items: center;
		width: 100%;
		padding: 0.75rem;
		border: 1px solid var(--color-border);
		background: none;
		cursor: pointer;
		text-align: left;
		border-radius: var(--radius-sm);
		margin-bottom: 0.5rem;
	}

	.template-item:hover {
		background: var(--color-border);
	}

	.template-item.active {
		border-color: var(--color-primary);
		background: rgba(29, 78, 216, 0.1);
	}

	.template-name {
		font-weight: 500;
	}

	.textarea {
		min-height: 300px;
		font-family: var(--font-mono);
		font-size: 0.875rem;
	}

	small {
		font-size: 0.75rem;
	}

	.table-container {
		overflow-x: auto;
	}

	.table {
		width: 100%;
		border-collapse: collapse;
	}

	.table th,
	.table td {
		padding: 0.75rem;
		text-align: left;
		border-bottom: 1px solid var(--color-border);
	}

	.table th {
		font-weight: 600;
		color: var(--color-text-muted);
		font-size: 0.75rem;
		text-transform: uppercase;
	}

	.badge {
		display: inline-block;
		padding: 0.25rem 0.5rem;
		border-radius: var(--radius-sm);
		font-size: 0.75rem;
		font-weight: 500;
	}

	.alert-success {
		background: rgba(34, 197, 94, 0.1);
		color: #22c55e;
		padding: 1rem;
		border-radius: var(--radius-md);
		margin-bottom: 1rem;
	}

	/* Mobile styles */
	@media (max-width: 768px) {
		.header {
			flex-direction: column;
			align-items: stretch;
			gap: 1rem;
		}

		h1 {
			font-size: 1.5rem;
		}

		.tabs {
			flex-wrap: wrap;
		}

		.tabs button {
			flex: 1;
			min-width: 100px;
			min-height: 44px;
			justify-content: center;
		}

		.form-row {
			flex-direction: column;
		}

		.templates-layout {
			grid-template-columns: 1fr;
		}

		.template-item {
			min-height: 44px;
		}

		.textarea {
			min-height: 200px;
		}

		.btn {
			min-height: 44px;
			justify-content: center;
		}
	}
</style>
