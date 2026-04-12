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
