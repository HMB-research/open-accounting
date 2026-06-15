import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/svelte";
import { baseLocale, setLocale } from "$lib/paraglide/runtime.js";
import type { DocumentAttachment } from "$lib/api";

const { apiMock } = vi.hoisted(() => ({
  apiMock: {
    listDocuments: vi.fn(),
    uploadDocument: vi.fn(),
    markDocumentReviewed: vi.fn(),
    reviewDocument: vi.fn(),
    downloadDocument: vi.fn(),
    deleteDocument: vi.fn(),
  },
}));

vi.mock("$lib/api", async () => {
  const actual = await vi.importActual<typeof import("$lib/api")>("$lib/api");
  return {
    ...actual,
    api: apiMock,
  };
});

import DocumentManager from "$lib/components/DocumentManager.svelte";

function createDocument(
  overrides: Partial<DocumentAttachment> = {},
): DocumentAttachment {
  return {
    id: "doc-1",
    tenant_id: "tenant-1",
    entity_type: "invoice",
    entity_id: "inv-1",
    document_type: "supporting_document",
    file_name: "invoice.pdf",
    content_type: "application/pdf",
    file_size: 2048,
    review_status: "PENDING",
    uploaded_by: "user-1",
    created_at: "2026-03-01T10:00:00Z",
    ...overrides,
  };
}

