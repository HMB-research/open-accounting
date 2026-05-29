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
