package payments

import (
	"strings"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSEPAExport(t *testing.T) {
	result, err := BuildSEPAExport(&SEPAExportRequest{
		MessageID:        "MSG-20260331",
		PaymentInfoID:    "PMTINF-20260331",
		CreationDateTime: "2026-03-31T09:30:00Z",
		DebtorName:       "Example OU",
		DebtorIBAN:       "EE382200221020145685",
		DebtorBIC:        "HABAEE2X",
		ExecutionDate:    "2026-04-01",
		Lines: []SEPACreditTransferLine{{
			EndToEndID:   "INV-1001",
			CreditorName: "Supplier AS",
			CreditorIBAN: "EE471000001020145685",
			CreditorBIC:  "EEUHEE2X",
			Amount:       decimal.RequireFromString("125.50"),
			Remittance:   "Invoice INV-1001",
		}, {
			CreditorName:  "Consultant OU",
			CreditorIBAN:  "EE871600161234567892",
			Amount:        decimal.RequireFromString("74.50"),
			PaymentNumber: "OUT-00042",
		}},
	})
	require.NoError(t, err)

	assert.Equal(t, "sepa-payments-2026-04-01.xml", result.FileName)
	assert.Equal(t, "MSG-20260331", result.MessageID)
	assert.Equal(t, 2, result.TransactionCount)
	assert.True(t, result.ControlSum.Equal(decimal.RequireFromString("200.00")))
	assert.Contains(t, result.XML, `<MsgId>MSG-20260331</MsgId>`)
	assert.Contains(t, result.XML, `<NbOfTxs>2</NbOfTxs>`)
	assert.Contains(t, result.XML, `<CtrlSum>200.00</CtrlSum>`)
	assert.Contains(t, result.XML, `<InstdAmt Ccy="EUR">125.50</InstdAmt>`)
	assert.Contains(t, result.XML, `<EndToEndId>OUT-00042</EndToEndId>`)
	assert.Contains(t, result.XML, `<Ustrd>Invoice INV-1001</Ustrd>`)
	assert.Contains(t, result.XML, `xmlns="urn:iso:std:iso:20022:tech:xsd:pain.001.001.03"`)
}

func TestBuildSEPAExportDefaultBranches(t *testing.T) {
	batchBooking := false

	result, err := BuildSEPAExport(&SEPAExportRequest{
		DebtorName:    "Example OU",
		DebtorIBAN:    "EE38 2200 2210 2014 5685",
		ExecutionDate: "2026-04-01",
		BatchBooking:  &batchBooking,
		Lines: []SEPACreditTransferLine{{
			CreditorName: "Supplier AS",
			CreditorIBAN: "EE471000001020145685",
			Amount:       decimal.RequireFromString("125.499"),
			InvoiceID:    "invoice-1",
		}, {
			CreditorName: "Fallback OU",
			CreditorIBAN: "EE871600161234567892",
			Amount:       decimal.RequireFromString("10"),
		}},
	})
	require.NoError(t, err)

	assert.NotEmpty(t, result.MessageID)
	assert.Equal(t, result.MessageID+"-PMT", result.PaymentInfoID)
	assert.True(t, result.ControlSum.Equal(decimal.RequireFromString("135.50")))
	assert.Contains(t, result.XML, "<BtchBookg>false</BtchBookg>")
	assert.Contains(t, result.XML, "<EndToEndId>invoice-1</EndToEndId>")
	assert.Contains(t, result.XML, "<EndToEndId>E2E-002</EndToEndId>")
	assert.Contains(t, result.XML, "<Id>NOTPROVIDED</Id>")
	assert.NotContains(t, result.XML, "<Ustrd>")
}

func TestBuildSEPAExportValidation(t *testing.T) {
	base := &SEPAExportRequest{
		MessageID:        "MSG-1",
		CreationDateTime: "2026-03-31T09:30:00Z",
		DebtorName:       "Example OU",
		DebtorIBAN:       "EE382200221020145685",
		ExecutionDate:    "2026-04-01",
		Lines: []SEPACreditTransferLine{{
			CreditorName: "Supplier AS",
			CreditorIBAN: "EE471000001020145685",
			Amount:       decimal.RequireFromString("125.50"),
		}},
	}

	tests := []struct {
		name    string
		mutate  func(*SEPAExportRequest)
		wantErr string
	}{
		{
			name: "invalid debtor IBAN",
			mutate: func(req *SEPAExportRequest) {
				req.DebtorIBAN = "EE001"
			},
			wantErr: "debtor_iban",
		},
		{
			name: "no lines",
			mutate: func(req *SEPAExportRequest) {
				req.Lines = nil
			},
			wantErr: "at least one",
		},
		{
			name: "non EUR line",
			mutate: func(req *SEPAExportRequest) {
				req.Lines[0].Currency = "USD"
			},
			wantErr: "currency must be EUR",
		},
		{
			name: "bad BIC",
			mutate: func(req *SEPAExportRequest) {
				req.DebtorBIC = "BAD"
			},
			wantErr: "debtor_bic",
		},
		{
			name: "missing debtor name",
			mutate: func(req *SEPAExportRequest) {
				req.DebtorName = " "
			},
			wantErr: "debtor_name is required",
		},
		{
			name: "missing execution date",
			mutate: func(req *SEPAExportRequest) {
				req.ExecutionDate = " "
			},
			wantErr: "execution_date is required",
		},
		{
			name: "bad execution date",
			mutate: func(req *SEPAExportRequest) {
				req.ExecutionDate = "2026/04/01"
			},
			wantErr: "execution_date must use YYYY-MM-DD",
		},
		{
			name: "bad creation date",
			mutate: func(req *SEPAExportRequest) {
				req.CreationDateTime = "2026-04-01"
			},
			wantErr: "creation_date_time must use RFC3339",
		},
		{
			name: "bad charge bearer",
			mutate: func(req *SEPAExportRequest) {
				req.ChargeBearer = "SHAR"
			},
			wantErr: "charge_bearer must be SLEV",
		},
		{
			name: "missing creditor name",
			mutate: func(req *SEPAExportRequest) {
				req.Lines[0].CreditorName = " "
			},
			wantErr: "creditor_name is required",
		},
		{
			name: "bad creditor IBAN",
			mutate: func(req *SEPAExportRequest) {
				req.Lines[0].CreditorIBAN = "EE001"
			},
			wantErr: "creditor_iban",
		},
		{
			name: "zero amount",
			mutate: func(req *SEPAExportRequest) {
				req.Lines[0].Amount = decimal.Zero
			},
			wantErr: "amount must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := *base
			req.Lines = append([]SEPACreditTransferLine(nil), base.Lines...)
			tt.mutate(&req)

			_, err := BuildSEPAExport(&req)
			require.Error(t, err)
			assert.True(t, strings.Contains(err.Error(), tt.wantErr), "error %q should contain %q", err.Error(), tt.wantErr)
		})
	}
}

func TestSEPAHelpers(t *testing.T) {
	_, err := BuildSEPAExport(nil)
	require.EqualError(t, err, "request is required")

	_, err = normalizeIBAN("EE38 2200 2210 2014 568!")
	require.EqualError(t, err, "IBAN must contain only letters and digits")

	assert.False(t, ibanChecksumValid("EE38-200221020145685"))
	assert.Equal(t, " payment-id ", firstNonEmpty(" ", " payment-id "))
	assert.Empty(t, firstNonEmpty(" ", "\t"))
	assert.Equal(t, "HABAEE2X", sepaAgentForBIC("HABAEE2X").FinancialInstitution.BIC)
	assert.NotNil(t, sepaAgentForBIC("").FinancialInstitution.Other)
}
