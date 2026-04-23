# Charge Flows

All charges go through `POST /charge` — a single endpoint where the channel
is determined by which sub-object you pass in the body. This matches the real
Paystack API exactly.

---

## Flow Status Values

| Status | Meaning | Next Call |
|--------|---------|-----------|
| `send_pin` | Enter card PIN | `POST /charge/submit_pin` |
| `send_otp` | Enter OTP | `POST /charge/submit_otp` |
| `send_birthday` | Enter date of birth | `POST /charge/submit_birthday` |
| `send_address` | Enter billing address | `POST /charge/submit_address` |
| `open_url` | Complete 3DS — open `url` | Navigate checkout to `data.url` |
| `pay_offline` | Approve USSD prompt | Poll `GET /api/v1/public/transaction/verify/:ref` |
| `success` | Payment complete | Fire webhook, update order |
| `failed` | Payment declined | Show `data.gateway_response` to customer |

---

## Card — Local (Verve)

BIN prefixes: `5061`, `5062`, `5063`, `6500`, `6501`

```bash
# Initiate
curl -X POST http://localhost:8080/charge \
  -H "Authorization: Bearer sk_test_xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "customer@example.com",
    "amount": 10000,
    "card": {
      "number": "5061000000000000",
      "cvv": "123",
      "expiry_month": "12",
      "expiry_year": "2026"
    }
  }'
# → { "status": true, "data": { "status": "send_pin", "reference": "TXN_xxx" } }

# Submit PIN
curl -X POST http://localhost:8080/charge/submit_pin \
  -H "Authorization: Bearer sk_test_xxx" \
  -H "Content-Type: application/json" \
  -d '{"reference":"TXN_xxx","pin":"1234"}'
# → { "status": true, "data": { "status": "send_otp" } }

# Get OTP (read from logs — no real phone needed)
curl "http://localhost:8080/api/v1/control/otp-logs?reference=TXN_xxx" \
  -H "Authorization: Bearer eyJ..."

# Submit OTP
curl -X POST http://localhost:8080/charge/submit_otp \
  -H "Authorization: Bearer sk_test_xxx" \
  -H "Content-Type: application/json" \
  -d '{"reference":"TXN_xxx","otp":"482931"}'
# → { "status": true, "data": { "status": "success" } }
```

Resend OTP if needed:

```bash
curl -X POST http://localhost:8080/charge/resend_otp \
  -H "Authorization: Bearer sk_test_xxx" \
  -H "Content-Type: application/json" \
  -d '{"reference":"TXN_xxx"}'
```

---

## Card — International (Visa/Mastercard)

```bash
curl -X POST http://localhost:8080/charge \
  -H "Authorization: Bearer sk_test_xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "customer@example.com",
    "amount": 10000,
    "card": {
      "number": "4111111111111111",
      "cvv": "123",
      "expiry_month": "12",
      "expiry_year": "2026"
    }
  }'
# → { "status": true, "data": {
#     "status": "open_url",
#     "url": "http://localhost:3000/simulate/3ds/TXN_xxx"
#   } }
```

The checkout app opens `data.url`. Customer confirms. Checkout calls:

```bash
curl -X POST http://localhost:8080/api/v1/public/simulate/3ds/TXN_xxx
# → { "status": true, "data": { "status": "success" } }
```

---

## Mobile Money

Providers: `mtn`, `vodafone`, `airteltigo`

```bash
# Initiate
curl -X POST http://localhost:8080/charge \
  -H "Authorization: Bearer sk_test_xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "customer@example.com",
    "amount": 10000,
    "mobile_money": {
      "phone": "+233241234567",
      "provider": "mtn"
    }
  }'
# → { "status": true, "data": { "status": "send_otp" } }

# Submit OTP
curl -X POST http://localhost:8080/charge/submit_otp \
  -H "Authorization: Bearer sk_test_xxx" \
  -H "Content-Type: application/json" \
  -d '{"reference":"TXN_xxx","otp":"291847"}'
# → { "status": true, "data": { "status": "pay_offline" } }

# Poll every 3 seconds
curl http://localhost:8080/api/v1/public/transaction/verify/TXN_xxx
# → { "status": true, "data": { "status": "success" } }
```

---

## Bank Transfer

