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
## [0.2.0] — 2026-04-17

### Added

**Multi-step charge flows**
- Card local (Verve) flow: send_pin → send_otp → success/failed
- Card international (Visa/Mastercard) flow: open_url → 3DS simulation → success/failed
- Mobile money flow: send_otp → pay_offline → webhook resolution
- Bank transfer flow: send_birthday → send_otp → success/failed
- POST /charge/submit_pin — card PIN submission
- POST /charge/submit_otp — OTP submission for card, MoMo and bank flows
- POST /charge/submit_birthday — date of birth for bank flow
- POST /charge/submit_address — billing address for AVS
- POST /charge/resend_otp — regenerate OTP (old one invalidated)
- GET  /charge/:reference — fetch current charge flow state
- Public equivalents for all submit endpoints under /api/v1/public/
- POST /api/v1/public/simulate/3ds/:reference — complete simulated 3DS

**OTP simulation**
- crypto/rand 6-digit OTP generation per charge step
- OTP stored on charge record, never returned in API response
- OTP visible in /control/logs introspection for developer testing

**Card type detection**
- Verve ranges (5061, 5062, 5063, 6500, 6501) → local → PIN flow
- All other Visa/Mastercard → international → 3DS flow

**3DS simulation**
- three_ds_url points to React checkout app /simulate/3ds route
- POST /public/simulate/3ds/:reference resolves charge via JSON API

**Cookie-based dashboard auth**
- Access token (15 min) + refresh token (7 days) set as HttpOnly cookies
- Refresh token rotation on every /auth/refresh call
- POST /auth/refresh — exchange refresh cookie for new token pair
- GET  /auth/me — hydrate dashboard session on mount
- POST /auth/logout — clear both cookies

**Merchant management**
- GET  /merchant — full profile
- PUT  /merchant — update business name and webhook URL
- PUT  /merchant/password — change password with current password verification
- GET  /merchant/webhook — webhook URL and status
- POST /merchant/webhook — set webhook URL from dashboard
- POST /merchant/webhook/test — fire test webhook to verify endpoint

**Dashboard endpoints (JWT)**
- GET /control/stats — overview numbers + 7-day activity chart
- GET /control/transactions — transaction list without secret key
- GET /control/customers — customer list without secret key

**CORS**
- Single root-level CORS middleware using AllowOriginFunc
- AllowCredentials: true for HttpOnly cookie support
- OPTIONS preflight handled before any route middleware

**Config**
- JWT_ACCESS_EXPIRY_MINUTES (default 15)
- JWT_REFRESH_EXPIRY_DAYS (default 7)
- FRONTEND_URL used for authorization_url and three_ds_url generation


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
