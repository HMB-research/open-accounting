// Mock for $app/stores
import { writable, readable } from 'svelte/store';

export const page = readable({
	url: new URL('http://localhost'),
	params: {},
	route: { id: null },
	status: 200,
	error: null,
	data: {},
	form: null,
	state: {}
});

export const navigating = readable(null);

export const updated = {
	subscribe: readable(false).subscribe,
	check: async () => false
};
