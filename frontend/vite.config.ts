import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import { svelteTesting } from '@testing-library/svelte/vite';

export default defineConfig({
	plugins: [sveltekit(), svelteTesting()],
	test: {
		environment: 'jsdom',
		setupFiles: ['src/test/setup.ts'],
		include: ['src/**/*.test.ts'],
		coverage: {
			provider: 'v8',
			reporter: ['text', 'html'],
			reportsDirectory: '../output/frontend-coverage',
			include: ['src/lib/**/*.ts'],
			exclude: [
				'src/**/*.test.ts',
				'src/**/*.svelte.ts',
				'src/**/*.d.ts',
				'src/lib/constants.ts',
				'src/lib/stores.ts'
			]
		}
	},
	server: {
		proxy: {
			'/api': {
				target: 'http://localhost:8080',
				changeOrigin: true,
				rewrite: (path) => path.replace(/^\/api/, '')
			}
		}
	}
});
