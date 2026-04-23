# Payfake

A self-hostable Paystack-compatible payment simulator for African developers.
Test every payment scenario — card charges, mobile money, bank transfers,
webhook delivery, failure simulation — without touching real money.

**Zero code changes to switch environments.** Change one env var:

```bash
# Development — Payfake
PAYSTACK_BASE_URL=http://localhost:8080

# Production — Real Paystack
PAYSTACK_BASE_URL=https://api.paystack.co
```

Same response shapes. Same URL structure. Same error formats. Same webhooks.
Your existing Paystack integration works against Payfake unchanged.

---

## Why Payfake over Paystack Test Mode

Paystack test mode is good but has real limitations:

| | Paystack test mode | Payfake |
|---|---|---|
| Force specific failure | ❌ | ✅ |
| Simulate network delays | ❌ | ✅ |
| MoMo without real phone | ❌ | ✅ |
| Full request/response logs | ❌ | ✅ |
| Works offline | ❌ | ✅ |
| CI/CD deterministic testing | ❌ | ✅ |
| No account required | ❌ | ✅ |

---

## Quick Start

```bash
git clone https://github.com/payfake/payfake-api
cd payfake-api
cp .env.example .env
docker-compose up --build
```

```bash
# Register
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"business_name":"My Store","email":"dev@mystore.com","password":"secret123"}'

# Get secret key
curl http://localhost:8080/api/v1/auth/keys \
  -H "Authorization: Bearer <token>"

# Initialize transaction
curl -X POST http://localhost:8080/transaction/initialize \
  -H "Authorization: Bearer sk_test_xxx" \
  -H "Content-Type: application/json" \
  -d '{"email":"customer@example.com","amount":10000}'
```

---

## Response Format

Identical to Paystack:

```json
{
  "status": true,
  "message": "Authorization URL created",
  "data": {
    "authorization_url": "http://localhost:3000?access_code=ACC_xxx",
    "access_code": "ACC_xxx",
    "reference": "TXN_xxx"
  }
}
```

Errors:

```json
{
  "status": false,
  "message": "Validation error has occurred",
  "errors": {
    "email": [{ "rule": "required", "message": "Email is required" }],
    "amount": [{ "rule": "min", "message": "Amount must be greater than 0" }]
  }
}
```

---

## Charge Flows

### Card — Local (Verve: 5061, 5062, 5063, 6500, 6501)

```bash
# 1. Charge
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
# → { "data": { "status": "send_pin" } }

# 2. Submit PIN
curl -X POST http://localhost:8080/charge/submit_pin \
  -H "Authorization: Bearer sk_test_xxx" \
  -H "Content-Type: application/json" \
  -d '{"reference":"TXN_xxx","pin":"1234"}'
# → { "data": { "status": "send_otp" } }

# 3. Get OTP (no real phone needed)
curl "http://localhost:8080/api/v1/control/otp-logs?reference=TXN_xxx" \
  -H "Authorization: Bearer <jwt>"

# 4. Submit OTP
curl -X POST http://localhost:8080/charge/submit_otp \
  -H "Authorization: Bearer sk_test_xxx" \
  -H "Content-Type: application/json" \
  -d '{"reference":"TXN_xxx","otp":"482931"}'
# → { "data": { "status": "success" } }
```

### Card — International (Visa 4xxx, Mastercard 5xxx)

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
# → { "data": { "status": "open_url", "url": "http://localhost:3000/simulate/3ds/TXN_xxx" } }
```

### Mobile Money

```bash
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
# → { "data": { "status": "send_otp" } }
```

### Bank Transfer

```bash
curl -X POST http://localhost:8080/charge \
  -H "Authorization: Bearer sk_test_xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "customer@example.com",
    "amount": 10000,
    "bank": {
      "code": "GCB",
      "account_number": "1234567890"
    },
    "birthday": "1990-01-15"
  }'
# → { "data": { "status": "send_birthday" } }
```

---

## Routes

**Paystack-compatible (no prefix):**

```
POST   /transaction/initialize
GET    /transaction/verify/:reference
GET    /transaction
GET    /transaction/:id
POST   /transaction/:id/refund

POST   /charge
POST   /charge/submit_pin
POST   /charge/submit_otp
POST   /charge/submit_birthday
POST   /charge/submit_address
POST   /charge/resend_otp
GET    /charge/:reference

POST   /customer
GET    /customer
GET    /customer/:code
PUT    /customer/:code
GET    /customer/:code/transactions
```

**Payfake-only (/api/v1):**

```
POST   /api/v1/auth/register
POST   /api/v1/auth/login
POST   /api/v1/auth/refresh
POST   /api/v1/auth/logout
GET    /api/v1/auth/me
GET    /api/v1/auth/keys
POST   /api/v1/auth/keys/regenerate

GET    /api/v1/merchant
PUT    /api/v1/merchant
PUT    /api/v1/merchant/password
GET    /api/v1/merchant/webhook
POST   /api/v1/merchant/webhook
POST   /api/v1/merchant/webhook/test

GET    /api/v1/control/stats
GET    /api/v1/control/transactions
GET    /api/v1/control/customers
GET    /api/v1/control/scenario
PUT    /api/v1/control/scenario
POST   /api/v1/control/scenario/reset
GET    /api/v1/control/webhooks
POST   /api/v1/control/webhooks/:id/retry
GET    /api/v1/control/webhooks/:id/attempts
POST   /api/v1/control/transactions/:ref/force
GET    /api/v1/control/logs
DELETE /api/v1/control/logs
GET    /api/v1/control/otp-logs

GET    /api/v1/public/transaction/verify/:reference
GET    /api/v1/public/transaction/:access_code
POST   /api/v1/public/charge
POST   /api/v1/public/charge/submit_pin
POST   /api/v1/public/charge/submit_otp
POST   /api/v1/public/charge/submit_birthday
POST   /api/v1/public/charge/submit_address
POST   /api/v1/public/charge/resend_otp
POST   /api/v1/public/simulate/3ds/:reference
```

---

## SDKs

| Language | Repository |
|----------|-----------|
| Go | [payfake/payfake-go](https://github.com/payfake/payfake-go) |
| Python | [payfake/payfake-python](https://github.com/payfake/payfake-python) |
| JavaScript | [payfake/payfake-js](https://github.com/payfake/payfake-js) |
| Rust | [payfake/payfake-rust](https://github.com/payfake/payfake-rust) |

---

## License

MIT
