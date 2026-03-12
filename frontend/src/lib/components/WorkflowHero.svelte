<script lang="ts">
	export type WorkflowHeroAction = {
		label: string;
		variant?: 'primary' | 'secondary';
		href?: string;
		onclick?: () => void;
		disabled?: boolean;
		type?: 'button' | 'submit';
		form?: string;
	};

	export type WorkflowHeroStat = {
		label: string;
		value: string;
		detail?: string;
		tone?: 'default' | 'success' | 'warning' | 'danger';
		href?: string;
	};

	export type WorkflowHeroAside = {
		kicker?: string;
		title: string;
		body: string;
		linkLabel?: string;
		href?: string;
		items?: string[];
	};

	interface Props {
		eyebrow?: string;
		title: string;
		description: string;
		backHref?: string;
		backLabel?: string;
		badgeLabel?: string;
		badgeTone?: 'neutral' | 'success' | 'warning' | 'danger';
		actions?: WorkflowHeroAction[];
		stats?: WorkflowHeroStat[];
		aside?: WorkflowHeroAside | null;
	}

	let {
		eyebrow = '',
		title,
		description,
		backHref,
		backLabel,
		badgeLabel,
		badgeTone = 'neutral',
		actions = [],
		stats = [],
		aside = null
	}: Props = $props();
</script>

