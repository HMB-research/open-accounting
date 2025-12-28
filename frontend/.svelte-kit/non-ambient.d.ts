
// this file is generated — do not edit it


declare module "svelte/elements" {
	export interface HTMLAttributes<T> {
		'data-sveltekit-keepfocus'?: true | '' | 'off' | undefined | null;
		'data-sveltekit-noscroll'?: true | '' | 'off' | undefined | null;
		'data-sveltekit-preload-code'?:
			| true
			| ''
			| 'eager'
			| 'viewport'
			| 'hover'
			| 'tap'
			| 'off'
			| undefined
			| null;
		'data-sveltekit-preload-data'?: true | '' | 'hover' | 'tap' | 'off' | undefined | null;
		'data-sveltekit-reload'?: true | '' | 'off' | undefined | null;
		'data-sveltekit-replacestate'?: true | '' | 'off' | undefined | null;
	}
}

export {};


declare module "$app/types" {
	export interface AppTypes {
		RouteId(): "/" | "/accounts" | "/contacts" | "/dashboard" | "/invoices" | "/journal" | "/login" | "/payments" | "/reports";
		RouteParams(): {
			
		};
		LayoutParams(): {
			"/": Record<string, never>;
			"/accounts": Record<string, never>;
			"/contacts": Record<string, never>;
			"/dashboard": Record<string, never>;
			"/invoices": Record<string, never>;
			"/journal": Record<string, never>;
			"/login": Record<string, never>;
			"/payments": Record<string, never>;
			"/reports": Record<string, never>
		};
		Pathname(): "/" | "/accounts" | "/accounts/" | "/contacts" | "/contacts/" | "/dashboard" | "/dashboard/" | "/invoices" | "/invoices/" | "/journal" | "/journal/" | "/login" | "/login/" | "/payments" | "/payments/" | "/reports" | "/reports/";
		ResolvedPathname(): `${"" | `/${string}`}${ReturnType<AppTypes['Pathname']>}`;
		Asset(): string & {};
	}
}