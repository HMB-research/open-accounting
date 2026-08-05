import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  createApiTransport,
  getApiResponseError,
  type ApiTransportDependencies,
} from "$lib/api-request";

describe("API request transport", () => {
  const mockFetch = vi.fn();
  let accessToken: string | null;

  beforeEach(() => {
    accessToken = "access-token";
    vi.stubGlobal("fetch", mockFetch);
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  function createDependencies(
    overrides: Partial<ApiTransportDependencies> = {},
  ): ApiTransportDependencies {
    return {
      getApiBase: () => "https://api.example.com",
      getAccessToken: () => accessToken,
      getRefreshToken: () => "refresh-token",
      refreshAccessToken: vi.fn(),
      clearTokens: vi.fn(),
      getTenantContext: () => "tenant-from-url",
      onSessionExpired: vi.fn(),
      ...overrides,
    };
  }

  it("builds authenticated JSON requests with admin tenant context", async () => {
    mockFetch.mockResolvedValueOnce({ ok: true, status: 200 });
    const transport = createApiTransport(createDependencies());

    await transport.request("POST", "/api/v1/admin/plugins/permissions", {
      enabled: true,
    });

    expect(mockFetch).toHaveBeenCalledWith(
      "https://api.example.com/api/v1/admin/plugins/permissions",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ enabled: true }),
        headers: {
          Authorization: "Bearer access-token",
          "X-Tenant-ID": "tenant-from-url",
          "Content-Type": "application/json",
        },
      }),
    );
  });

  it("leaves multipart content types to the browser", async () => {
    mockFetch.mockResolvedValueOnce({ ok: true, status: 201 });
    const transport = createApiTransport(createDependencies());
    const formData = new FormData();
    formData.set("file", "receipt");

    await transport.requestOnce(
      "POST",
      "/api/v1/tenants/tenant-1/documents",
      formData,
    );

    const [, requestInit] = mockFetch.mock.calls[0] as [string, RequestInit];
    expect(requestInit.body).toBe(formData);
    expect(requestInit.headers).toEqual({
      Authorization: "Bearer access-token",
    });
  });

  it("refreshes once and retries with the updated access token", async () => {
    const refreshAccessToken = vi.fn().mockImplementation(async () => {
      accessToken = "new-access-token";
      return true;
    });
    mockFetch
      .mockResolvedValueOnce({ ok: false, status: 401 })
      .mockResolvedValueOnce({ ok: true, status: 200 });
    const transport = createApiTransport(
      createDependencies({ refreshAccessToken }),
    );

    await transport.requestOnce("GET", "/api/v1/me");

    expect(refreshAccessToken).toHaveBeenCalledOnce();
    expect(mockFetch).toHaveBeenNthCalledWith(
      2,
      "https://api.example.com/api/v1/me",
      expect.objectContaining({
        headers: { Authorization: "Bearer new-access-token" },
      }),
    );
  });

  it("uses API error payloads and fallback messages consistently", async () => {
    const apiError = await getApiResponseError(
      {
        json: async () => ({ error: "Export unavailable" }),
      } as Response,
      "Export failed",
    );
    const fallbackError = await getApiResponseError(
      {} as Response,
      "Export failed",
    );

    expect(apiError.message).toBe("Export unavailable");
    expect(fallbackError.message).toBe("Export failed");
  });
});
