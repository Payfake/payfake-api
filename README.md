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

Payfake fills that gap. It's a glass box — you see every request, every response, every webhook attempt, every OTP generated. You control every outcome.

---

## Features

- **Paystack-compatible API** — same URL structure, same payload keys, same response shapes. Swap one base URL and nothing else changes in your code
- **Full multi-step charge flows** — card PIN → OTP, MoMo OTP → USSD prompt, bank birthday → OTP, international card 3DS — mirrors real Paystack flows exactly
- **Card, Mobile Money and Bank Transfer simulation** — including Ghana-specific channels (MTN MoMo, Vodafone Cash, AirtelTigo)
- **OTP simulation** — 6-digit OTPs generated per charge step, visible in `/control/logs` — no real phone needed during testing
- **3DS simulation** — international cards return an `open_url` pointing to the React checkout app's 3DS page
- **Webhook delivery** — fires `charge.success`, `charge.failed`, `refund.processed` with HMAC-SHA512 signatures identical to Paystack's
- **Scenario engine** — configure failure rates, artificial delays, and forced outcomes per merchant
- **Force endpoint** — deterministically force any pending transaction to any terminal state
- **Introspection logs** — every request and response stored and queryable from the dashboard
- **Cookie-based dashboard auth** — HttpOnly access + refresh tokens, automatic rotation
- **React checkout page** — hosted payment popup served separately, talks to the public charge endpoints
- **Docker support** — single command to run the full stack

---

## Quick Start

### With Docker (recommended)

```bash
git clone https://github.com/payfake/payfake-api.git
cd payfake
docker-compose up --build
```

Server live at `http://localhost:8080`.

### Without Docker

**Prerequisites:** Go 1.21+, PostgreSQL 14+

```bash
git clone https://github.com/payfake/payfake-api.git
cd payfake
cp .env.example .env
# edit .env with your database credentials
go mod tidy
go run cmd/api/main.go
```

---

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `APP_NAME` | No | `payfake` | Application name |
| `APP_ENV` | No | `development` | `development` or `production` |
| `APP_PORT` | No | `8080` | HTTP server port |
| `FRONTEND_URL` | No | `http://localhost:3000` | React checkout + dashboard URL |
| `DB_HOST` | Yes | `localhost` | PostgreSQL host |
| `DB_PORT` | No | `5432` | PostgreSQL port |
| `DB_USER` | Yes | — | PostgreSQL user |
| `DB_PASSWORD` | Yes | — | PostgreSQL password |
| `DB_NAME` | Yes | — | PostgreSQL database name |
| `DB_SSLMODE` | No | `disable` | PostgreSQL SSL mode |
| `JWT_SECRET` | Yes | — | Secret for signing JWT tokens |
| `JWT_ACCESS_EXPIRY_MINUTES` | No | `15` | Access token expiry in minutes |
| `JWT_REFRESH_EXPIRY_DAYS` | No | `7` | Refresh token expiry in days |

---

## Authentication

Three authentication schemes:

**Secret Key** — server-side API calls (transaction, charge, customer):
```
Authorization: Bearer sk_test_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

**Cookie** — dashboard (set automatically on login/register):
```
Cookie: payfake_access=eyJ...   (HttpOnly, 15 min)
Cookie: payfake_refresh=eyJ...  (HttpOnly, 7 days, path=/api/v1/auth/refresh only)
```

**Access Token in header** — SDK/programmatic dashboard access:
```
Authorization: Bearer eyJ...
```

**Public endpoints** (`/api/v1/public/*`) — no Authorization header. Authenticated via `access_code` in the request body.

---

## Charge Flows

Payfake simulates the full multi-step charge flow exactly as Paystack does. Every charge endpoint returns a `status` field that tells the checkout page what to do next.

### Card — Local (Ghana cards: Verve 5061, 5062, 5063, 6500, 6501)
```
POST /charge/card
└── status: "send_pin"         → customer enters card PIN

POST /charge/submit_pin
└── status: "send_otp"         → OTP sent to registered phone

POST /charge/submit_otp
└── status: "success"          → webhook fires charge.success
    status: "failed"           → webhook fires charge.failed
```

### Card — International (Visa 4xxx, Mastercard 5xxx)
```
POST /charge/card
└── status: "open_url"         → three_ds_url returned
                                  checkout opens http://localhost:3000/simulate/3ds/:reference

POST /simulate/3ds/:reference  → customer confirms fake 3DS form
└── status: "success"          → webhook fires charge.success
    status: "failed"           → webhook fires charge.failed
```

### Mobile Money
```
POST /charge/mobile_money
└── status: "send_otp"         → OTP sent to MoMo phone number

POST /charge/submit_otp
└── status: "pay_offline"      → USSD prompt sent, waiting for approval
                                  poll GET /public/transaction/:access_code
                                  webhook fires when resolved
```

### Bank Transfer
```
POST /charge/bank
└── status: "send_birthday"    → customer enters date of birth

POST /charge/submit_birthday
└── status: "send_otp"         → OTP sent to registered phone

POST /charge/submit_otp
└── status: "success"          → webhook fires charge.success
    status: "failed"           → webhook fires charge.failed
```

### OTP During Testing

OTPs are generated server-side but never returned in any API response. Read them from the introspection logs:

```bash
curl http://localhost:8080/api/v1/control/logs \
  -H "Authorization: Bearer <jwt>" | jq '.data.logs[0].response_body'
```

The OTP appears in the response body of the charge or submit_pin log entry.

---

## Payment Flow (Full)

```
Your Backend                    Payfake                  React Checkout
────────────                    ───────                  ──────────────
POST /transaction/initialize ──► creates pending tx
                             ◄── access_code + auth_url
                                        │
                             customer opens auth_url
                             (http://localhost:3000?access_code=ACC_xxx)
                                        │
                             GET /public/transaction/:access_code
                             ◄── amount, currency, merchant name, customer email
                                        │
                             customer selects payment method and fills details
                                        │
                             POST /public/charge/card
                             ◄── { status: "send_pin" }
                                        │
                             customer enters PIN
                             POST /public/charge/submit_pin
                             ◄── { status: "send_otp" }
                                        │
                             customer enters OTP (read from /control/logs in dev)
                             POST /public/charge/submit_otp
                             ◄── { status: "success" }
                                        │
                             ◄── webhook fires to your backend
                                        │
Your Backend                            │
POST /transaction/verify/:ref ──► confirms final status
                             ◄── { status: "success" }
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
├── cmd/api/main.go
├── internal/
│   ├── config/
│   ├── database/
│   ├── domain/          → entity structs, flow status constants
│   ├── repository/      → DB access layer
│   ├── service/         → business logic, simulator engine, charge flows
│   ├── handler/         → HTTP handlers
│   ├── middleware/       → auth, CORS, logging, rate limit
│   ├── router/          → route wiring
│   └── response/        → envelope and response codes
├── pkg/
│   ├── keygen/          → pk/sk key generation
│   ├── crypto/          → HMAC webhook signing
│   ├── uid/             → prefixed ID generation
│   └── otp/             → cryptographic OTP generation
├── docs/
├── Dockerfile
└── docker-compose.yml
```

---

## License

MIT
