# API Reference

Base URL: `http://localhost:8080`

All responses follow the envelope:
```json
{
  "status": "success|error",
  "message": "...",
  "data": {},
  "metadata": { "timestamp": "...", "request_id": "..." },
  "code": "..."
}
```

---

## Health

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/health` | None | Server liveness check |

---

## Auth `/api/v1/auth`

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/register` | None | Create merchant account, sets cookies |
| POST | `/login` | None | Login, sets cookies |
| POST | `/refresh` | `payfake_refresh` cookie | Rotate token pair |
| POST | `/logout` | Cookie or Bearer | Clear cookies |
| GET | `/me` | Cookie or Bearer | Get current merchant profile |
| GET | `/keys` | Cookie or Bearer | Get pk/sk keys |
| POST | `/keys/regenerate` | Cookie or Bearer | Rotate key pair |

---

## Merchant `/api/v1/merchant`

Auth: Cookie or Bearer (JWT)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/` | Get full merchant profile |
| PUT | `/` | Update business name and/or webhook URL |
| PUT | `/password` | Change password (requires current password) |
| GET | `/webhook` | Get current webhook URL and config |
| POST | `/webhook` | Set webhook URL |
| POST | `/webhook/test` | Fire test webhook to verify endpoint |

---

## Transaction `/transaction`

Auth: `Bearer sk_test_xxx`

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/initialize` | Create pending transaction |
| GET | `/verify/:reference` | Verify by reference |
| GET | `/` | List with pagination and status filter |
| GET | `/:id` | Fetch by ID |
| POST | `/:id/refund` | Refund a successful transaction |

**Initialize request:**
```json
{
  "email": "customer@example.com",
  "amount": 10000,
  "currency": "GHS",
  "reference": "your-ref-001",
  "callback_url": "https://yourapp.com/callback",
  "channels": ["card", "mobile_money", "bank_transfer"],
  "metadata": {}
}
```

**Initialize response:**
```json
{
  "data": {
    "authorization_url": "http://localhost:3000/ACC_xxx",
    "access_code": "ACC_xxxxxxxxxxxx",
    "reference": "TXN_xxxxxxxxxxxx"
  }
}
```

---

## Charge `/charge`

Auth: `Bearer sk_test_xxx`

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/` | Initiate card, MoMo, or bank charge |
| POST | `/submit_pin` | Submit card PIN → `send_otp` |
| POST | `/submit_otp` | Submit OTP → `pay_offline` or `success`/`failed` |
| POST | `/submit_birthday` | Submit DOB → `send_otp` |
| POST | `/submit_address` | Submit billing address → `success`/`failed` |
| POST | `/resend_otp` | Resend OTP → `send_otp` (fresh OTP) |
| GET | `/:reference` | Fetch current charge state |

**Charge response shape (all steps):**
```json
{
  "data": {
    "status": "send_pin",
    "reference": "TXN_xxxxxxxxxxxx",
    "display_text": "Please enter your card PIN",
    "charge": { "id": "CHG_xxx", "channel": "card", "flow_status": "send_pin" },
    "transaction": { "id": "TXN_xxx", "status": "pending", "amount": 10000 },
    "three_ds_url": ""
  }
}
```

**Flow status values:**

| Value | Meaning |
|-------|---------|
| `send_pin` | Show PIN input |
| `send_otp` | Show OTP input |
| `send_birthday` | Show DOB input |
| `send_address` | Show address form |
| `open_url` | Open `three_ds_url` |
| `pay_offline` | Show "approve on phone" |
| `success` | Show success screen |
| `failed` | Show failure screen |

---

## Customer `/customer`

Auth: `Bearer sk_test_xxx`

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/` | Create customer |
| GET | `/` | List with pagination |
| GET | `/:code` | Fetch by code |
| PUT | `/:code` | Partial update |
| GET | `/:code/transactions` | Customer transaction history |

---

## Control `/api/v1/control`

Auth: Cookie or Bearer (JWT)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/stats` | Overview numbers + 7-day chart data |
| GET | `/transactions` | Transaction list (JWT-based, for dashboard) |
| GET | `/customers` | Customer list (JWT-based, for dashboard) |
| GET | `/scenario` | Get scenario config |
| PUT | `/scenario` | Update scenario config |
| POST | `/scenario/reset` | Reset to defaults |
| GET | `/webhooks` | List webhook events |
| POST | `/webhooks/:id/retry` | Retry failed webhook |
| GET | `/webhooks/:id/attempts` | Delivery attempt log |
| POST | `/transactions/:ref/force` | Force transaction outcome |
| GET | `/logs` | Request/response introspection logs (sensitive fields redacted) |
| DELETE | `/logs` | Clear logs |
| GET | `/otp-logs` | OTP codes generated during charge flows |

**Stats response:**
```json
{
  "data": {
    "transactions": {
      "total": 142,
      "successful": 118,
      "failed": 14,
      "pending": 6,
      "abandoned": 4,
      "success_rate": 83.09
    },
    "volume": { "total_amount": 1420000 },
    "customers": { "total": 38 },
    "webhooks": { "total": 132, "delivered": 128, "failed": 4 },
    "daily_activity": [
      { "date": "2026-04-11", "count": 28, "volume": 280000 },
      { "date": "2026-04-12", "count": 29, "volume": 290000 }
    ]
  }
}
```

**Scenario update request:**
```json
{
  "failure_rate": 0.3,
  "delay_ms": 2000,
  "force_status": "failed",
  "error_code": "CHARGE_INSUFFICIENT_FUNDS"
}
```

**Force transaction request:**
```json
{
  "status": "failed",
  "error_code": "CHARGE_INSUFFICIENT_FUNDS"
}
```

---

## Public Checkout `/api/v1/public`

Auth: None. Initial checkout loads by `access_code`. Every mutating follow-up
request must include both `access_code` and `reference`.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/transaction/verify/:reference?access_code=ACC_xxx` | Poll transaction status (MoMo) |
| GET | `/transaction/:access_code` | Load transaction for checkout page |
| POST | `/charge` | Browser-safe charge initiation (`access_code` in body) |
| POST | `/charge/submit_pin` | Browser-safe PIN submission (`access_code`, `reference`, `pin`) |
| POST | `/charge/submit_otp` | Browser-safe OTP submission (`access_code`, `reference`, `otp`) |
| POST | `/charge/submit_birthday` | Browser-safe DOB submission (`access_code`, `reference`, `birthday`) |
| POST | `/charge/submit_address` | Browser-safe address submission (`access_code`, `reference`, address fields) |
| POST | `/charge/resend_otp` | Browser-safe OTP resend (`access_code`, `reference`) |
| POST | `/simulate/3ds/:reference` | Complete simulated 3DS flow (`access_code` + matching `reference` in body) |

