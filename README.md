
# Payfake

A self-hostable, Paystack-compatible payment simulator built for African developers. Test every payment scenario — card charges, mobile money flows, bank transfers, webhook delivery, failure simulation — without touching real money or depending on an external sandbox.

Point your app at `http://localhost:8080` instead of `https://api.paystack.co`. Change one line in your `.env`. Everything else stays the same.

---

## Why Payfake

Paystack's test mode works but it has real limitations:

- You can't force a specific failure scenario deterministically
- You can't simulate MoMo timeouts, network errors, or flaky webhooks
- You can't reproduce a race condition between webhook delivery and your DB write
- You're dependent on Paystack's infrastructure even in development
- You can't work offline

Payfake fills that gap. It's a glass box — you see every request, every response, every webhook attempt. You control every outcome.

---

## Features

- **Paystack-compatible API** — same URL structure, same payload keys, same response shapes. Swap one base URL and nothing else changes in your code.
- **Card, Mobile Money and Bank Transfer simulation** — including Ghana-specific channels (MTN MoMo, Vodafone Cash, AirtelTigo)
- **Webhook delivery** — fires `charge.success`, `charge.failed`, `refund.processed` events with HMAC-SHA512 signatures identical to Paystack's
- **Scenario engine** — configure failure rates, artificial delays, and forced outcomes per merchant
- **Force endpoint** — deterministically force any pending transaction to any terminal state
- **Introspection logs** — every request and response stored and queryable from the dashboard
- **React checkout page** — hosted payment popup served separately, talks to the public charge endpoints
- **Docker support** — single command to run the full stack

---

## Quick Start

### With Docker (recommended)

```bash
git clone https://github.com/payfake/payfake-api
cd payfake-api
docker-compose up --build
```

Server is live at `http://localhost:8080`.

### Without Docker

**Prerequisites:** Go 1.21+, PostgreSQL 14+

```bash
git clone https://github.com/payfake/payfake-api
cd payfake-api
cp .env.example .env
# edit .env with your database credentials
go run cmd/api/main.go
```

---

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `APP_NAME` | No | `payfake` | Application name |
| `APP_ENV` | No | `development` | Environment (`development` or `production`) |
| `APP_PORT` | No | `8080` | HTTP server port |
| `FRONTEND_URL` | No | `http://localhost:3000` | Dashboard frontend URL for CORS |
| `DB_HOST` | Yes | `localhost` | PostgreSQL host |
| `DB_PORT` | No | `5432` | PostgreSQL port |
| `DB_USER` | Yes | — | PostgreSQL user |
| `DB_PASSWORD` | Yes | — | PostgreSQL password |
| `DB_NAME` | Yes | — | PostgreSQL database name |
| `DB_SSLMODE` | No | `disable` | PostgreSQL SSL mode |
| `JWT_SECRET` | Yes | — | Secret for signing JWT tokens |
| `JWT_EXPIRY_HOURS` | No | `24` | JWT token expiry in hours |

---

## Authentication

Payfake uses two authentication schemes — identical to Paystack:

**Secret Key** — for all server-side API calls (transaction, charge, customer endpoints):
```
Authorization: Bearer sk_test_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

**JWT Token** — for dashboard operations (control, scenario, logs endpoints):
```
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Public endpoints** (`/api/v1/public/*`) — no Authorization header. Authenticated via `access_code` in the request body. Safe to call from the browser.

---

## API Reference

### Response Envelope

Every response from Payfake follows this shape:

**Success:**
```json
{
  "status": "success",
  "message": "Transaction initialized",
  "data": { },
  "metadata": {
    "timestamp": "2026-04-12T00:00:00Z",
    "request_id": "req_1775953469582_2962"
  },
  "code": "TRANSACTION_INITIALIZED"
}
```

**Error:**
```json
{
  "status": "error",
  "message": "Validation failed",
  "errors": [
    { "field": "amount", "message": "Amount must be greater than 0" }
  ],
  "metadata": {
    "timestamp": "2026-04-12T00:00:00Z",
    "request_id": "req_1775953469582_2962"
  },
  "code": "VALIDATION_ERROR"
}
```

