package tax

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// VATAggregateRow represents a VAT aggregate from journal entries
type VATAggregateRow struct {
	VATRate   decimal.Decimal
	IsOutput  bool
	TaxBase   decimal.Decimal
	TaxAmount decimal.Decimal
}

// Repository defines the contract for tax data access
type Repository interface {
	// QueryVATData queries VAT data from journal entries for a period
	QueryVATData(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time) ([]VATAggregateRow, error)

	// QueryKMDINFData queries invoice rows eligible for the KMD INF appendix.
	QueryKMDINFData(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time, threshold decimal.Decimal) ([]KMDINFReportRow, error)

	// QueryEUVATOSSData queries EU VAT OSS destination-country aggregates.
	QueryEUVATOSSData(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time, includeB2B bool) ([]EUVATOSSReportRow, error)

	// SaveDeclaration saves a KMD declaration (upsert)
	SaveDeclaration(ctx context.Context, schemaName string, decl *KMDDeclaration) error

	// GetDeclaration retrieves a KMD declaration for a given period
	GetDeclaration(ctx context.Context, schemaName, tenantID string, year, month int) (*KMDDeclaration, error)

	// ListDeclarations lists all KMD declarations for a tenant
	ListDeclarations(ctx context.Context, schemaName, tenantID string) ([]KMDDeclaration, error)
}
