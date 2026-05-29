package payments

import (
	"encoding/xml"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/shopspring/decimal"
)

const (
	sepaPain001Namespace = "urn:iso:std:iso:20022:tech:xsd:pain.001.001.03"
	sepaDefaultCharge    = "SLEV"
)

var bicPattern = regexp.MustCompile(`^[A-Z0-9]{8}([A-Z0-9]{3})?$`)

// SEPACreditTransferLine is one outgoing SEPA credit transfer in a payment file.
type SEPACreditTransferLine struct {
	EndToEndID    string          `json:"end_to_end_id,omitempty"`
	CreditorName  string          `json:"creditor_name"`
	CreditorIBAN  string          `json:"creditor_iban"`
	CreditorBIC   string          `json:"creditor_bic,omitempty"`
	Amount        decimal.Decimal `json:"amount"`
	Currency      string          `json:"currency,omitempty"`
	Remittance    string          `json:"remittance,omitempty"`
	InvoiceID     string          `json:"invoice_id,omitempty"`
	PaymentID     string          `json:"payment_id,omitempty"`
	PaymentNumber string          `json:"payment_number,omitempty"`
}

// SEPAExportRequest describes a SEPA pain.001 XML export request.
type SEPAExportRequest struct {
	MessageID        string                   `json:"message_id,omitempty"`
	PaymentInfoID    string                   `json:"payment_info_id,omitempty"`
	CreationDateTime string                   `json:"creation_date_time,omitempty"`
	DebtorName       string                   `json:"debtor_name"`
	DebtorIBAN       string                   `json:"debtor_iban"`
	DebtorBIC        string                   `json:"debtor_bic,omitempty"`
	ExecutionDate    string                   `json:"execution_date"`
	BatchBooking     *bool                    `json:"batch_booking,omitempty"`
	ChargeBearer     string                   `json:"charge_bearer,omitempty"`
	Lines            []SEPACreditTransferLine `json:"lines"`
}

// SEPAExportResult summarizes the generated SEPA payment file.
type SEPAExportResult struct {
	FileName         string          `json:"file_name"`
	MessageID        string          `json:"message_id"`
	PaymentInfoID    string          `json:"payment_info_id"`
	ExecutionDate    string          `json:"execution_date"`
	TransactionCount int             `json:"transaction_count"`
	ControlSum       decimal.Decimal `json:"control_sum"`
	XML              string          `json:"xml"`
}

// BuildSEPAExport validates and renders an ISO 20022 pain.001.001.03 XML payment file.
func BuildSEPAExport(req *SEPAExportRequest) (*SEPAExportResult, error) {
	if req == nil {
		return nil, errors.New("request is required")
	}

	debtorName := strings.TrimSpace(req.DebtorName)
	if debtorName == "" {
		return nil, errors.New("debtor_name is required")
	}
	debtorIBAN, err := normalizeIBAN(req.DebtorIBAN)
	if err != nil {
		return nil, fmt.Errorf("debtor_iban: %w", err)
	}
	debtorBIC, err := normalizeOptionalBIC(req.DebtorBIC)
	if err != nil {
		return nil, fmt.Errorf("debtor_bic: %w", err)
	}
	if strings.TrimSpace(req.ExecutionDate) == "" {
		return nil, errors.New("execution_date is required")
	}
	executionDate, err := time.Parse("2006-01-02", strings.TrimSpace(req.ExecutionDate))
	if err != nil {
		return nil, fmt.Errorf("execution_date must use YYYY-MM-DD")
	}
	if len(req.Lines) == 0 {
		return nil, errors.New("at least one SEPA payment line is required")
	}

	createdAt := time.Now().UTC().Truncate(time.Second)
	if strings.TrimSpace(req.CreationDateTime) != "" {
		createdAt, err = time.Parse(time.RFC3339, strings.TrimSpace(req.CreationDateTime))
		if err != nil {
			return nil, fmt.Errorf("creation_date_time must use RFC3339")
		}
		createdAt = createdAt.UTC()
	}

	messageID := strings.TrimSpace(req.MessageID)
	if messageID == "" {
		messageID = fmt.Sprintf("SEPA-%s-%d", executionDate.Format("20060102"), createdAt.Unix())
	}
	paymentInfoID := strings.TrimSpace(req.PaymentInfoID)
	if paymentInfoID == "" {
		paymentInfoID = messageID + "-PMT"
	}
	chargeBearer := strings.ToUpper(strings.TrimSpace(req.ChargeBearer))
	if chargeBearer == "" {
		chargeBearer = sepaDefaultCharge
	}
	if chargeBearer != sepaDefaultCharge {
		return nil, errors.New("charge_bearer must be SLEV for SEPA credit transfers")
	}
	batchBooking := true
	if req.BatchBooking != nil {
		batchBooking = *req.BatchBooking
	}

	transactions := make([]sepaCreditTransferTransaction, 0, len(req.Lines))
	controlSum := decimal.Zero
	for i, line := range req.Lines {
		tx, amount, err := sepaTransactionFromLine(i, line)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, tx)
		controlSum = controlSum.Add(amount)
	}
	controlSum = controlSum.Round(2)

	doc := sepaDocument{
		XMLNS: sepaPain001Namespace,
		CustomerCreditTransfer: sepaCustomerCreditTransfer{
			GroupHeader: sepaGroupHeader{
				MessageID:        messageID,
				CreationDateTime: createdAt.Format(time.RFC3339),
				NumberOfTxs:      fmt.Sprintf("%d", len(transactions)),
				ControlSum:       controlSum.StringFixed(2),
				InitiatingParty:  sepaParty{Name: debtorName},
			},
			PaymentInfo: sepaPaymentInfo{
				PaymentInfoID:   paymentInfoID,
				PaymentMethod:   "TRF",
				BatchBooking:    batchBooking,
				NumberOfTxs:     fmt.Sprintf("%d", len(transactions)),
				ControlSum:      controlSum.StringFixed(2),
				PaymentTypeInfo: sepaPaymentTypeInfo{ServiceLevel: sepaCode{Code: "SEPA"}},
				ExecutionDate:   executionDate.Format("2006-01-02"),
				Debtor:          sepaParty{Name: debtorName},
				DebtorAccount:   sepaAccount{ID: sepaAccountID{IBAN: debtorIBAN}},
				DebtorAgent:     sepaAgentForBIC(debtorBIC),
				ChargeBearer:    chargeBearer,
				Transactions:    transactions,
			},
		},
	}

	payload, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal SEPA XML: %w", err)
	}
	xmlPayload := xml.Header + string(payload)

	return &SEPAExportResult{
		FileName:         fmt.Sprintf("sepa-payments-%s.xml", executionDate.Format("2006-01-02")),
		MessageID:        messageID,
		PaymentInfoID:    paymentInfoID,
		ExecutionDate:    executionDate.Format("2006-01-02"),
		TransactionCount: len(transactions),
		ControlSum:       controlSum,
		XML:              xmlPayload,
	}, nil
}

