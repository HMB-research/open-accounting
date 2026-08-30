package importdelivery

import (
	"context"
	"errors"
	"strings"

	"github.com/HMB-research/open-accounting/internal/smartaccountssync"
)

var ErrSourceNotConfiguredForTenant = errors.New("bridge package source is not configured for this tenant")

// SmartAccountsControlStore is the narrow read-only proof that a bridge
// connection was explicitly configured for this tenant/source. It prevents a
// bridge package from creating a new binding merely by presenting a manifest.
type SmartAccountsControlStore interface {
	Get(ctx context.Context, tenantID, sourceCompanyID string) (*smartaccountssync.Control, error)
}

// ControlledSourceBinder first proves the bridge-derived source has an
// existing OA control binding, then enforces the global provider/source-to-one
// tenant registry used by import receipts. It contains no credential access.
type ControlledSourceBinder struct {
	controls SmartAccountsControlStore
	registry SourceBinder
}

func NewControlledSourceBinder(controls SmartAccountsControlStore, registry SourceBinder) *ControlledSourceBinder {
	return &ControlledSourceBinder{controls: controls, registry: registry}
}

func (b *ControlledSourceBinder) EnsureSourceCompanyBinding(ctx context.Context, tenantID, provider, sourceCompanyID string) error {
	if b == nil || b.controls == nil || b.registry == nil || provider != ProviderSmartAccounts {
		return ErrSourceNotConfiguredForTenant
	}
	control, err := b.controls.Get(ctx, strings.TrimSpace(tenantID), strings.TrimSpace(sourceCompanyID))
	if err != nil || control == nil || strings.TrimSpace(control.SecretReference) == "" || control.TenantID != strings.TrimSpace(tenantID) || control.SourceCompanyID != strings.TrimSpace(sourceCompanyID) {
		return ErrSourceNotConfiguredForTenant
	}
	return b.registry.EnsureSourceCompanyBinding(ctx, tenantID, provider, sourceCompanyID)
}
