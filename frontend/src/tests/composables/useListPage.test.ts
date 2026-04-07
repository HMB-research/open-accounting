import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useListPage } from "$lib/composables/useListPage.svelte";

describe("useListPage", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("loads items and dependencies with the current filter", async () => {
    const fetchItems = vi.fn().mockResolvedValue([{ id: "item-1" }]);
    const fetchDependencies = vi.fn().mockResolvedValue({
      contacts: [{ id: "contact-1" }],
    });

    const listPage = useListPage({
      fetchItems,
      fetchDependencies,
      initialFilter: { status: "draft" },
    });

    await listPage.loadData("tenant-1");

    expect(fetchItems).toHaveBeenCalledWith("tenant-1", { status: "draft" });
    expect(fetchDependencies).toHaveBeenCalledWith("tenant-1");
    expect(listPage.items).toEqual([{ id: "item-1" }]);
    expect(listPage.dependencies).toEqual({
      contacts: [{ id: "contact-1" }],
    });
    expect(listPage.isLoading).toBe(false);
    expect(listPage.error).toBe("");
  });

  it("surfaces fetch errors and clears the loading state", async () => {
    const fetchItems = vi.fn().mockRejectedValue(new Error("boom"));

    const listPage = useListPage({
      fetchItems,
      initialFilter: { status: "draft" },
    });

    await listPage.loadData("tenant-1");

    expect(listPage.items).toEqual([]);
    expect(listPage.error).toBe("boom");
    expect(listPage.isLoading).toBe(false);
  });

  it("uses the fallback error message for non-Error rejections", async () => {
    const fetchItems = vi.fn().mockRejectedValue("bad");

    const listPage = useListPage({
      fetchItems,
      initialFilter: { status: "draft" },
    });

    await listPage.loadData("tenant-1");

    expect(listPage.error).toBe("Failed to load data");
  });

  it("updates filter state and reloads through handleFilter", async () => {
    const fetchItems = vi.fn().mockResolvedValue([{ id: "item-2" }]);

    const listPage = useListPage({
      fetchItems,
      initialFilter: { status: "draft", year: 2024 },
    });

    listPage.setFilter({ status: "approved" });
    await listPage.handleFilter("tenant-2");

    expect(fetchItems).toHaveBeenCalledWith("tenant-2", {
      status: "approved",
      year: 2024,
    });
    expect(listPage.items).toEqual([{ id: "item-2" }]);
    expect(listPage.filter).toEqual({ status: "approved", year: 2024 });
  });

  it("manages optimistic state, explicit clearing, and auto-clearing success messages", async () => {
    const listPage = useListPage({
      fetchItems: vi.fn().mockResolvedValue([]),
      initialFilter: {},
      successTimeout: 50,
    });

    listPage.setItems([{ id: "item-9" }]);
    listPage.setActionLoading(true);
    listPage.setError("broken");
    listPage.clearError();
    listPage.setSuccess("saved");

    expect(listPage.items).toEqual([{ id: "item-9" }]);
    expect(listPage.actionLoading).toBe(true);
    expect(listPage.error).toBe("");
    expect(listPage.success).toBe("saved");

    await vi.advanceTimersByTimeAsync(50);
    expect(listPage.success).toBe("");

    listPage.setSuccess("keep", false);
    listPage.clearSuccess();
    listPage.setActionLoading(false);

    expect(listPage.success).toBe("");
    expect(listPage.actionLoading).toBe(false);
  });
});
