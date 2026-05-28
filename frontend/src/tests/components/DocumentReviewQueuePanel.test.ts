import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/svelte";
import { baseLocale, setLocale } from "$lib/paraglide/runtime.js";
import type {
  DocumentAttachment,
  DocumentRetentionReview,
  DocumentReviewQueue,
} from "$lib/api";

const { apiMock } = vi.hoisted(() => ({
  apiMock: {
    getDocumentReviewQueue: vi.fn(),
    getDocumentRetentionReview: vi.fn(),
    markDocumentReviewed: vi.fn(),
    reviewDocument: vi.fn(),
    downloadDocument: vi.fn(),
  },
}));

vi.mock("$lib/api", async () => {
  const actual = await vi.importActual<typeof import("$lib/api")>("$lib/api");
  return {
    ...actual,
    api: apiMock,
  };
});

import DocumentReviewQueuePanel from "$lib/components/DocumentReviewQueuePanel.svelte";

function createDocument(
  overrides: Partial<DocumentAttachment> = {},
): DocumentAttachment {
  return {
    id: "doc-1",
    tenant_id: "tenant-1",
    entity_type: "year_end_close",
    entity_id: "2025-12-31",
    document_type: "close_pack",
    file_name: "close-pack.pdf",
    content_type: "application/pdf",
    file_size: 4096,
    notes: "Signed close checklist",
    retention_until: "2032-12-31T00:00:00Z",
    review_status: "PENDING",
    uploaded_by: "user-1",
    created_at: "2026-01-02T10:00:00Z",
    ...overrides,
  };
}

function createReviewQueue(
  documents: DocumentAttachment[] = [createDocument()],
  overrides: Partial<DocumentReviewQueue> = {},
): DocumentReviewQueue {
  return {
    review_status: "PENDING",
    limit: 50,
    total_count: documents.length,
    pending_review_count: documents.filter((doc) => doc.review_status === "PENDING").length,
    reviewed_count: documents.filter((doc) => doc.review_status === "REVIEWED").length,
    approved_count: documents.filter((doc) => doc.review_status === "APPROVED").length,
    rejected_count: documents.filter((doc) => doc.review_status === "REJECTED").length,
    documents,
    ...overrides,
  };
}

function createRetentionReview(
  documents: DocumentAttachment[] = [
    createDocument({
      id: "doc-retention",
      file_name: "receipt.pdf",
      document_type: "receipt",
      entity_type: "payment",
      entity_id: "pay-1",
    }),
  ],
  overrides: Partial<DocumentRetentionReview> = {},
): DocumentRetentionReview {
  return {
    as_of_date: "2027-03-01",
    cutoff_date: "2027-03-31",
    total_count: documents.length,
    expired_count: 0,
    due_soon_count: documents.length,
    missing_retention_count: 0,
    pending_review_count: documents.filter((doc) => doc.review_status === "PENDING").length,
    rejected_count: documents.filter((doc) => doc.review_status === "REJECTED").length,
    documents,
    ...overrides,
  };
}

describe("DocumentReviewQueuePanel", () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  beforeEach(() => {
    setLocale(baseLocale, { reload: false });
    vi.clearAllMocks();
    apiMock.getDocumentReviewQueue.mockResolvedValue(createReviewQueue());
    apiMock.getDocumentRetentionReview.mockResolvedValue(createRetentionReview());
    apiMock.markDocumentReviewed.mockResolvedValue(
      createDocument({ review_status: "REVIEWED" }),
    );
    apiMock.reviewDocument.mockResolvedValue(
      createDocument({
        review_status: "APPROVED",
        review_note: "Evidence accepted",
      }),
    );
    apiMock.downloadDocument.mockResolvedValue(undefined);
  });

  it("loads the tenant review queue and approves a document", async () => {
    render(DocumentReviewQueuePanel, {
      tenantId: "tenant-1",
      backHref: "/settings?tenant=tenant-1",
    });

    await waitFor(() => {
      expect(apiMock.getDocumentReviewQueue).toHaveBeenCalledWith("tenant-1", {
        entity_type: "",
        document_type: "",
        review_status: "PENDING",
        limit: 50,
      });
    });

    expect(screen.getByText("close-pack.pdf")).toBeInTheDocument();
    await fireEvent.input(
      screen.getByLabelText("Review note for close-pack.pdf"),
      { target: { value: "Evidence accepted" } },
    );
    await fireEvent.click(screen.getByRole("button", { name: "Approve" }));

    await waitFor(() => {
      expect(apiMock.reviewDocument).toHaveBeenCalledWith("tenant-1", "doc-1", {
        review_status: "APPROVED",
        review_note: "Evidence accepted",
      });
    });
    expect(
      await screen.findByText("close-pack.pdf review status updated."),
    ).toBeInTheDocument();
  });

  it("applies review filters and opens the retention queue", async () => {
    render(DocumentReviewQueuePanel, { tenantId: "tenant-1" });

    await screen.findByText("close-pack.pdf");

    await fireEvent.change(screen.getByLabelText("Review status"), {
      target: { value: "ALL" },
    });
    await fireEvent.change(screen.getByLabelText("Entity type"), {
      target: { value: "bank_transaction" },
    });
    await fireEvent.change(screen.getByLabelText("Document type"), {
      target: { value: "reconciliation_evidence" },
    });
    await fireEvent.click(screen.getByRole("button", { name: "Apply filters" }));

    await waitFor(() => {
      expect(apiMock.getDocumentReviewQueue).toHaveBeenLastCalledWith(
        "tenant-1",
        {
          entity_type: "bank_transaction",
          document_type: "reconciliation_evidence",
          review_status: "ALL",
          limit: 50,
        },
      );
    });

    await fireEvent.click(screen.getByRole("tab", { name: "Retention queue" }));

    await waitFor(() => {
      expect(apiMock.getDocumentRetentionReview).toHaveBeenCalledWith(
        "tenant-1",
        expect.objectContaining({
          horizon_days: 30,
          include_missing: undefined,
        }),
      );
    });
    expect(screen.getByText("receipt.pdf")).toBeInTheDocument();
    expect(screen.getByText("Due soon")).toBeInTheDocument();
  });

  it("requires rejection notes and downloads documents", async () => {
    render(DocumentReviewQueuePanel, { tenantId: "tenant-1" });

    await screen.findByText("close-pack.pdf");
    await fireEvent.click(screen.getByRole("button", { name: "Reject" }));
    expect(screen.getByText("Add a review note before rejecting a document.")).toBeInTheDocument();
    expect(apiMock.reviewDocument).not.toHaveBeenCalled();

    await fireEvent.input(
      screen.getByLabelText("Review note for close-pack.pdf"),
      { target: { value: "Missing signature" } },
    );
    await fireEvent.click(screen.getByRole("button", { name: "Reject" }));
    await waitFor(() => {
      expect(apiMock.reviewDocument).toHaveBeenCalledWith("tenant-1", "doc-1", {
        review_status: "REJECTED",
        review_note: "Missing signature",
      });
    });

    await fireEvent.click(screen.getByRole("button", { name: "Download" }));
    expect(apiMock.downloadDocument).toHaveBeenCalledWith(
      "tenant-1",
      "doc-1",
      "close-pack.pdf",
    );
  });
});