```bash
# Initiate
curl -X POST http://localhost:8080/charge \
  -H "Authorization: Bearer sk_test_xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "customer@example.com",
    "amount": 10000,
    "bank": {
      "code": "GCB",
      "account_number": "1234567890"
    }
  }'
# → { "status": true, "data": { "status": "send_birthday" } }

# Submit DOB
curl -X POST http://localhost:8080/charge/submit_birthday \
  -H "Authorization: Bearer sk_test_xxx" \
  -H "Content-Type: application/json" \
  -d '{"reference":"TXN_xxx","birthday":"1990-01-15"}'
# → { "status": true, "data": { "status": "send_otp" } }

# Submit OTP (read from otp-logs)
curl -X POST http://localhost:8080/charge/submit_otp \
  -H "Authorization: Bearer sk_test_xxx" \
  -H "Content-Type: application/json" \
  -d '{"reference":"TXN_xxx","otp":"738291"}'
# → { "status": true, "data": { "status": "success" } }
```

---

## OTP Logs

OTPs are never returned in API responses. Read them during testing:

```bash
curl "http://localhost:8080/api/v1/control/otp-logs?reference=TXN_xxx" \
  -H "Authorization: Bearer eyJ..."
```

```json
{
  "status": true,
  "data": [
    {
      "reference": "TXN_xxx",
      "otp_code": "482931",
      "step": "submit_pin",
      "used": false,
      "expires_at": "2026-04-22T10:10:00Z"
    }
  ]
}
```

OTPs expire after 10 minutes. Call `/charge/resend_otp` to regenerate.
```

---

**`docs/api-reference.md`** — full rewrite:

```markdown
# API Reference

Base URL: `https://api.payfake.co` (hosted) or `http://localhost:8080` (local)

**Response envelope — identical to Paystack:**

```json
{ "status": true, "message": "...", "data": {} }
```

**Error envelope:**

```json
{
  "status": false,
  "message": "Validation error has occurred",
  "errors": {
    "email": [{ "rule": "required", "message": "Email is required" }]
  }
}
```

**Extra headers on every response:**
- `X-Payfake-Code` — machine-readable result code for the dashboard
- `X-Request-ID` — unique request ID for debugging

---

