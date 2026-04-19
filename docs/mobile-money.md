# Mobile Money Guide

Mobile money is the dominant payment method in West Africa.
Payfake simulates the full async MoMo flow including OTP verification,
the USSD prompt approval window, timeout scenarios and provider outages.

---

## Supported Providers

| Provider | Value | Country |
|----------|-------|---------|
| MTN Mobile Money | `mtn` | Ghana, Nigeria |
| Vodafone Cash | `vodafone` | Ghana |
| AirtelTigo Money | `airteltigo` | Ghana |

---

## Full Flow

```
Step 1: POST /charge/mobile_money
  { phone: "+233241234567", provider: "mtn" }
  ← { status: "send_otp", display_text: "Enter OTP sent to +233241***567" }

Step 2: Read OTP from /control/logs
  (In production this arrives via SMS)

Step 3: POST /charge/submit_otp
  { reference: "TXN_xxx", otp: "482931" }
  ← { status: "pay_offline", display_text: "Approve the prompt on your phone" }

Step 4: Checkout polls GET /api/v1/public/transaction/:access_code every 3s
  ← { status: "pending" }  (still waiting)
  ← { status: "success" }  (approved)
  ← { status: "failed" }   (declined or timed out)

Step 5: Webhook fires
  POST your_webhook_url { event: "charge.success" or "charge.failed" }
```

---

## Getting the OTP During Testing

```bash
curl "http://localhost:8080/api/v1/control/logs?per_page=10" \
  -H "Authorization: Bearer <jwt>" | \
  jq '.data.logs[] | select(.path | contains("mobile_money")) | .response_body'
```

The OTP is visible in the charge data inside the response body.

---

## Simulating Scenarios

### MoMo timeout (customer ignores prompt)
```bash
curl -X PUT http://localhost:8080/api/v1/control/scenario \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{"force_status": "failed", "error_code": "CHARGE_MOMO_TIMEOUT"}'
```

### Slow approval (realistic 5 second wait)
```bash
curl -X PUT http://localhost:8080/api/v1/control/scenario \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{"delay_ms": 5000}'
```

### Provider outage
```bash
curl -X PUT http://localhost:8080/api/v1/control/scenario \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{"force_status": "failed", "error_code": "CHARGE_MOMO_PROVIDER_UNAVAILABLE"}'
```

### Invalid number
```bash
curl -X PUT http://localhost:8080/api/v1/control/scenario \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{"force_status": "failed", "error_code": "CHARGE_MOMO_INVALID_NUMBER"}'
```

---

## Common Errors

| Error | When it happens | How to handle |
|-------|----------------|---------------|
| `CHARGE_MOMO_TIMEOUT` | Customer ignored or missed prompt | Offer resend or different payment |
| `CHARGE_MOMO_INVALID_NUMBER` | Number not on selected network | Ask customer to check provider |
| `CHARGE_MOMO_LIMIT_EXCEEDED` | Wallet limit reached | Offer alternative payment |
| `CHARGE_MOMO_PROVIDER_UNAVAILABLE` | Network down | Retry later or offer alternative |

---

## UX Recommendations

- Show "Check your phone" immediately after OTP submission
- Display a countdown — customers expect to wait up to 60 seconds
- If webhook doesn't arrive within 2 minutes call `GET /transaction/verify/:reference`
- Always offer a fallback payment method
- Never auto-retry silently — the customer may have deliberately declined
