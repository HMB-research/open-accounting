<script lang="ts">
	import { getLocale, setLocale, locales, type Locale } from '$lib/paraglide/runtime.js';

	let currentLocale = $state(getLocale());

	function switchLanguage(lang: Locale) {
		setLocale(lang);
		currentLocale = lang;
		// Store preference in localStorage for persistence
		if (typeof localStorage !== 'undefined') {
			localStorage.setItem('PARAGLIDE_LOCALE', lang);
		}
	}

	const languageNames: Record<Locale, string> = {
		en: 'English',
		et: 'Eesti'
	};
</script>

<div class="language-selector">
	<select
		value={currentLocale}
		onchange={(e) => switchLanguage((e.target as HTMLSelectElement).value as Locale)}
		aria-label="Select language"
	>
		{#each locales as lang (lang)}
			<option value={lang}>{languageNames[lang]}</option>
		{/each}
	</select>
</div>

<style>
	.language-selector select {
		padding: 0.375rem 0.5rem;
		font-size: 0.875rem;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-sm);
		background: var(--color-bg);
		color: var(--color-text);
		cursor: pointer;
	}

	.language-selector select:hover {
		border-color: var(--color-primary);
	}

	.language-selector select:focus {
		outline: none;
		border-color: var(--color-primary);
		box-shadow: 0 0 0 2px rgba(79, 70, 229, 0.2);
	}
</style>
