package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/payfake/payfake-api/internal/domain"
	"github.com/payfake/payfake-api/internal/service"
)

// buildTransactionData constructs the transaction data object matching
// Paystack's verify/list response shape exactly.
// Every transaction endpoint, initialize, verify, list, fetch, refund —
// returns data through this function so the shape is consistent everywhere.
func buildTransactionData(tx *domain.Transaction, charge *domain.Charge) gin.H {
	return gin.H{
		"id":               tx.ID,
		"domain":           "test",
		"status":           string(tx.Status),
		"reference":        tx.Reference,
		"receipt_number":   nil,
		"amount":           tx.Amount,
		"message":          nil,
		"gateway_response": gatewayResponse(tx.Status),
		"paid_at":          tx.PaidAt,
		"created_at":       tx.CreatedAt,
		"channel":          string(tx.Channel),
		"currency":         string(tx.Currency),
		"ip_address":       nil,
		"fees":             tx.Fees,
		"fees_split":       nil,
		"fees_breakdown":   nil,
		"metadata":         tx.Metadata,
		"log":              nil,
		"customer":         buildCustomerSummary(&tx.Customer),
		"authorization":    buildAuthorization(tx, charge),
	}
}

// buildCustomerSummary returns the customer object embedded in transaction responses.
// Matches Paystack's customer shape inside transaction data.
func buildCustomerSummary(c *domain.Customer) gin.H {
	if c == nil {
		return nil
	}
	return gin.H{
		"id":            c.ID,
		"customer_code": c.Code,
		"email":         c.Email,
		"first_name":    c.FirstName,
		"last_name":     c.LastName,
		"phone":         c.Phone,
	}
}

// buildCustomerData returns the full customer object for customer endpoints.
// Matches Paystack's customer response shape including domain and timestamps.
func buildCustomerData(c *domain.Customer) gin.H {
	return gin.H{
		"id":            c.ID,
		"customer_code": c.Code,
		"email":         c.Email,
		"first_name":    c.FirstName,
		"last_name":     c.LastName,
		"phone":         c.Phone,
		"metadata":      c.Metadata,
		"domain":        "test",
		"createdAt":     c.CreatedAt,
		"updatedAt":     c.UpdatedAt,
		"integration":   0,
	}
}

// buildAuthorization builds the authorization object matching Paystack's shape.
// This is what developers store for recurring charges.
// Real Paystack: { authorization_code, bin, last4, exp_month, exp_year, brand, bank... }
func buildAuthorization(tx *domain.Transaction, charge *domain.Charge) gin.H {
	if charge == nil {
		return nil
	}

	auth := gin.H{
		"authorization_code": fmt.Sprintf("AUTH_%s", safeSlice(charge.ID, 4)),
		"channel":            string(tx.Channel),
		"reusable":           false,
		"country_code":       "GH",
		"account_name":       nil,
		"signature":          fmt.Sprintf("SIG_%s", safeSlice(charge.ID, 4)),
	}

	switch tx.Channel {
	case domain.ChannelCard:
		auth["bin"] = binFromLast4(charge.CardLast4)
		auth["last4"] = charge.CardLast4
		auth["exp_month"] = "12"
		auth["exp_year"] = "2026"
		auth["card_type"] = charge.CardBrand
		auth["bank"] = "TEST BANK"
		auth["brand"] = charge.CardBrand
	case domain.ChannelMobileMoney:
		auth["mobile_money_number"] = charge.MomoPhone
		auth["mobile_money_name"] = string(charge.MomoProvider)
	case domain.ChannelBankTransfer:
		auth["bank_code"] = charge.BankCode
		auth["account_number"] = "****" + safeSlice(charge.BankAccountNumber, len(charge.BankAccountNumber)-4)
	}

	return auth
}

// buildChargeFlowData builds the response for charge step endpoints.
// Matches Paystack's charge API response data shape.
func buildChargeFlowData(out *service.ChargeFlowResponse) gin.H {
	data := gin.H{
		"status":       out.Status,
		"reference":    out.Reference,
		"display_text": out.DisplayText,
	}

	// For open_url (3DS) Paystack returns the URL as "url" not "three_ds_url"
	if out.ThreeDSURL != "" {
		data["url"] = out.ThreeDSURL
	}

	if out.Transaction != nil {
		data["amount"] = out.Transaction.Amount
		data["currency"] = string(out.Transaction.Currency)
		data["transaction_date"] = out.Transaction.CreatedAt
		data["domain"] = "test"
		data["metadata"] = out.Transaction.Metadata
		data["gateway_response"] = gatewayResponse(domain.TransactionStatus(string(out.Status)))
		data["channel"] = string(out.Transaction.Channel)
		data["fees"] = out.Transaction.Fees
	}

	if out.Charge != nil && out.Transaction != nil {
		data["authorization"] = buildAuthorization(out.Transaction, out.Charge)
	}

	return data
}

// gatewayResponse returns the human-readable gateway response for a status.
// Matches Paystack's gateway_response field values.
func gatewayResponse(status domain.TransactionStatus) string {
	switch status {
	case domain.TransactionSuccess:
		return "Approved"
	case domain.TransactionFailed:
		return "Declined"
	case domain.TransactionPending:
		return "Transaction pending"
	case domain.TransactionAbandoned:
		return "Transaction abandoned"
	case domain.TransactionReversed:
		return "Reversed"
	default:
		return ""
	}
}

func binFromLast4(last4 string) string {
	if last4 == "1111" {
		return "411111"
	}
	return "506100"
}

func safeSlice(s string, from int) string {
	if from < 0 || from >= len(s) {
		return s
	}
	return s[from:]
}
