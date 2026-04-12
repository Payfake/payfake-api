# Getting Started

This guide walks you through setting up Payfake locally and making your
first simulated payment end-to-end.

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

The server starts at `http://localhost:8080` with a fresh PostgreSQL instance.
All tables are created automatically on first run.

### Option 2 — Manual

```bash
git clone https://github.com/payfake/payfake-api
cd payfake
cp .env.example .env
```

Edit `.env` with your local database credentials:

```bash
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=yourpassword
DB_NAME=payfake
JWT_SECRET=your-secret-key-change-this
```

Run:

```bash
go mod tidy
go run cmd/api/main.go
```

---

## Verify the Server is Running

```bash
curl http://localhost:8080/health
```

Expected response:

```json
{
  "status": "success",
  "message": "Payfake is running",
  "data": { "service": "payfake", "status": "ok" },
  "metadata": { "timestamp": "...", "request_id": "..." },
  "code": "HEALTH_OK"
}
```

---

## Step 1 — Register a Merchant

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "business_name": "My Store",
    "email": "dev@mystore.com",
    "password": "secret123"
  }'
```

Response:

```json
{
  "data": {
    "merchant": {
      "id": "MRC_xxxxxxxxxxxx",
      "business_name": "My Store",
      "email": "dev@mystore.com",
      "public_key": "pk_test_xxx"
    },
    "token": "eyJ..."
  }
}
```

Save the `token` — you need it to fetch your secret key.

---

## Step 2 — Get Your Secret Key

```bash
curl http://localhost:8080/api/v1/auth/keys \
  -H "Authorization: Bearer eyJ..."
```

Response:

```json
{
  "data": {
    "public_key": "pk_test_xxx",
    "secret_key": "sk_test_xxx"
  }
}
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
    "authorization_url": "http://localhost:5173?access_code=ACC_xxx",
    "access_code": "ACC_xxxxxxxxxxxx",
    "reference": "TXN_xxxxxxxxxxxx"
  }
}
```

The `authorization_url` is what you redirect your customer to. The `access_code`
is what the checkout page uses to identify the transaction.

---

## Step 4 — Charge a Card

```bash
curl -X POST http://localhost:8080/api/v1/charge/card \
  -H "Authorization: Bearer sk_test_xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "access_code": "ACC_xxxxxxxxxxxx",
    "card_number": "4111111111111111",
    "card_expiry": "12/26",
    "cvv": "123",
    "email": "customer@example.com"
  }'
```

Response on success:

```json
{
  "data": {
    "transaction": { "status": "success", "amount": 10000, ... },
    "charge": { "channel": "card", "card_last4": "1111", ... }
  },
  "code": "CHARGE_SUCCESSFUL"
}
```

---

## Step 5 — Verify the Transaction

```bash
curl http://localhost:8080/api/v1/transaction/verify/TXN_xxxxxxxxxxxx \
  -H "Authorization: Bearer sk_test_xxx"
```

Always verify via this endpoint after a charge — never trust the charge
response alone. This is the same pattern Paystack requires.

---

## Step 6 — Test a Failure Scenario

Login to get a JWT then configure a failure scenario:

```bash
# Login
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "dev@mystore.com", "password": "secret123"}'

# Set 100% failure rate
curl -X PUT http://localhost:8080/api/v1/control/scenario \
  -H "Authorization: Bearer eyJ..." \
  -H "Content-Type: application/json" \
  -d '{"failure_rate": 1.0}'
```

Every subsequent charge will now fail. Reset when done:

```bash
curl -X POST http://localhost:8080/api/v1/control/scenario/reset \
  -H "Authorization: Bearer eyJ..."
```

---

## Next Steps

- [Scenario Testing](./scenario-testing.md) — failure rates, delays, force outcomes
- [Webhook Integration](./webhooks.md) — receiving and verifying webhook events
- [Mobile Money Guide](./mobile-money.md) — async MoMo flow end-to-end
- [API Reference](./api-reference.md) — complete endpoint documentation
- [SDK Guides](./sdks.md) — using the Go, Python, JS and Rust SDKs
