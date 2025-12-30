// Mock for $app/navigation
export const goto = async (url: string) => {};
export const invalidate = async (url: string) => {};
export const invalidateAll = async () => {};
export const preloadData = async (url: string) => ({ type: 'loaded', status: 200, data: {} });
export const preloadCode = async (...urls: string[]) => {};
export const beforeNavigate = (callback: Function) => {};
export const afterNavigate = (callback: Function) => {};
export const onNavigate = (callback: Function) => {};
export const disableScrollHandling = () => {};
export const pushState = (url: string, state: any) => {};
export const replaceState = (url: string, state: any) => {};