**Public transaction response:**
```json
{
  "data": {
    "amount": 10000,
    "currency": "GHS",
    "status": "pending",
    "reference": "TXN_xxx",
    "callback_url": "https://yourapp.com/callback",
    "access_code": "ACC_xxx",
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

**Public follow-up request examples:**
```json
{
  "access_code": "ACC_xxx",
  "reference": "TXN_xxx",
  "otp": "482931"
}
```

Transaction status messages:

| Status | Message | Checkout Action |
|--------|---------|----------------|
| `pending` | Transaction fetched | Show payment form |
| `success` | Payment already completed | Show success screen |
| `failed` | Payment was not successful | Show retry screen |
| `abandoned` | This payment link has expired | Show expired screen |
| `reversed` | This payment has been refunded | Show refund screen |

---

## Complete Route Table

```
Method  Path                                               Auth
──────  ────                                               ────
GET     /health                                            None

POST    /api/v1/auth/register                              None
POST    /api/v1/auth/login                                 None
POST    /api/v1/auth/refresh                               Cookie (payfake_refresh)
POST    /api/v1/auth/logout                                Cookie or Bearer
GET     /api/v1/auth/me                                    Cookie or Bearer
GET     /api/v1/auth/keys                                  Cookie or Bearer
POST    /api/v1/auth/keys/regenerate                       Cookie or Bearer

GET     /api/v1/merchant                                   Cookie or Bearer
PUT     /api/v1/merchant                                   Cookie or Bearer
PUT     /api/v1/merchant/password                          Cookie or Bearer
GET     /api/v1/merchant/webhook                           Cookie or Bearer
POST    /api/v1/merchant/webhook                           Cookie or Bearer
POST    /api/v1/merchant/webhook/test                      Cookie or Bearer

POST    /transaction/initialize                            sk_test_xxx
GET     /transaction/verify/:reference                     sk_test_xxx
GET     /transaction                                       sk_test_xxx
GET     /transaction/:id                                   sk_test_xxx
POST    /transaction/:id/refund                            sk_test_xxx

POST    /charge                                            sk_test_xxx
POST    /charge/submit_pin                                 sk_test_xxx
POST    /charge/submit_otp                                 sk_test_xxx
POST    /charge/submit_birthday                            sk_test_xxx
POST    /charge/submit_address                             sk_test_xxx
POST    /charge/resend_otp                                 sk_test_xxx
GET     /charge/:reference                                 sk_test_xxx

POST    /customer                                          sk_test_xxx
GET     /customer                                          sk_test_xxx
GET     /customer/:code                                    sk_test_xxx
PUT     /customer/:code                                    sk_test_xxx
GET     /customer/:code/transactions                       sk_test_xxx

GET     /api/v1/control/stats                              Cookie or Bearer
GET     /api/v1/control/transactions                       Cookie or Bearer
GET     /api/v1/control/customers                          Cookie or Bearer
GET     /api/v1/control/scenario                           Cookie or Bearer
PUT     /api/v1/control/scenario                           Cookie or Bearer
POST    /api/v1/control/scenario/reset                     Cookie or Bearer
GET     /api/v1/control/webhooks                           Cookie or Bearer
POST    /api/v1/control/webhooks/:id/retry                 Cookie or Bearer
GET     /api/v1/control/webhooks/:id/attempts              Cookie or Bearer
POST    /api/v1/control/transactions/:ref/force            Cookie or Bearer
GET     /api/v1/control/logs                               Cookie or Bearer
DELETE  /api/v1/control/logs                               Cookie or Bearer
GET     /api/v1/control/otp-logs                           Cookie or Bearer

GET     /api/v1/public/transaction/:access_code            None
POST    /api/v1/public/charge                              None
POST    /api/v1/public/charge/submit_pin                   None
POST    /api/v1/public/charge/submit_otp                   None
POST    /api/v1/public/charge/submit_birthday              None
POST    /api/v1/public/charge/submit_address               None
POST    /api/v1/public/charge/resend_otp                   None
POST    /api/v1/public/simulate/3ds/:reference             None
GET     /api/v1/public/transaction/verify/:reference       None (requires access_code query)
```

**Total: 51 endpoints**
