package payments

import (
	"context"

	"github.com/HMB-research/open-accounting/internal/invoicing"
	"gorm.io/gorm"
)

type paymentTransactionRunner interface {
	WithTransaction(ctx context.Context, fn func(Repository, InvoiceService) error) error
}

type gormPaymentTransactionRunner struct {
	db        *gorm.DB
	invoicing *invoicing.Service
}

func (r *gormPaymentTransactionRunner) WithTransaction(ctx context.Context, fn func(Repository, InvoiceService) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var invoiceService InvoiceService
		if r.invoicing != nil {
			invoiceService = r.invoicing.WithRepository(invoicing.NewGORMRepository(tx))
		}
		return fn(NewGORMRepository(tx), invoiceService)
	})
}