## Auth `/api/v1/auth`

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/register` | None | Create merchant account |
| POST | `/login` | None | Login |
| POST | `/refresh` | Refresh cookie | Rotate token pair |
| POST | `/logout` | Bearer JWT | Clear cookies |
| GET | `/me` | Bearer JWT | Current merchant |
| GET | `/keys` | Bearer JWT | Get pk/sk keys |
| POST | `/keys/regenerate` | Bearer JWT | Rotate keys |

---

## Transaction `/transaction`

Auth: `Bearer sk_test_xxx`

| Method | Path | Description |
|--------|------|-------------|
| POST | `/initialize` | Create transaction |
| GET | `/verify/:reference` | Verify by reference |
| GET | `/` | List (`perPage`, `page`, `status`) |
| GET | `/:id` | Fetch by ID |
| POST | `/:id/refund` | Refund |

**Initialize request:**
```json
{
  "email": "customer@example.com",
  "amount": 10000,
  "currency": "GHS",
  "reference": "order-001",
  "callback_url": "https://yourapp.com/callback"
}
```

**Initialize response data:**
```json
{
  "authorization_url": "http://localhost:3000?access_code=ACC_xxx",
  "access_code": "ACC_xxx",
  "reference": "TXN_xxx"
}
```

**Verify response data:**
```json
{
  "id": "TXN_xxx",
  "domain": "test",
  "status": "success",
  "reference": "order-001",
  "amount": 10000,
  "currency": "GHS",
  "gateway_response": "Approved",
  "paid_at": "2026-04-22T10:00:00Z",
  "created_at": "2026-04-22T09:59:00Z",
  "channel": "card",
  "fees": 150,
  "authorization": {
    "authorization_code": "AUTH_xxx",
    "bin": "506100",
    "last4": "0000",
    "exp_month": "12",
    "exp_year": "2026",
    "card_type": "verve",
    "bank": "TEST BANK",
    "brand": "verve",
    "reusable": false,
    "signature": "SIG_xxx",
    "country_code": "GH"
  },
  "customer": {
    "id": "CUS_xxx",
    "customer_code": "CUS_xxx",
    "email": "customer@example.com",
    "first_name": "Kofi",
    "last_name": "Mensah"
  }
}
```

**List meta:**
```json
{ "total": 142, "skipped": 0, "per_page": 50, "page": 1, "pageCount": 3 }
```

---

## Charge `/charge`

Auth: `Bearer sk_test_xxx`

| Method | Path | Description |
|--------|------|-------------|
| POST | `/` | Initiate charge (card/mobile_money/bank) |
| POST | `/submit_pin` | Submit card PIN |
| POST | `/submit_otp` | Submit OTP |
| POST | `/submit_birthday` | Submit date of birth |
| POST | `/submit_address` | Submit billing address |
| POST | `/resend_otp` | Resend OTP |
| GET | `/:reference` | Fetch charge state |

**Charge request — card:**
```json
{
  "email": "customer@example.com",
  "amount": 10000,
  "card": {
    "number": "5061000000000000",
    "cvv": "123",
    "expiry_month": "12",
    "expiry_year": "2026"
  }
}
```

**Charge request — mobile money:**
```json
{
  "email": "customer@example.com",
  "amount": 10000,
  "mobile_money": { "phone": "+233241234567", "provider": "mtn" }
}
```

**Charge request — bank:**
```json
{
  "email": "customer@example.com",
  "amount": 10000,
  "bank": { "code": "GCB", "account_number": "1234567890" },
  "birthday": "1990-01-15"
}
```

**Charge response data (all steps):**
```json
{
  "status": "send_pin",
  "reference": "TXN_xxx",
  "display_text": "Please enter your PIN",
  "amount": 10000,
  "currency": "GHS",
  "channel": "card"
}
```

For `open_url`:
```json
{
  "status": "open_url",
  "url": "http://localhost:3000/simulate/3ds/TXN_xxx",
  "display_text": "Please complete authentication on the provided url"
}
```

---

## Customer `/customer`

Auth: `Bearer sk_test_xxx`

| Method | Path | Description |
|--------|------|-------------|
| POST | `/` | Create customer |
| GET | `/` | List (`perPage`, `page`) |
| GET | `/:code` | Fetch |
| PUT | `/:code` | Update |
| GET | `/:code/transactions` | Customer transactions |

---

## Merchant `/api/v1/merchant`

Auth: Bearer JWT

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Get profile |
| PUT | `/` | Update profile |
| PUT | `/password` | Change password |
| GET | `/webhook` | Get webhook URL |
| POST | `/webhook` | Set webhook URL |
| POST | `/webhook/test` | Fire test webhook (5/min limit) |

---

## Control `/api/v1/control`

Auth: Bearer JWT

| Method | Path | Description |
|--------|------|-------------|
| GET | `/stats` | Overview + 7-day activity |
| GET | `/transactions` | Transaction list with search |
| GET | `/customers` | Customer list |
| GET | `/scenario` | Scenario config |
| PUT | `/scenario` | Update scenario |
| POST | `/scenario/reset` | Reset to defaults |
| GET | `/webhooks` | Webhook events |
| POST | `/webhooks/:id/retry` | Retry delivery |
| GET | `/webhooks/:id/attempts` | Delivery history |
| POST | `/transactions/:ref/force` | Force outcome |
| GET | `/logs` | Request/response logs |
| DELETE | `/logs` | Clear logs |
| GET | `/otp-logs` | OTP codes (`?reference=TXN_xxx`) |

**Scenario config:**
```json
{
  "failure_rate": 0.3,
  "delay_ms": 2000,
  "force_status": "failed",
  "error_code": "CHARGE_INSUFFICIENT_FUNDS"
}
```

---

## Public `/api/v1/public`

No auth, access_code authenticates.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/transaction/verify/:reference` | Poll status (MoMo) |
| GET | `/transaction/:access_code` | Load checkout |
| POST | `/charge` | Initiate (with `access_code` in body) |
| POST | `/charge/submit_pin` | Submit PIN |
| POST | `/charge/submit_otp` | Submit OTP |
| POST | `/charge/submit_birthday` | Submit DOB |
| POST | `/charge/submit_address` | Submit address |
| POST | `/charge/resend_otp` | Resend OTP |
| POST | `/simulate/3ds/:reference` | Complete 3DS |
```
