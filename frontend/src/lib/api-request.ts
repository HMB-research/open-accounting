export interface RetryConfig {
  maxRetries: number;
  baseDelayMs: number;
  maxDelayMs: number;
}

export const DEFAULT_RETRY_CONFIG: RetryConfig = {
  maxRetries: 3,
  baseDelayMs: 1000,
  maxDelayMs: 10000,
};

/**
 * Minimal retry config for testing - fast retries with minimal delay
 */
export const TEST_RETRY_CONFIG: RetryConfig = {
  maxRetries: 3,
  baseDelayMs: 10,
  maxDelayMs: 50,
};

/**
 * Check if an error is retryable (network errors or server errors)
 */
export function isRetryableError(error: unknown, status?: number): boolean {
  if (error instanceof TypeError && error.message.includes("fetch")) {
    return true;
  }

  if (status && status >= 500 && status <= 599) {
    return true;
  }

  return status === 429;
}

/**
 * Calculate delay with exponential backoff and jitter
 */
export function calculateBackoffDelay(
  attempt: number,
  config: RetryConfig,
): number {
  const exponentialDelay = config.baseDelayMs * Math.pow(2, attempt);
  const jitter = exponentialDelay * 0.25 * Math.random();

  return Math.min(exponentialDelay + jitter, config.maxDelayMs);
}

interface RequestOptions {
  skipAuth?: boolean;
  retryConfig?: RetryConfig;
  headers?: Record<string, string>;
  signal?: AbortSignal;
}

export interface ApiTransportDependencies {
  getApiBase: () => string;
  getAccessToken: () => string | null;
  getRefreshToken: () => string | null;
  refreshAccessToken: () => Promise<boolean>;
  clearTokens: () => void;
  getTenantContext: () => string | null;
  onSessionExpired: () => void;
}

export interface ApiTransport {
  request(
    method: string,
    path: string,
    body?: unknown,
    options?: RequestOptions,
  ): Promise<Response>;
  requestOnce(
    method: string,
    path: string,
    body?: unknown,
    options?: Omit<RequestOptions, "retryConfig">,
  ): Promise<Response>;
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function isFormDataBody(body: unknown): body is FormData {
  return typeof FormData !== "undefined" && body instanceof FormData;
}

function getRequestBody(body: unknown): BodyInit | undefined {
  if (!body) {
    return undefined;
  }

  return isFormDataBody(body) ? body : JSON.stringify(body);
}

function buildHeaders(
  path: string,
  body: unknown,
  options: RequestOptions,
  dependencies: ApiTransportDependencies,
): Record<string, string> {
  const headers = { ...options.headers };
  const isFormData = isFormDataBody(body);
  const accessToken = dependencies.getAccessToken();

  if (!options.skipAuth && accessToken) {
    headers.Authorization = `Bearer ${accessToken}`;
  }

  if (!options.skipAuth && path.startsWith("/api/v1/admin/")) {
    const tenantContext = dependencies.getTenantContext();
    if (tenantContext) {
      headers["X-Tenant-ID"] = tenantContext;
    }
  }

  if (body !== undefined && !isFormData) {
    headers["Content-Type"] = "application/json";
  }

  return headers;
}

export function createApiTransport(
  dependencies: ApiTransportDependencies,
): ApiTransport {
  async function requestOnce(
    method: string,
    path: string,
    body?: unknown,
    options: Omit<RequestOptions, "retryConfig"> = {},
  ): Promise<Response> {
    const requestOptions: RequestOptions = options;
    const requestInit: RequestInit = {
      method,
      headers: buildHeaders(path, body, requestOptions, dependencies),
      body: getRequestBody(body),
    };
    if (options.signal) {
      requestInit.signal = options.signal;
    }

    const response = await fetch(
      `${dependencies.getApiBase()}${path}`,
      requestInit,
    );

    if (response.status === 401 && !options.skipAuth) {
      if (dependencies.getRefreshToken()) {
        const refreshed = await dependencies.refreshAccessToken();
        if (refreshed) {
          return requestOnce(method, path, body, options);
        }
      }

      dependencies.clearTokens();
      dependencies.onSessionExpired();
      throw new Error("Session expired. Please log in again.");
    }

    return response;
  }

  async function request(
    method: string,
    path: string,
    body?: unknown,
    options: RequestOptions = {},
  ): Promise<Response> {
    const retryConfig = options.retryConfig ?? DEFAULT_RETRY_CONFIG;
    let lastError: Error | null = null;
    let lastStatus: number | undefined;

    for (let attempt = 0; attempt <= retryConfig.maxRetries; attempt++) {
      try {
        const response = await requestOnce(method, path, body, options);
        lastStatus = response.status;

        if (
          isRetryableError(null, response.status) &&
          attempt < retryConfig.maxRetries
        ) {
          await sleep(calculateBackoffDelay(attempt, retryConfig));
          continue;
        }

        return response;
      } catch (error) {
        lastError = error instanceof Error ? error : new Error(String(error));

        if (
          isRetryableError(error, lastStatus) &&
          attempt < retryConfig.maxRetries
        ) {
          await sleep(calculateBackoffDelay(attempt, retryConfig));
          continue;
        }

        throw lastError;
      }
    }

    throw lastError || new Error("Request failed after retries");
  }

  return { request, requestOnce };
}

export async function getApiResponseError(
  response: Response,
  fallbackMessage: string,
): Promise<Error> {
  let data: unknown = null;
  try {
    data = await response.json();
  } catch {
    // Non-JSON error responses use the caller's fallback message.
  }

  const message =
    data &&
    typeof data === "object" &&
    "error" in data &&
    typeof data.error === "string" &&
    data.error
      ? data.error
      : fallbackMessage;

  return new Error(message);
}
