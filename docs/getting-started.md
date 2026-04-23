# Getting Started

---

## Install

```bash
git clone https://github.com/payfake/payfake-api
cd payfake-api
cp .env.example .env
docker-compose up --build
```

Server at `http://localhost:8080`.

For browser checkout flows under `/api/v1/public`, keep both the `access_code`
and the transaction `reference`. Public follow-up calls require both.

---

## Register

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"business_name":"My Store","email":"dev@mystore.com","password":"secret123"}'
```

```json
{
  "status": true,
  "message": "Account created",
  "data": {
    "merchant": { "id": "MRC_xxx", "email": "dev@mystore.com" },
    "access_token": "eyJ..."
  }
}
```

---

## Get Your Keys

```bash
curl http://localhost:8080/api/v1/auth/keys \
  -H "Authorization: Bearer eyJ..."
```

```json
{
  "status": true,
  "data": {
    "public_key": "pk_test_xxx",
    "secret_key": "sk_test_xxx"
  }
}
```

Set in your `.env`:

```bash
PAYSTACK_SECRET_KEY=sk_test_xxx
PAYSTACK_BASE_URL=http://localhost:8080
```

---

## Initialize a Transaction

```bash
curl -X POST http://localhost:8080/transaction/initialize \
  -H "Authorization: Bearer sk_test_xxx" \
  -H "Content-Type: application/json" \
  -d '{"email":"customer@example.com","amount":10000,"currency":"GHS"}'
```

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

---

## Full Local Card Flow

```bash
# Step 1 — charge
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
# → data.status: "send_pin"

# Step 2 — PIN
curl -X POST http://localhost:8080/charge/submit_pin \
  -H "Authorization: Bearer sk_test_xxx" \
  -H "Content-Type: application/json" \
  -d '{"reference":"TXN_xxx","pin":"1234"}'
# → data.status: "send_otp"

# Step 3 — get OTP
curl "http://localhost:8080/api/v1/control/otp-logs?reference=TXN_xxx" \
  -H "Authorization: Bearer eyJ..."
# → data.data[0].otp_code: "482931"

# Step 4 — OTP
curl -X POST http://localhost:8080/charge/submit_otp \
  -H "Authorization: Bearer sk_test_xxx" \
  -H "Content-Type: application/json" \
  -d '{"reference":"TXN_xxx","otp":"482931"}'
# → data.status: "success"

# Step 5 — verify
curl http://localhost:8080/transaction/verify/TXN_xxx \
  -H "Authorization: Bearer sk_test_xxx"
# → data.status: "success", data.gateway_response: "Approved"
```

---

## Force a Failure

```bash
# Login to get JWT
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"dev@mystore.com","password":"secret123"}'

# Force all charges to fail
curl -X PUT http://localhost:8080/api/v1/control/scenario \
  -H "Authorization: Bearer eyJ..." \
  -H "Content-Type: application/json" \
  -d '{"force_status":"failed","error_code":"CHARGE_INSUFFICIENT_FUNDS"}'

# Reset
curl -X POST http://localhost:8080/api/v1/control/scenario/reset \
  -H "Authorization: Bearer eyJ..."
```

---

## Next Steps

- [Charge Flows](./charge-flows.md)
- [Scenario Testing](./scenario-testing.md)
- [Webhooks](./webhooks.md)
- [Mobile Money](./mobile-money.md)
- [API Reference](./api-reference.md)
- [Integration Guide](./integration.md)
