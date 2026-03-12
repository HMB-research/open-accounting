import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';

const srcDir = fileURLToPath(new URL('./src', import.meta.url));
const libDir = fileURLToPath(new URL('./src/lib', import.meta.url));
const appMocksDir = fileURLToPath(new URL('./src/tests/mocks/app', import.meta.url));
const envPublicMock = fileURLToPath(new URL('./src/tests/mocks/env/dynamic/public.ts', import.meta.url));

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
		conditions: ['browser'],
		alias: {
			$lib: libDir,
			$app: appMocksDir,
			'$env/dynamic/public': envPublicMock,
			'/src': srcDir
		}
	}
});
