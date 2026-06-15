import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import FormModalHarness from './fixtures/FormModalHarness.svelte';

describe('FormModal', () => {
	afterEach(() => {
		cleanup();
	});

	it('renders dialog content, footer actions, and the requested size class', () => {
		const onClose = vi.fn();

		const { container } = render(FormModalHarness, {
			open: true,
			title: 'Edit contact',
			size: 'lg',
			onClose
		});

		const dialog = screen.getByRole('dialog', { name: 'Edit contact' });
		expect(dialog).toHaveAttribute('aria-modal', 'true');
		expect(dialog).toHaveClass('modal-lg');
		expect(screen.getByTestId('modal-body')).toHaveTextContent('Modal body content');
		expect(screen.getByRole('button', { name: 'Footer Action' })).toBeInTheDocument();
		expect(container.querySelector('.modal-footer')).toBeInTheDocument();
	});

	it('does not render the dialog when closed', () => {
		render(FormModalHarness, {
			open: false,
			title: 'Hidden modal',
			onClose: vi.fn()
		});

		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
	});

	it('closes from Escape and backdrop clicks but not inner dialog clicks', async () => {
		const onClose = vi.fn();

		const { container } = render(FormModalHarness, {
			open: true,
			title: 'Close me',
			onClose
		});

		await fireEvent.click(screen.getByRole('dialog', { name: 'Close me' }));
		expect(onClose).not.toHaveBeenCalled();

		const backdrop = container.querySelector('.modal-backdrop');
		if (!backdrop) throw new Error('Expected modal backdrop to render');
		await fireEvent.click(backdrop);
		expect(onClose).toHaveBeenCalledTimes(1);

		await fireEvent.keyDown(window, { key: 'Escape' });
		expect(onClose).toHaveBeenCalledTimes(2);
	});
});
