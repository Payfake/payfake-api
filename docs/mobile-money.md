# Mobile Money Guide

Mobile money is the dominant payment method across West Africa.
Payfake simulates the full async MoMo flow, including the approval
window, timeout scenarios and provider outages.

---

## Supported Providers

| Provider | Value | Country |
|----------|-------|---------|
| MTN Mobile Money | `mtn` | Ghana, Nigeria |
| Vodafone Cash | `vodafone` | Ghana |
| AirtelTigo Money | `airteltigo` | Ghana |

---

## How MoMo Works (Real World)

1. Your backend calls `POST /charge/mobile_money` with the customer's phone and provider
2. Payfake sends a USSD prompt to the customer's phone
3. Customer approves on their phone (or ignores it, timeout)
4. Provider confirms or rejects
5. Payfake fires `charge.success` or `charge.failed` webhook to your backend
6. Your backend updates the order

Steps 3-6 happen asynchronously, the charge endpoint returns immediately
with `status: pending` after step 2.

---

## Integration Pattern

```go
// 1. Initialize transaction
tx, _ := client.Transaction.Initialize(ctx, payfake.InitializeInput{
    Email:    "customer@example.com",
    Amount:   10000,
    Currency: "GHS",
})

// 2. Charge mobile money — returns pending immediately
charge, _ := client.Charge.MobileMoney(ctx, payfake.ChargeMomoInput{
    AccessCode: tx.AccessCode,
    Phone:      "+233241234567",
    Provider:   "mtn",
    Email:      "customer@example.com",
})

// charge.Transaction.Status is always "pending" here
// DO NOT mark the order as paid yet

// 3. Tell the customer to check their phone
// 4. Wait for the webhook

// In your webhook handler:
func handleWebhook(event WebhookEvent) {
    if event.Event == "charge.success" {
        // NOW mark the order as paid
        // reference is event.Data.Reference
    }
    if event.Event == "charge.failed" {
        // notify customer, offer retry
    }
}
```

---

## Simulating MoMo Scenarios

### Simulate MoMo timeout (customer ignores prompt)

```bash
curl -X PUT http://localhost:8080/api/v1/control/scenario \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "force_status": "failed",
    "error_code": "CHARGE_MOMO_TIMEOUT"
  }'
```

### Simulate slow approval (5 second delay)

```bash
curl -X PUT http://localhost:8080/api/v1/control/scenario \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{"delay_ms": 5000}'
```

Your app should handle up to 30 seconds of delay gracefully, show a
"waiting for approval" state and don't time out too early.

### Simulate provider outage

```bash
curl -X PUT http://localhost:8080/api/v1/control/scenario \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "force_status": "failed",
    "error_code": "CHARGE_MOMO_PROVIDER_UNAVAILABLE"
  }'
```

---

## Common MoMo Errors

| Error | When it happens | How to handle |
|-------|----------------|---------------|
| `CHARGE_MOMO_TIMEOUT` | Customer ignored or missed prompt | Offer to resend or use different payment |
| `CHARGE_MOMO_INVALID_NUMBER` | Number not on selected network | Ask customer to check their number and provider |
| `CHARGE_MOMO_LIMIT_EXCEEDED` | Wallet daily/transaction limit | Ask customer to use a different payment method |
| `CHARGE_MOMO_PROVIDER_UNAVAILABLE` | Network down | Retry later or offer alternative |

---

## UX Recommendations

Based on real production MoMo integration patterns in Ghana:

- Show a clear "Check your phone" message immediately after initiating
- Display a countdown or spinner, customers expect to wait up to 60 seconds
- If the webhook doesn't arrive within 2 minutes, call `GET /transaction/verify/:reference`
  to check status — the webhook may have failed delivery
- Always offer a fallback payment method, MoMo failures are common
- Never auto-retry silently, the customer may have intentionally declined
