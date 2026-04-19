# Charge Flows

Payfake simulates the full multi-step charge flow for every payment channel
exactly as Paystack does. The checkout page reads the `status` field from
every response and renders the appropriate next step.

---

## Flow Status Values

| Status | Meaning | Next Action |
|--------|---------|-------------|
| `send_pin` | Enter card PIN | `POST /charge/submit_pin` |
| `send_otp` | Enter OTP | `POST /charge/submit_otp` |
| `send_birthday` | Enter date of birth | `POST /charge/submit_birthday` |
| `send_address` | Enter billing address | `POST /charge/submit_address` |
| `open_url` | Complete 3DS verification | Open `three_ds_url` in checkout |
| `pay_offline` | Approve USSD prompt | Poll transaction status |
| `success` | Payment complete | Show success, redirect to callback |
| `failed` | Payment declined | Show error with `error_code` |

---

## Card — Local (Verve)

Verve card number prefixes: `5061`, `5062`, `5063`, `6500`, `6501`

```
Step 1: POST /charge/card
Request:
{
  "access_code": "ACC_xxx",
  "card_number": "5061000000000000",
  "card_expiry": "12/26",
  "cvv": "123",
  "email": "customer@example.com"
}
Response: { "status": "send_pin", "display_text": "Please enter your card PIN" }

Step 2: POST /charge/submit_pin
Request: { "reference": "TXN_xxx", "pin": "1234" }
Response: { "status": "send_otp", "display_text": "Enter the OTP sent to your phone" }
Note: OTP generated here — read from /control/logs

Step 3: POST /charge/submit_otp
Request: { "reference": "TXN_xxx", "otp": "482931" }
Response: { "status": "success" } or { "status": "failed", "error_code": "..." }
Webhook: charge.success or charge.failed fires immediately
```

If the customer doesn't receive the OTP:
```
POST /charge/resend_otp
Request: { "reference": "TXN_xxx" }
Response: { "status": "send_otp", "display_text": "A new OTP has been sent" }
Note: New OTP generated — old OTP invalidated — read new OTP from /control/logs
```

---

## Card — International (Visa / Mastercard)

All Visa (`4xxx`) and Mastercard (`5xxx`) cards that are not Verve ranges.

```
Step 1: POST /charge/card
Request:
{
  "access_code": "ACC_xxx",
  "card_number": "4111111111111111",
  "card_expiry": "12/26",
  "cvv": "123",
  "email": "customer@example.com"
}
Response:
{
  "status": "open_url",
  "three_ds_url": "http://localhost:3000/simulate/3ds/TXN_xxx",
  "display_text": "Complete 3D Secure verification to proceed"
}

Step 2: Checkout app navigates to three_ds_url
The React checkout app's /simulate/3ds route shows a fake bank
authentication page. Customer clicks "Authenticate".

Step 3: Checkout app calls POST /api/v1/public/simulate/3ds/TXN_xxx
Response: { "status": "success" } or { "status": "failed" }
Webhook: charge.success or charge.failed fires
```

---

## Mobile Money

Supported providers: `mtn`, `vodafone`, `airteltigo`

```
Step 1: POST /charge/mobile_money
Request:
{
  "access_code": "ACC_xxx",
  "phone": "+233241234567",
  "provider": "mtn",
  "email": "customer@example.com"
}
Response:
{
  "status": "send_otp",
  "display_text": "Enter the OTP sent to +233241***567"
}
Note: OTP generated — read from /control/logs

Step 2: POST /charge/submit_otp
Request: { "reference": "TXN_xxx", "otp": "291847" }
Response:
{
  "status": "pay_offline",
  "display_text": "Approve the payment prompt on +233241***567"
}

Step 3: Customer approves USSD prompt on their phone (simulated via delay)
Checkout polls: GET /api/v1/public/transaction/:access_code every 3 seconds
Webhook: charge.success or charge.failed fires after delay_ms
```

The delay is controlled by `delay_ms` in the scenario config. Default is 0 — resolves instantly. Set to 5000 for a realistic 5 second wait.

---

## Bank Transfer

```
Step 1: POST /charge/bank
Request:
{
  "access_code": "ACC_xxx",
  "bank_code": "GCB",
  "account_number": "1234567890",
  "email": "customer@example.com"
}
Response:
{
  "status": "send_birthday",
  "display_text": "Enter your date of birth to verify your identity"
}

Step 2: POST /charge/submit_birthday
Request: { "reference": "TXN_xxx", "birthday": "1990-01-15" }
Response: { "status": "send_otp", "display_text": "Enter the OTP sent to your phone" }
Note: OTP generated — read from /control/logs

Step 3: POST /charge/submit_otp
Request: { "reference": "TXN_xxx", "otp": "738291" }
Response: { "status": "success" } or { "status": "failed" }
Webhook: charge.success or charge.failed fires
```

---

## Reading OTPs During Testing

OTPs are generated server-side and stored in the OTP log table.
They are never returned in any API response.

Read the OTP for a specific transaction:

```bash
curl "http://localhost:8080/api/v1/control/otp-logs?reference=TXN_xxx" \
  -H "Authorization: Bearer <jwt>"
```

Response:
```json
{
  "data": {
    "otp_logs": [
      {
        "id": "LOG_xxx",
        "reference": "TXN_xxx",
        "channel": "card",
        "otp_code": "482931",
        "step": "submit_pin",
        "used": false,
        "expires_at": "2026-04-12T00:10:00Z",
        "created_at": "2026-04-12T00:00:00Z"
      }
    ]
  }
}
```

The most recent unused OTP is the one to submit. OTPs expire after 10 minutes.
If the OTP has expired, call `resend_otp` to generate a fresh one.
---

## Forcing Flow Outcomes

Use the scenario engine to control what happens at each step:

```bash
# Force all charges to fail at the OTP step
curl -X PUT http://localhost:8080/api/v1/control/scenario \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "force_status": "failed",
    "error_code": "CHARGE_INVALID_OTP"
  }'

# Force card PIN failure specifically
curl -X PUT http://localhost:8080/api/v1/control/scenario \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "force_status": "failed",
    "error_code": "CHARGE_INVALID_PIN"
  }'

# Force MoMo timeout
curl -X PUT http://localhost:8080/api/v1/control/scenario \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "force_status": "failed",
    "error_code": "CHARGE_MOMO_TIMEOUT"
  }'
```

---

## Public Endpoints (Checkout Page)

All charge flow endpoints have public equivalents under `/api/v1/public/`
that the React checkout page uses. These authenticate via `access_code`
in the request body — no secret key needed in the browser.

| Endpoint | Public Equivalent |
|----------|-------------------|
| `POST /charge/card` | `POST /public/charge/card` |
| `POST /charge/mobile_money` | `POST /public/charge/mobile_money` |
| `POST /charge/bank` | `POST /public/charge/bank` |
| `POST /charge/submit_pin` | `POST /public/charge/submit_pin` |
| `POST /charge/submit_otp` | `POST /public/charge/submit_otp` |
| `POST /charge/submit_birthday` | `POST /public/charge/submit_birthday` |
| `POST /charge/submit_address` | `POST /public/charge/submit_address` |
| `POST /charge/resend_otp` | `POST /public/charge/resend_otp` |
| `POST /simulate/3ds/:reference` | `POST /public/simulate/3ds/:reference` |
