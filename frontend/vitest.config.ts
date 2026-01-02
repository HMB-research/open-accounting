import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
	plugins: [svelte({ hot: !process.env.VITEST })],
	test: {
		include: ['src/**/*.{test,spec}.{js,ts}'],
		globals: true,
		environment: 'jsdom',
		setupFiles: ['./src/tests/setup.ts'],
		coverage: {
			provider: 'v8',
			reporter: ['text', 'json', 'html'],
			include: ['src/lib/**/*.ts', 'src/routes/**/*.ts'],
			exclude: [
				'node_modules/**',
				'src/tests/**',
				'src/lib/paraglide/**',
				'**/*.d.ts',
				'**/*.config.{js,ts}'
			]
		}
	},
	resolve: {
		alias: {
			$lib: '/src/lib',
			$app: '/src/tests/mocks/app',
			'$env/dynamic/public': '/src/tests/mocks/env/dynamic/public'
		}
	}
});