<section class="workflow-hero card" class:with-aside={Boolean(aside)}>
	<div class="workflow-main">
		{#if backHref && backLabel}
			<a class="workflow-back-link" href={backHref}>
				&larr; {backLabel}
			</a>
		{/if}

		<div class="workflow-topline">
			{#if eyebrow}
				<span class="workflow-eyebrow">{eyebrow}</span>
			{/if}
			{#if badgeLabel}
				<span class="workflow-badge" data-tone={badgeTone}>{badgeLabel}</span>
			{/if}
		</div>

		<div class="workflow-heading-row">
			<div class="workflow-copy">
				<h1>{title}</h1>
				<p>{description}</p>
			</div>

			{#if actions.length > 0}
				<div class="workflow-actions">
					{#each actions as action}
						{#if action.href}
							<a
								class={`btn ${action.variant === 'secondary' ? 'btn-secondary' : 'btn-primary'}`}
								href={action.href}
								aria-disabled={action.disabled}
							>
								{action.label}
							</a>
						{:else}
							<button
								type={action.type || 'button'}
								form={action.form}
								class={`btn ${action.variant === 'secondary' ? 'btn-secondary' : 'btn-primary'}`}
								onclick={action.onclick}
								disabled={action.disabled}
							>
								{action.label}
							</button>
						{/if}
					{/each}
				</div>
			{/if}
		</div>

		{#if stats.length > 0}
			<div class="workflow-stats">
				{#each stats as stat}
					{#if stat.href}
						<a class="workflow-stat" data-tone={stat.tone || 'default'} href={stat.href}>
							<span class="workflow-stat-label">{stat.label}</span>
							<strong>{stat.value}</strong>
							{#if stat.detail}
								<span class="workflow-stat-detail">{stat.detail}</span>
							{/if}
						</a>
					{:else}
						<div class="workflow-stat" data-tone={stat.tone || 'default'}>
							<span class="workflow-stat-label">{stat.label}</span>
							<strong>{stat.value}</strong>
							{#if stat.detail}
								<span class="workflow-stat-detail">{stat.detail}</span>
							{/if}
						</div>
					{/if}
				{/each}
			</div>
		{/if}
	</div>

	{#if aside}
		<aside class="workflow-aside">
			{#if aside.kicker}
				<div class="workflow-aside-kicker">{aside.kicker}</div>
			{/if}
			<h2>{aside.title}</h2>
			<p>{aside.body}</p>

			{#if aside.items?.length}
				<ul class="workflow-aside-list">
					{#each aside.items as item}
						<li>{item}</li>
					{/each}
				</ul>
			{/if}

			{#if aside.href && aside.linkLabel}
				<a class="workflow-aside-link" href={aside.href}>{aside.linkLabel}</a>
			{/if}
		</aside>
	{/if}
</section>

<style>
	.workflow-hero {
		display: grid;
		gap: 1.5rem;
		padding: 1.75rem;
		border-radius: var(--radius-lg);
		background:
			linear-gradient(135deg, rgba(255, 255, 255, 0.78), rgba(255, 250, 241, 0.92)),
			var(--color-card);
	}

	.workflow-hero.with-aside {
		grid-template-columns: minmax(0, 1.7fr) minmax(18rem, 0.95fr);
		align-items: stretch;
	}

	.workflow-main {
		display: flex;
		flex-direction: column;
		gap: 1.25rem;
	}

	.workflow-back-link {
		display: inline-flex;
		align-items: center;
		gap: 0.35rem;
		width: fit-content;
		color: var(--color-text-muted);
		font-size: 0.9rem;
	}

	.workflow-back-link:hover {
		text-decoration: none;
		color: var(--color-primary);
	}

	.workflow-topline {
		display: flex;
		flex-wrap: wrap;
		gap: 0.75rem;
		align-items: center;
	}

	.workflow-eyebrow {
		font-size: 0.76rem;
		font-weight: 700;
		letter-spacing: 0.18em;
		text-transform: uppercase;
		color: rgba(15, 23, 42, 0.48);
	}

	.workflow-badge {
		display: inline-flex;
		align-items: center;
		padding: 0.3rem 0.7rem;
		border-radius: 999px;
		font-size: 0.76rem;
		font-weight: 700;
		border: 1px solid rgba(15, 23, 42, 0.08);
		background: rgba(255, 255, 255, 0.72);
	}

	.workflow-badge[data-tone='success'] {
		color: #166534;
		background: rgba(220, 252, 231, 0.9);
	}

	.workflow-badge[data-tone='warning'] {
		color: #92400e;
		background: rgba(254, 243, 199, 0.9);
	}

	.workflow-badge[data-tone='danger'] {
		color: #991b1b;
		background: rgba(254, 226, 226, 0.9);
	}

	.workflow-heading-row {
		display: flex;
		gap: 1rem;
		justify-content: space-between;
		align-items: flex-start;
	}

	.workflow-copy {
		max-width: 46rem;
	}

	.workflow-copy h1 {
		margin: 0;
		font-size: clamp(2rem, 3.6vw, 3.15rem);
		line-height: 0.92;
		letter-spacing: -0.03em;
		font-family: var(--font-display);
		font-weight: 600;
	}

	.workflow-copy p {
		margin-top: 0.8rem;
		max-width: 40rem;
		font-size: 1rem;
		color: var(--color-text-muted);
	}

	.workflow-actions {
		display: flex;
		flex-wrap: wrap;
		gap: 0.65rem;
		justify-content: flex-end;
	}

	.workflow-actions :global(.btn) {
		justify-content: center;
		white-space: nowrap;
	}

	.workflow-stats {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
		gap: 0.85rem;
	}

	.workflow-stat {
		display: flex;
		flex-direction: column;
		gap: 0.35rem;
		padding: 1rem;
		border-radius: 1rem;
		border: 1px solid rgba(15, 23, 42, 0.08);
		background: rgba(255, 255, 255, 0.72);
		color: inherit;
		text-decoration: none;
	}

	.workflow-stat:hover {
		text-decoration: none;
	}

	.workflow-stat[data-tone='success'] {
		background: rgba(220, 252, 231, 0.62);
	}

	.workflow-stat[data-tone='warning'] {
		background: rgba(254, 243, 199, 0.68);
	}

	.workflow-stat[data-tone='danger'] {
		background: rgba(254, 226, 226, 0.68);
	}

	.workflow-stat-label,
	.workflow-stat-detail {
		font-size: 0.83rem;
		color: var(--color-text-muted);
	}

	.workflow-stat strong {
		font-size: 1.35rem;
		line-height: 1;
	}

	.workflow-aside {
		display: flex;
		flex-direction: column;
		justify-content: space-between;
		gap: 1rem;
		padding: 1.35rem;
		border-radius: 1.25rem;
		background:
			linear-gradient(160deg, rgba(15, 23, 42, 0.98), rgba(30, 41, 59, 0.9));
		color: rgba(255, 255, 255, 0.9);
		box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.06);
	}

	.workflow-aside-kicker {
		font-size: 0.76rem;
		letter-spacing: 0.16em;
		text-transform: uppercase;
		color: rgba(191, 219, 254, 0.78);
	}

	.workflow-aside h2 {
		margin: 0;
		font-size: 1.4rem;
		font-family: var(--font-display);
		font-weight: 600;
	}

	.workflow-aside p {
		margin: 0;
		color: rgba(226, 232, 240, 0.85);
	}

	.workflow-aside-list {
		display: grid;
		gap: 0.55rem;
		padding-left: 1rem;
		color: rgba(226, 232, 240, 0.9);
	}

	.workflow-aside-link {
		display: inline-flex;
		align-items: center;
		gap: 0.35rem;
		font-weight: 600;
		color: #bfdbfe;
	}

	.workflow-aside-link:hover {
		text-decoration: none;
		color: white;
	}

	@media (max-width: 900px) {
		.workflow-hero.with-aside {
			grid-template-columns: 1fr;
		}

		.workflow-heading-row {
			flex-direction: column;
		}

		.workflow-actions {
			width: 100%;
			justify-content: flex-start;
		}
	}

	@media (max-width: 640px) {
		.workflow-hero {
			padding: 1.25rem;
		}

		.workflow-actions {
			flex-direction: column;
		}

		.workflow-actions :global(.btn) {
			width: 100%;
		}

		.workflow-stats {
			grid-template-columns: 1fr;
		}
	}
</style>
