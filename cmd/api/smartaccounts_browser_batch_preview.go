package main

import (
	"context"
	"errors"

	"github.com/HMB-research/open-accounting/internal/smartaccountsexecutor"
	"github.com/HMB-research/open-accounting/internal/smartaccountssync"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

// smartAccountsBrowserBatchPreviewAdapter deliberately projects the existing
// executor to digest-only workflow state. Preview is non-financial; applying
// it remains a separate tenant-scoped confirmation endpoint.
type smartAccountsBrowserBatchPreviewAdapter struct {
	executor *smartaccountsexecutor.Service
	tenants  *tenant.Service
}

func (a smartAccountsBrowserBatchPreviewAdapter) Preview(ctx context.Context, tenantID, packageID, actor string, useSourceChart bool) (smartaccountssync.BrowserBatchPreviewReceipt, error) {
	if a.executor == nil {
		return smartaccountssync.BrowserBatchPreviewReceipt{}, smartaccountssync.ErrBrowserBatchWorkflowUnavailable
	}
	schema := "tenant_" + tenantID
	if a.tenants != nil {
		if found, err := a.tenants.GetTenant(ctx, tenantID); err == nil && found != nil {
			schema = found.SchemaName
		}
	}
	preview, err := a.executor.Preview(ctx, schema, tenantID, packageID, actor, smartaccountsexecutor.PreviewRequest{UseSourceChart: useSourceChart})
	if preview == nil {
		return smartaccountssync.BrowserBatchPreviewReceipt{}, err
	}
	receipt := smartaccountssync.BrowserBatchPreviewReceipt{PreviewID: preview.ID, PreviewSHA256: preview.PreviewSHA256, Status: preview.Status}
	if errors.Is(err, smartaccountsexecutor.ErrPackageNotReady) || errors.Is(err, smartaccountsexecutor.ErrPreviewReviewRequired) {
		return receipt, smartaccountssync.ErrBrowserBatchPreviewReviewRequired
	}
	return receipt, err
}