describe("DocumentManager", () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  beforeEach(() => {
    setLocale(baseLocale, { reload: false });
    vi.clearAllMocks();
    vi.stubGlobal(
      "confirm",
      vi.fn(() => true),
    );
    apiMock.listDocuments.mockResolvedValue([]);
    apiMock.uploadDocument.mockResolvedValue(createDocument());
    apiMock.markDocumentReviewed.mockResolvedValue(
      createDocument({
        review_status: "REVIEWED",
        reviewed_at: "2026-03-02T09:00:00Z",
        reviewed_by: "reviewer-1",
      }),
    );
    apiMock.reviewDocument.mockResolvedValue(
      createDocument({
        review_status: "APPROVED",
        review_note: "Evidence accepted",
        reviewed_at: "2026-03-02T09:00:00Z",
        reviewed_by: "reviewer-1",
      }),
    );
    apiMock.downloadDocument.mockResolvedValue(undefined);
    apiMock.deleteDocument.mockResolvedValue({ status: "deleted" });
  });

  it("loads, downloads, and deletes existing documents", async () => {
    apiMock.listDocuments.mockResolvedValueOnce([
      createDocument({ file_name: "invoice-1001.pdf" }),
    ]);

    render(DocumentManager, {
      open: true,
      tenantId: "tenant-1",
      entityType: "invoice",
      entityId: "inv-1",
      title: "Documents for invoice INV-1001",
    });

    await waitFor(() => {
      expect(apiMock.listDocuments).toHaveBeenCalledWith(
        "tenant-1",
        "invoice",
        "inv-1",
      );
    });

    expect(screen.getByText("invoice-1001.pdf")).toBeInTheDocument();

    await fireEvent.click(screen.getByRole("button", { name: "Download" }));
    expect(apiMock.downloadDocument).toHaveBeenCalledWith(
      "tenant-1",
      "doc-1",
      "invoice-1001.pdf",
    );

    await fireEvent.click(
      screen.getByRole("button", { name: "Mark reviewed" }),
    );
    expect(apiMock.markDocumentReviewed).toHaveBeenCalledWith(
      "tenant-1",
      "doc-1",
    );
    await waitFor(() => {
      expect(screen.getByText("Reviewed")).toBeInTheDocument();
    });

    await fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    expect(apiMock.deleteDocument).toHaveBeenCalledWith("tenant-1", "doc-1");
    await waitFor(() => {
      expect(screen.queryByText("invoice-1001.pdf")).not.toBeInTheDocument();
    });
  });

  it("uploads selected files and refreshes the list", async () => {
    apiMock.listDocuments.mockResolvedValueOnce([]).mockResolvedValueOnce([
      createDocument({
        id: "doc-2",
        entity_type: "payment",
        entity_id: "pay-1",
        file_name: "receipt.pdf",
      }),
    ]);

    const { container } = render(DocumentManager, {
      open: true,
      tenantId: "tenant-1",
      entityType: "payment",
      entityId: "pay-1",
      title: "Documents for payment PMT-001",
    });

    await waitFor(() => {
      expect(apiMock.listDocuments).toHaveBeenCalledWith(
        "tenant-1",
        "payment",
        "pay-1",
      );
    });

    const fileInput = container.querySelector(
      'input[type="file"]',
    ) as HTMLInputElement | null;
    expect(fileInput).not.toBeNull();

    const file = new File(["receipt"], "receipt.pdf", {
      type: "application/pdf",
    });
    Object.defineProperty(fileInput, "files", {
      configurable: true,
      value: [file],
    });
    await fireEvent.change(fileInput as HTMLInputElement);
    await fireEvent.input(screen.getByLabelText("Notes"), {
      target: { value: "Matched against uploaded receipt" },
    });
    await fireEvent.click(
      screen.getByRole("button", { name: "Upload selected files" }),
    );

    await waitFor(() => {
      expect(apiMock.uploadDocument).toHaveBeenCalledWith(
        "tenant-1",
        "payment",
        "pay-1",
        file,
        {
          document_type: "receipt",
          notes: "Matched against uploaded receipt",
          retention_until: undefined,
        },
      );
    });
    await waitFor(() => {
      expect(apiMock.listDocuments).toHaveBeenCalledTimes(2);
    });
    expect(screen.getByText("receipt.pdf")).toBeInTheDocument();
  });

  it("defaults year-end close uploads to close-pack evidence and approves reviews", async () => {
    const onchanged = vi.fn();
    apiMock.listDocuments.mockResolvedValueOnce([
      createDocument({
        entity_type: "year_end_close",
        entity_id: "close-entity-1",
        document_type: "close_pack",
        file_name: "close-pack.pdf",
      }),
    ]);

    const { container } = render(DocumentManager, {
      open: true,
      tenantId: "tenant-1",
      entityType: "year_end_close",
      entityId: "close-entity-1",
      title: "Close-pack evidence",
      onchanged,
    });

    await waitFor(() => {
      expect(apiMock.listDocuments).toHaveBeenCalledWith(
        "tenant-1",
        "year_end_close",
        "close-entity-1",
      );
    });

    expect(screen.getByText("close-pack.pdf")).toBeInTheDocument();
    const typeSelect = screen.getByLabelText("Document type") as HTMLSelectElement;
    expect(typeSelect.value).toBe("close_pack");

    await fireEvent.input(screen.getByLabelText("Review note for close-pack.pdf"), {
      target: { value: "Evidence accepted" },
    });
    await fireEvent.click(screen.getByRole("button", { name: "Approve" }));

    expect(apiMock.reviewDocument).toHaveBeenCalledWith(
      "tenant-1",
      "doc-1",
      {
        review_status: "APPROVED",
        review_note: "Evidence accepted",
      },
    );
    await waitFor(() => {
      expect(onchanged).toHaveBeenCalledTimes(1);
    });

    apiMock.listDocuments.mockResolvedValueOnce([]).mockResolvedValueOnce([
      createDocument({
        id: "doc-close-pack",
        entity_type: "year_end_close",
        entity_id: "close-entity-1",
        document_type: "close_pack",
        file_name: "close-pack-uploaded.pdf",
      }),
    ]);
    cleanup();
    const uploadRender = render(DocumentManager, {
      open: true,
      tenantId: "tenant-1",
      entityType: "year_end_close",
      entityId: "close-entity-1",
      title: "Close-pack evidence",
    });

    const fileInput = uploadRender.container.querySelector(
      'input[type="file"]',
    ) as HTMLInputElement | null;
    expect(fileInput).not.toBeNull();
    const file = new File(["close pack"], "close-pack.pdf", {
      type: "application/pdf",
    });
    Object.defineProperty(fileInput, "files", {
      configurable: true,
      value: [file],
    });
    await fireEvent.change(fileInput as HTMLInputElement);
    await fireEvent.click(
      screen.getByRole("button", { name: "Upload selected files" }),
    );

    await waitFor(() => {
      expect(apiMock.uploadDocument).toHaveBeenLastCalledWith(
        "tenant-1",
        "year_end_close",
        "close-entity-1",
        file,
        {
          document_type: "close_pack",
          notes: undefined,
          retention_until: undefined,
        },
      );
    });
  });
});