---

### Auth — `/api/v1/auth`

#### Register
```
POST /api/v1/auth/register
```
```json
{
  "business_name": "Acme Store",
  "email": "dev@acme.com",
  "password": "secret123"
}
```
Returns merchant data and a JWT token. The merchant's `pk_test_` and `sk_test_` keys are generated automatically.

---

#### Login
```
POST /api/v1/auth/login
```
```json
{
  "email": "dev@acme.com",
  "password": "secret123"
}
```

---

#### Get Keys
```
GET /api/v1/auth/keys
Authorization: Bearer <jwt_token>
```

---

#### Regenerate Keys
```
POST /api/v1/auth/keys/regenerate
Authorization: Bearer <jwt_token>
```
The old secret key is immediately invalid after this call.

---

### Transaction — `/api/v1/transaction`

All transaction endpoints require `Authorization: Bearer sk_test_xxx`.

#### Initialize
```
POST /api/v1/transaction/initialize
```
```json
{
  "email": "customer@example.com",
  "amount": 10000,
  "currency": "GHS",
  "reference": "unique-ref-001",
  "callback_url": "https://yourapp.com/callback",
  "channels": ["card", "mobile_money", "bank_transfer"],
  "metadata": {}
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `email` | string | Yes | Customer email |
| `amount` | integer | Yes | Amount in smallest currency unit (pesewas for GHS) |
| `currency` | string | No | `GHS`, `NGN`, `KES`, `USD`. Defaults to `GHS` |
| `reference` | string | No | Unique reference. Auto-generated if not provided |
| `callback_url` | string | No | URL to redirect after payment |
| `channels` | array | No | Payment channels to offer |
| `metadata` | object | No | Custom key-value data |

**Response:**
```json
{
  "data": {
    "authorization_url": "http://localhost:5173?access_code=ACC_xxx",
    "access_code": "ACC_A1B2C3D4E5F6",
    "reference": "unique-ref-001"
  }
}
```

---

#### Verify
```
GET /api/v1/transaction/verify/:reference
```
Call this after the payment popup closes to confirm the outcome.

---

#### List
```
GET /api/v1/transaction?page=1&per_page=50&status=success
```

| Query param | Description |
|-------------|-------------|
| `page` | Page number. Defaults to 1 |
| `per_page` | Results per page. Max 100, defaults to 50 |
| `status` | Filter by status: `pending`, `success`, `failed`, `abandoned`, `reversed` |

---

#### Fetch
```
GET /api/v1/transaction/:id
```

---

#### Refund
```
POST /api/v1/transaction/:id/refund
```
Only transactions with `status: success` can be refunded.

---

### Charge — `/api/v1/charge`

All charge endpoints require `Authorization: Bearer sk_test_xxx`.

#### Charge Card
```
POST /api/v1/charge/card
```
```json
{
  "access_code": "ACC_A1B2C3D4E5F6",
  "card_number": "4111111111111111",
  "card_expiry": "12/26",
  "cvv": "123",
  "email": "customer@example.com"
}
```

Provide either `access_code` (popup flow) or `reference` (direct API flow).

---

#### Charge Mobile Money
```
POST /api/v1/charge/mobile_money
```
```json
{
  "access_code": "ACC_A1B2C3D4E5F6",
  "phone": "+233241234567",
  "provider": "mtn",
  "email": "customer@example.com"
}
```

| Provider | Value |
|----------|-------|
| MTN Mobile Money | `mtn` |
| Vodafone Cash | `vodafone` |
| AirtelTigo Money | `airteltigo` |

**Important:** MoMo always returns `status: pending` immediately. The final outcome (`charge.success` or `charge.failed`) arrives via webhook after the configured `delay_ms`. Always implement a webhook handler — never assume pending means success.

---

#### Charge Bank
```
POST /api/v1/charge/bank
```
```json
{
  "access_code": "ACC_A1B2C3D4E5F6",
  "bank_code": "GCB",
  "account_number": "1234567890",
  "email": "customer@example.com"
}
```

---

#### Fetch Charge
```
GET /api/v1/charge/:reference
```

---

### Customer — `/api/v1/customer`

All customer endpoints require `Authorization: Bearer sk_test_xxx`.

#### Create Customer
```
POST /api/v1/customer
```
```json
{
  "email": "customer@example.com",
  "first_name": "Kofi",
  "last_name": "Mensah",
  "phone": "+233241234567",
  "metadata": {}
}
```

---

#### List Customers
```
GET /api/v1/customer?page=1&per_page=50
```

---

#### Fetch Customer
```
GET /api/v1/customer/:code
```
Customer code format: `CUS_xxxxxxxxxxxx`

---

#### Update Customer
```
PUT /api/v1/customer/:code
```
All fields are optional — only provided fields are updated.

---

#### Customer Transactions
```
GET /api/v1/customer/:code/transactions?page=1&per_page=50
```

---

### Control — `/api/v1/control`

All control endpoints require `Authorization: Bearer <jwt_token>`.

These endpoints have no Paystack equivalent — they are Payfake's power layer.

---

#### Get Scenario Config
```
GET /api/v1/control/scenario
```

**Response:**
```json
{
  "data": {
    "failure_rate": 0.0,
    "delay_ms": 0,
    "force_status": "",
    "error_code": ""
  }
}
```

---

#### Update Scenario Config
```
PUT /api/v1/control/scenario
```
```json
{
  "failure_rate": 0.3,
  "delay_ms": 2000,
  "force_status": "failed",
  "error_code": "CHARGE_INSUFFICIENT_FUNDS"
}
```

All fields are optional — only provided fields are updated.

| Field | Type | Description |
|-------|------|-------------|
| `failure_rate` | float | Probability of charge failure. `0.0` = never, `1.0` = always |
| `delay_ms` | integer | Artificial delay applied to every charge. Max `30000` |
| `force_status` | string | Override all charges to this status: `success`, `failed`, `abandoned` |
| `error_code` | string | Error code returned when `force_status` is `failed` |

---

#### Reset Scenario Config
```
POST /api/v1/control/scenario/reset
```
Resets `failure_rate` to `0`, `delay_ms` to `0`, clears `force_status` and `error_code`. All subsequent charges will succeed instantly.

---

#### List Webhooks
```
GET /api/v1/control/webhooks?page=1&per_page=50
```

---

#### Retry Webhook
```
POST /api/v1/control/webhooks/:id/retry
```
Manually re-triggers delivery for a failed webhook event. Delivery happens asynchronously.

---

#### Webhook Attempts
```
GET /api/v1/control/webhooks/:id/attempts
```
Returns the full delivery attempt log for a webhook event — HTTP status codes, response bodies, timestamps.

---

#### Force Transaction Outcome
```
POST /api/v1/control/transactions/:reference/force
```
```json
{
  "status": "failed",
  "error_code": "CHARGE_INSUFFICIENT_FUNDS"
}
```

Forces a pending transaction to a specific terminal state. Bypasses the scenario engine entirely — the outcome is always exactly what you specify. Only pending transactions can be forced.

| Status | Description |
|--------|-------------|
| `success` | Mark as paid |
| `failed` | Mark as failed with optional error code |
| `abandoned` | Mark as abandoned by customer |

---

#### Get Logs
```
GET /api/v1/control/logs?page=1&per_page=50
```
Returns the full request/response introspection log — every API call made against this merchant's keys with request bodies, response bodies, status codes and durations.

---

#### Clear Logs
```
DELETE /api/v1/control/logs
```
Permanently deletes all log entries for the merchant.

---

### Public Checkout — `/api/v1/public`

No authentication required. Called from the React checkout page in the customer's browser. The merchant's secret key never touches the frontend.

#### Fetch Transaction by Access Code
```
GET /api/v1/public/transaction/:access_code
```

Returns transaction details safe for frontend display. Response varies by transaction status:

| Status | Message |
|--------|---------|
| `pending` | `Transaction fetched` — show payment form |
| `success` | `Payment already completed` — show success screen |
| `failed` | `Payment was not successful` — show retry screen |
| `abandoned` | `This payment link has expired` — show expired screen |
| `reversed` | `This payment has been refunded` — show refund screen |

**Response:**
```json
{
  "data": {
    "amount": 10000,
    "currency": "GHS",
    "status": "pending",
    "reference": "unique-ref-001",
    "callback_url": "https://yourapp.com/callback",
    "access_code": "ACC_A1B2C3D4E5F6",
    "merchant": {
      "business_name": "Acme Store",
      "public_key": "pk_test_xxx"
    },
    "customer": {
      "email": "customer@example.com",
      "first_name": "Kofi",
      "last_name": "Mensah"
    }
  }
}
```

---

#### Public Charge Card
```
POST /api/v1/public/charge/card
```
```json
{
  "access_code": "ACC_A1B2C3D4E5F6",
  "card_number": "4111111111111111",
  "card_expiry": "12/26",
  "cvv": "123",
  "email": "customer@example.com"
}
```

---

#### Public Charge Mobile Money
```
POST /api/v1/public/charge/mobile_money
```
```json
{
  "access_code": "ACC_A1B2C3D4E5F6",
  "phone": "+233241234567",
  "provider": "mtn",
  "email": "customer@example.com"
}
```

---

#### Public Charge Bank
```
POST /api/v1/public/charge/bank
```
```json
{
  "access_code": "ACC_A1B2C3D4E5F6",
  "bank_code": "GCB",
  "account_number": "1234567890",
  "email": "customer@example.com"
}
```

---

## Webhook Events

Payfake fires webhooks to your configured `webhook_url` after every terminal transaction state change.

### Events

| Event | Fired when |
|-------|-----------|
| `charge.success` | A charge completes successfully |
| `charge.failed` | A charge fails |
| `transfer.success` | A transfer completes |
| `transfer.failed` | A transfer fails |
| `refund.processed` | A refund is processed |

### Payload Shape

```json
{
  "event": "charge.success",
  "data": {
    "id": "TXN_A1B2C3D4E5F6",
    "reference": "unique-ref-001",
    "amount": 10000,
    "currency": "GHS",
    "status": "success",
    "channel": "card",
    "customer": {
      "email": "customer@example.com"
    },
    "paid_at": "2026-04-12T00:00:00Z"
  }
}
```

### Verifying Webhook Signatures

Payfake signs every webhook with your secret key using HMAC-SHA512 — identical to Paystack's signature scheme. The signature is sent in the `X-Paystack-Signature` header.

**Go:**
```go
import (
    "crypto/hmac"
    "crypto/sha512"
    "encoding/hex"
)