func sepaTransactionFromLine(index int, line SEPACreditTransferLine) (sepaCreditTransferTransaction, decimal.Decimal, error) {
	creditorName := strings.TrimSpace(line.CreditorName)
	if creditorName == "" {
		return sepaCreditTransferTransaction{}, decimal.Zero, fmt.Errorf("line %d creditor_name is required", index+1)
	}
	creditorIBAN, err := normalizeIBAN(line.CreditorIBAN)
	if err != nil {
		return sepaCreditTransferTransaction{}, decimal.Zero, fmt.Errorf("line %d creditor_iban: %w", index+1, err)
	}
	creditorBIC, err := normalizeOptionalBIC(line.CreditorBIC)
	if err != nil {
		return sepaCreditTransferTransaction{}, decimal.Zero, fmt.Errorf("line %d creditor_bic: %w", index+1, err)
	}
	if line.Amount.LessThanOrEqual(decimal.Zero) {
		return sepaCreditTransferTransaction{}, decimal.Zero, fmt.Errorf("line %d amount must be positive", index+1)
	}
	currency := strings.ToUpper(strings.TrimSpace(line.Currency))
	if currency == "" {
		currency = "EUR"
	}
	if currency != "EUR" {
		return sepaCreditTransferTransaction{}, decimal.Zero, fmt.Errorf("line %d currency must be EUR for SEPA credit transfers", index+1)
	}
	endToEndID := strings.TrimSpace(line.EndToEndID)
	if endToEndID == "" {
		endToEndID = strings.TrimSpace(firstNonEmpty(line.PaymentNumber, line.PaymentID, line.InvoiceID))
	}
	if endToEndID == "" {
		endToEndID = fmt.Sprintf("E2E-%03d", index+1)
	}

	tx := sepaCreditTransferTransaction{
		PaymentID: sepaPaymentID{EndToEndID: endToEndID},
		Amount: sepaAmount{
			InstructedAmount: sepaInstructedAmount{
				Currency: currency,
				Value:    line.Amount.Round(2).StringFixed(2),
			},
		},
		Creditor:        sepaParty{Name: creditorName},
		CreditorAccount: sepaAccount{ID: sepaAccountID{IBAN: creditorIBAN}},
	}
	if creditorBIC != "" {
		agent := sepaAgentForBIC(creditorBIC)
		tx.CreditorAgent = &agent
	}
	if remittance := strings.TrimSpace(line.Remittance); remittance != "" {
		tx.RemittanceInfo = &sepaRemittanceInfo{Unstructured: remittance}
	}
	return tx, line.Amount.Round(2), nil
}

