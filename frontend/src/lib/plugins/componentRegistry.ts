import type { Component } from 'svelte';
import type { PluginSlotRegistration } from './manager';

export type PluginFrontendComponentProps = Record<string, unknown> & {
	registration: PluginSlotRegistration;
};

export type PluginFrontendComponent = Component<PluginFrontendComponentProps>;

const registeredComponents = new Map<string, PluginFrontendComponent>();

/**
 * Register Svelte components that were bundled by the operator at build time.
 * Plugin manifests only name component references; they never become import paths.
 */
export function registerPluginFrontendComponent(
	componentId: string,
	component: PluginFrontendComponent
): void {
	const normalizedId = normalizePluginComponentReference(componentId);
	if (!normalizedId) {
		throw new Error(`Unsafe plugin frontend component id: ${componentId}`);
	}

	registeredComponents.set(normalizedId, component);
}

export function unregisterPluginFrontendComponent(componentId: string): void {
	const normalizedId = normalizePluginComponentReference(componentId);
	if (!normalizedId) {
		return;
	}

	registeredComponents.delete(normalizedId);
}

export function clearPluginFrontendComponents(): void {
	registeredComponents.clear();
}

export function resolvePluginFrontendComponent(
	registration: PluginSlotRegistration
): PluginFrontendComponent | undefined {
	for (const componentId of getPluginFrontendComponentCandidateIds(registration)) {
		const component = registeredComponents.get(componentId);
		if (component) {
			return component;
		}
	}

	return undefined;
}

export function getPluginFrontendComponentCandidateIds(
	registration: PluginSlotRegistration
): string[] {
	if (!registration.componentRef) {
		return [];
	}

	const candidates = [
		`${registration.pluginId}/${registration.componentRef}`,
		`${registration.pluginName}/${registration.componentRef}`
	];
	const safeCandidates = candidates.flatMap((candidate) => {
		const normalized = normalizePluginComponentReference(candidate);
		return normalized ? [normalized] : [];
	});

	return [...new Set(safeCandidates)];
}

export function normalizePluginComponentReference(componentRef?: string): string | undefined {
	if (!componentRef) {
		return undefined;
	}

	if (componentRef !== componentRef.trim() || componentRef.length > 160) {
		return undefined;
	}

	if (
		componentRef.startsWith('/') ||
		componentRef.startsWith('.') ||
		componentRef.startsWith('~') ||
		componentRef.includes('\\') ||
		componentRef.includes('?') ||
		componentRef.includes('#') ||
		componentRef.includes('\0') ||
		/^[a-z][a-z0-9+.-]*:/i.test(componentRef)
	) {
		return undefined;
	}

	const segments = componentRef.split('/');
	if (segments.some((segment) => !segment || segment === '.' || segment === '..')) {
		return undefined;
	}

	if (!segments.every((segment) => /^[A-Za-z0-9][A-Za-z0-9._-]*$/.test(segment))) {
		return undefined;
	}

	return componentRef;
}