func verifyWebhook(body []byte, signature, secretKey string) bool {
    mac := hmac.New(sha512.New, []byte(secretKey))
    mac.Write(body)
    expected := hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(expected), []byte(signature))
}
```

**Python:**
```python
import hmac
import hashlib

def verify_webhook(body: bytes, signature: str, secret_key: str) -> bool:
    expected = hmac.new(
        secret_key.encode(),
        body,
        hashlib.sha512
    ).hexdigest()
    return hmac.compare_digest(expected, signature)
```

**JavaScript:**
```js
import { createHmac } from "crypto"

function verifyWebhook(body, signature, secretKey) {
    const expected = createHmac("sha512", secretKey)
        .update(body)
        .digest("hex")
    return expected === signature
}
```

---

## Error Codes

Every error response includes a `code` field for programmatic handling.

### Auth
| Code | Description |
|------|-------------|
| `AUTH_EMAIL_TAKEN` | Email already registered |
| `AUTH_INVALID_CREDENTIALS` | Wrong email or password |
| `AUTH_UNAUTHORIZED` | Missing or invalid credentials |
| `AUTH_TOKEN_EXPIRED` | JWT has expired |
| `AUTH_TOKEN_INVALID` | JWT is malformed or tampered |
| `AUTH_MERCHANT_NOT_FOUND` | Secret key doesn't match any merchant |

### Transaction
| Code | Description |
|------|-------------|
| `TRANSACTION_INITIALIZED` | Transaction created successfully |
| `TRANSACTION_NOT_FOUND` | No transaction matches the reference or ID |
| `TRANSACTION_REFERENCE_TAKEN` | Reference already used by another transaction |
| `TRANSACTION_INVALID_AMOUNT` | Amount is zero or negative |
| `TRANSACTION_INVALID_CURRENCY` | Currency not supported |
| `TRANSACTION_ALREADY_REFUNDED` | Refund already processed |

### Charge
| Code | Description |
|------|-------------|
| `CHARGE_FAILED` | Generic charge failure |
| `CHARGE_INVALID_CARD` | Card number failed validation |
| `CHARGE_CARD_EXPIRED` | Card expiry date is in the past |
| `CHARGE_INVALID_CVV` | CVV doesn't match |
| `CHARGE_INVALID_PIN` | PIN is incorrect |
| `CHARGE_INSUFFICIENT_FUNDS` | Account balance too low |
| `CHARGE_DO_NOT_HONOR` | Issuing bank declined — most common in Ghana |
| `CHARGE_NOT_PERMITTED` | Transaction type not allowed on this card |
| `CHARGE_LIMIT_EXCEEDED` | Daily or transaction limit exceeded |
| `CHARGE_NETWORK_ERROR` | Simulated network timeout |
| `CHARGE_MOMO_TIMEOUT` | Customer didn't respond to MoMo prompt |
| `CHARGE_MOMO_INVALID_NUMBER` | Phone not registered on provider network |
| `CHARGE_MOMO_LIMIT_EXCEEDED` | MoMo wallet limit exceeded |
| `CHARGE_MOMO_PROVIDER_UNAVAILABLE` | MoMo provider temporarily down |
| `CHARGE_BANK_INVALID_ACCOUNT` | Bank account doesn't exist |
| `CHARGE_BANK_TRANSFER_FAILED` | Bank transfer failed |

### Generic
| Code | Description |
|------|-------------|
| `VALIDATION_ERROR` | Request body failed validation |
| `INTERNAL_ERROR` | Unexpected server error |
| `RATE_LIMIT_EXCEEDED` | Too many requests |
| `NOT_FOUND` | Resource not found |
| `BAD_REQUEST` | Malformed request |

---

## Payment Flow

```
Your Backend                    Payfake                  React Checkout
────────────                    ───────                  ──────────────
POST /transaction/initialize ──► creates pending tx
                             ◄── access_code + auth_url
                                        │
                             customer opens auth_url
                                        │
                                        ▼
                             GET /public/transaction/:code ──► returns amount,
                                                               merchant, customer
                                        │
                             customer fills payment details
                                        │
                             POST /public/charge/card ──► SimulatorService
                                                          resolves outcome
                                        │
                             ◄── charge.success webhook fires to your backend
                                        │
Your Backend                            │
GET /transaction/verify/:ref ──► confirms final status
```

---

## SDKs

| Language | Repository |
|----------|-----------|
| Go | [payfake-go](https://github.com/payfake/payfake-go) |
| Python | [payfake-python](https://github.com/payfake/payfake-python) |
| JavaScript | [payfake-js](https://github.com/payfake/payfake-js) |
| Rust | [payfake-rust](https://github.com/payfake/payfake-rust) |

---

## Project Structure

```
payfake/
├── cmd/api/main.go             → entry point
├── internal/
│   ├── config/                 → env config
│   ├── database/               → GORM connection and migrations
│   ├── domain/                 → entity structs and constants
│   ├── repository/             → database access layer
│   ├── service/                → business logic
│   │   └── simulator_service.go → core simulation engine
│   ├── handler/                → HTTP handlers
│   ├── middleware/             → auth, CORS, logging, rate limit
│   ├── router/                 → route wiring
│   └── response/               → envelope and response codes
├── pkg/
│   ├── keygen/                 → pk/sk key generation
│   ├── crypto/                 → HMAC webhook signing
│   └── uid/                    → prefixed ID generation
├── Dockerfile
└── docker-compose.yml
```

---

## License

MIT