func normalizeIBAN(value string) (string, error) {
	iban := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(value), " ", ""))
	if len(iban) < 15 || len(iban) > 34 {
		return "", errors.New("invalid IBAN length")
	}
	for _, r := range iban {
		if !unicode.IsDigit(r) && (r < 'A' || r > 'Z') {
			return "", errors.New("IBAN must contain only letters and digits")
		}
	}
	if !ibanChecksumValid(iban) {
		return "", errors.New("invalid IBAN checksum")
	}
	return iban, nil
}

func ibanChecksumValid(iban string) bool {
	rearranged := iban[4:] + iban[:4]
	remainder := 0
	for _, r := range rearranged {
		switch {
		case r >= '0' && r <= '9':
			remainder = (remainder*10 + int(r-'0')) % 97
		case r >= 'A' && r <= 'Z':
			value := int(r-'A') + 10
			remainder = (remainder*10 + value/10) % 97
			remainder = (remainder*10 + value%10) % 97
		default:
			return false
		}
	}
	return remainder == 1
}

func normalizeOptionalBIC(value string) (string, error) {
	bic := strings.ToUpper(strings.TrimSpace(value))
	if bic == "" {
		return "", nil
	}
	if !bicPattern.MatchString(bic) {
		return "", errors.New("BIC must be 8 or 11 alphanumeric characters")
	}
	return bic, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func sepaAgentForBIC(bic string) sepaAgent {
	if bic != "" {
		return sepaAgent{FinancialInstitution: sepaFinancialInstitution{BIC: bic}}
	}
	return sepaAgent{FinancialInstitution: sepaFinancialInstitution{Other: &sepaOtherID{ID: "NOTPROVIDED"}}}
}

type sepaDocument struct {
	XMLName                xml.Name                   `xml:"Document"`
	XMLNS                  string                     `xml:"xmlns,attr"`
	CustomerCreditTransfer sepaCustomerCreditTransfer `xml:"CstmrCdtTrfInitn"`
}

type sepaCustomerCreditTransfer struct {
	GroupHeader sepaGroupHeader `xml:"GrpHdr"`
	PaymentInfo sepaPaymentInfo `xml:"PmtInf"`
}

type sepaGroupHeader struct {
	MessageID        string    `xml:"MsgId"`
	CreationDateTime string    `xml:"CreDtTm"`
	NumberOfTxs      string    `xml:"NbOfTxs"`
	ControlSum       string    `xml:"CtrlSum"`
	InitiatingParty  sepaParty `xml:"InitgPty"`
}

type sepaPaymentInfo struct {
	PaymentInfoID   string                          `xml:"PmtInfId"`
	PaymentMethod   string                          `xml:"PmtMtd"`
	BatchBooking    bool                            `xml:"BtchBookg"`
	NumberOfTxs     string                          `xml:"NbOfTxs"`
	ControlSum      string                          `xml:"CtrlSum"`
	PaymentTypeInfo sepaPaymentTypeInfo             `xml:"PmtTpInf"`
	ExecutionDate   string                          `xml:"ReqdExctnDt"`
	Debtor          sepaParty                       `xml:"Dbtr"`
	DebtorAccount   sepaAccount                     `xml:"DbtrAcct"`
	DebtorAgent     sepaAgent                       `xml:"DbtrAgt"`
	ChargeBearer    string                          `xml:"ChrgBr"`
	Transactions    []sepaCreditTransferTransaction `xml:"CdtTrfTxInf"`
}

type sepaPaymentTypeInfo struct {
	ServiceLevel sepaCode `xml:"SvcLvl"`
}

type sepaCode struct {
	Code string `xml:"Cd"`
}

type sepaParty struct {
	Name string `xml:"Nm"`
}

type sepaAccount struct {
	ID sepaAccountID `xml:"Id"`
}

type sepaAccountID struct {
	IBAN string `xml:"IBAN"`
}

type sepaAgent struct {
	FinancialInstitution sepaFinancialInstitution `xml:"FinInstnId"`
}

type sepaFinancialInstitution struct {
	BIC   string       `xml:"BIC,omitempty"`
	Other *sepaOtherID `xml:"Othr,omitempty"`
}

type sepaOtherID struct {
	ID string `xml:"Id"`
}

type sepaCreditTransferTransaction struct {
	PaymentID       sepaPaymentID       `xml:"PmtId"`
	Amount          sepaAmount          `xml:"Amt"`
	CreditorAgent   *sepaAgent          `xml:"CdtrAgt,omitempty"`
	Creditor        sepaParty           `xml:"Cdtr"`
	CreditorAccount sepaAccount         `xml:"CdtrAcct"`
	RemittanceInfo  *sepaRemittanceInfo `xml:"RmtInf,omitempty"`
}

type sepaPaymentID struct {
	EndToEndID string `xml:"EndToEndId"`
}

type sepaAmount struct {
	InstructedAmount sepaInstructedAmount `xml:"InstdAmt"`
}

type sepaInstructedAmount struct {
	Currency string `xml:"Ccy,attr"`
	Value    string `xml:",chardata"`
}

type sepaRemittanceInfo struct {
	Unstructured string `xml:"Ustrd"`
}
