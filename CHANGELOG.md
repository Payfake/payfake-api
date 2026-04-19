# Changelog

All notable changes to Payfake are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

---
## [Unreleased]

### Planned
- Unit and integration test suite
- Flutterwave-compatible API surface
- Nigeria USSD and QR channels
- Kenya M-Pesa channel
- Webhook retry background worker
- Admin panel for multi-merchant management
- Rate limiting per merchant key


---
## [0.2.0] — 2026-04-19

### Added

**Multi-step charge flows** — mirrors real Paystack flows exactly
- Card local (Verve): send_pin → send_otp → success/failed
- Card international (Visa/Mastercard): open_url → 3DS → success/failed
- Mobile money: send_otp → pay_offline → webhook
- Bank transfer: send_birthday → send_otp → success/failed
- POST /charge/submit_pin
- POST /charge/submit_otp
- POST /charge/submit_birthday
- POST /charge/submit_address
- POST /charge/resend_otp
- GET  /charge/:reference
- Public equivalents for all submit endpoints

**OTP simulation**
- crypto/rand 6-digit OTP per charge step
- OTPLog domain model — reference, channel, step, used, expires_at
- OTP expiry enforced — 10 minute window, rejected after expiry
- GET /control/otp-logs?reference=xxx — read OTPs without a real phone

**3DS simulation**
- three_ds_url points to React checkout app /simulate/3ds route
- POST /public/simulate/3ds/:reference resolves via JSON API

**MoMo polling**
- GET /public/transaction/verify/:reference — public polling endpoint
- Returns transaction status + charge flow_status
- Checkout page polls every 3s during pay_offline state

**Cookie-based dashboard auth**
- Access token (15 min) + refresh token (7 days) as HttpOnly cookies
- Refresh token rotation on every /auth/refresh
- POST /auth/refresh, GET /auth/me, POST /auth/logout

**Merchant management**
- GET/PUT /merchant — profile management
- PUT /merchant/password — password change with current password verification
- GET/POST /merchant/webhook — webhook URL management
- POST /merchant/webhook/test — test webhook with rate limiting (5/min)

**Dashboard endpoints**
- GET /control/stats — overview + 7-day activity chart
- GET /control/transactions — JWT-based with search and status filter
- GET /control/customers — JWT-based

**Security fixes**
- OTP expiry enforcement — 10 minute window
- Cross-merchant reference validation on public submit endpoints
- FindByTransactionID returns latest charge (DESC order)
- 3DS URL uses FRONTEND_URL from config not hardcoded localhost
- 2MB request body size limit
- Graceful shutdown — SIGINT/SIGTERM with 10 second drain window
- Webhook retry worker — background goroutine, 60 second tick, context-aware

**SDKs — all four updated**
- SubmitPIN, SubmitOTP, SubmitBirthday, SubmitAddress, ResendOTP, Simulate3DS
- GetOTPLogs with reference filter
- ListTransactions (JWT, with search)
- ListCustomers (JWT)
- GetProfile, UpdateProfile
- GetWebhookURL, UpdateWebhookURL, TestWebhook
- MerchantProfile, ChargeFlowResponse, OTPLog types

---

## [0.1.0] — 2026-04-12

### Added

**Core server**
- Paystack-compatible REST API — same URL structure, payload keys and response shapes
- Merchant registration and authentication with `pk_test_` / `sk_test_` key pairs
- JWT-based dashboard authentication separate from API key auth
- Transaction initialize and verify flow mirroring Paystack exactly
- Card charge simulation with Luhn check and brand detection
- Mobile Money simulation — MTN, Vodafone Cash, AirtelTigo (Ghana)
- Bank transfer simulation with Ghana bank codes
- MoMo async resolution — returns `pending` immediately, resolves via webhook
- Webhook delivery with HMAC-SHA512 signatures matching Paystack's scheme
- Webhook retry and delivery attempt logging
- Scenario engine — failure rate, delay, force status per merchant
- Force endpoint — deterministically set any pending transaction to any terminal state
- Request/response introspection logs
- Public checkout endpoints — browser-safe charge routes authenticated via `access_code`
- Hosted React checkout page support (separate frontend repo)
- CORS — permissive for public checkout routes, restrictive for API routes
- Rate limiting — 200 requests per minute global
- Structured JSON logging via zerolog
- Request ID propagation across request lifecycle
- Docker and docker-compose support
- PostgreSQL with GORM AutoMigrate
- Africa/Accra timezone baked into DB connection

**SDKs**
- Go SDK (`payfake-go`) — full API coverage, thread-safe JWT handling
- Python SDK (`payfake-python`) — dataclasses, httpx, context manager support
- JavaScript SDK (`payfake-js`) — zero dependencies, native fetch, camelCase API
- Rust SDK (`payfake-rust`) — async/tokio, Arc<ClientInner>, thiserror

**Error codes**
- 60 typed response codes across auth, transaction, charge, customer, control namespaces
- Ghana-specific charge error codes: `CHARGE_DO_NOT_HONOR`, `CHARGE_MOMO_TIMEOUT`,
  `CHARGE_MOMO_PROVIDER_UNAVAILABLE`, `CHARGE_MOMO_INVALID_NUMBER`

**Documentation**
- Full API reference README
- SDK READMEs for all four languages
- Contributing guide
- This changelog
