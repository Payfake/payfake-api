# Getting Started

This guide walks you through setting up Payfake locally and making your
first simulated payment end-to-end including the full multi-step charge flow.

---

## Prerequisites

- Go 1.21+ (if running without Docker)
- PostgreSQL 14+ (if running without Docker)
- Docker and Docker Compose (recommended)

---

## Installation

### Option 1 — Docker (recommended)

```bash
git clone https://github.com/payfake/payfake-api
cd payfake
docker-compose up --build
```

Server starts at `http://localhost:8080`.

### Option 2 — Manual

```bash
git clone https://github.com/payfake/payfake-api
cd payfake
cp .env.example .env
```

Edit `.env`:

```bash
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=yourpassword
DB_NAME=payfake
JWT_SECRET=your-secret-key-change-this
FRONTEND_URL=http://localhost:3000
JWT_ACCESS_EXPIRY_MINUTES=15
JWT_REFRESH_EXPIRY_DAYS=7
```

```bash
go mod tidy
go run cmd/api/main.go
```

---

## Verify

```bash
curl http://localhost:8080/health
```

```json
{
  "status": "success",
  "message": "Payfake is running",
  "data": { "service": "payfake", "status": "ok" }
}
```

---

## Step 1 — Register

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "business_name": "My Store",
    "email": "dev@mystore.com",
    "password": "secret123"
  }'
```

Save the `token` from the response.

---

## Step 2 — Get Your Secret Key

```bash
curl http://localhost:8080/api/v1/auth/keys \
  -H "Authorization: Bearer <token>"
```

Save `sk_test_xxx` — this goes in your backend's environment variables.

---

## Step 3 — Initialize a Transaction

```bash
curl -X POST http://localhost:8080/api/v1/transaction/initialize \
  -H "Authorization: Bearer sk_test_xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "customer@example.com",
    "amount": 10000,
    "currency": "GHS",
    "callback_url": "http://localhost:3000/callback"
  }'
```

Response:
```json
{
  "data": {
    "authorization_url": "http://localhost:3000?access_code=ACC_xxx",
    "access_code": "ACC_xxxxxxxxxxxx",
    "reference": "TXN_xxxxxxxxxxxx"
  }
}
```

The `authorization_url` is what you redirect your customer to.

---

## Step 4 — Simulate a Local Card Charge (Full Flow)

### 4a — Initiate card charge
```bash
curl -X POST http://localhost:8080/api/v1/charge/card \
  -H "Authorization: Bearer sk_test_xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "access_code": "ACC_xxxxxxxxxxxx",
    "card_number": "5061000000000000",
    "card_expiry": "12/26",
    "cvv": "123",
    "email": "customer@example.com"
  }'
```

Response:
```json
{
  "data": {
    "status": "send_pin",
    "reference": "TXN_xxxxxxxxxxxx",
    "display_text": "Please enter your card PIN"
  }
}
```

### 4b — Submit PIN
```bash
curl -X POST http://localhost:8080/api/v1/charge/submit_pin \
  -H "Authorization: Bearer sk_test_xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "reference": "TXN_xxxxxxxxxxxx",
    "pin": "1234"
  }'
```

Response:
```json
{
  "data": {
    "status": "send_otp",
    "display_text": "Enter the OTP sent to your registered phone number"
  }
}
```

### 4c — Get the OTP from logs

```bash
curl http://localhost:8080/api/v1/control/logs?per_page=5 \
  -H "Authorization: Bearer <jwt>"
```

Look for the `submit_pin` log entry — the OTP is in the response body.

### 4d — Submit OTP

```bash
curl -X POST http://localhost:8080/api/v1/charge/submit_otp \
  -H "Authorization: Bearer sk_test_xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "reference": "TXN_xxxxxxxxxxxx",
    "otp": "482931"
  }'
```

Response:
```json
{
  "data": {
    "status": "success",
    "reference": "TXN_xxxxxxxxxxxx",
    "transaction": { "status": "success", "amount": 10000 }
  }
}
```

---

## Step 5 — Verify

```bash
curl http://localhost:8080/api/v1/transaction/verify/TXN_xxxxxxxxxxxx \
  -H "Authorization: Bearer sk_test_xxx"
```

---

## Step 6 — Test a Failure Scenario

```bash
# Get JWT first
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "dev@mystore.com", "password": "secret123"}'

# Force all charges to fail with insufficient funds
curl -X PUT http://localhost:8080/api/v1/control/scenario \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "force_status": "failed",
    "error_code": "CHARGE_INSUFFICIENT_FUNDS"
  }'

# Reset when done
curl -X POST http://localhost:8080/api/v1/control/scenario/reset \
  -H "Authorization: Bearer <jwt>"
```

---

## Next Steps

- [Charge Flows](./charge-flows.md) — all four payment channels step by step
- [Scenario Testing](./scenario-testing.md) — failure rates, delays, force outcomes
- [Webhook Integration](./webhooks.md) — receiving and verifying webhook events
- [Mobile Money Guide](./mobile-money.md) — async MoMo flow end to end
- [API Reference](./api-reference.md) — complete endpoint documentation
- [SDK Guides](./sdks.md) — Go, Python, JS and Rust SDKs
