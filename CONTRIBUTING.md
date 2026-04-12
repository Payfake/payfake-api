# Contributing to Payfake

Thanks for your interest in contributing. Payfake is built for African developers
and contributions that improve African payment rail coverage, developer experience,
or documentation are especially welcome.

---

## Before You Start

- Check [open issues](https://github.com/payfake/payfake-api/issues) to avoid duplicate work
- For significant changes open an issue first to discuss the approach
- For small fixes (typos, docs, minor bugs) just open a PR directly

---

## Development Setup

**Prerequisites:** Go 1.21+, PostgreSQL 14+, Docker (optional)

```bash
git clone https://github.com/payfake/payfake-api
cd payfake-api
cp .env.example .env
# fill in your local DB credentials
go mod tidy
go run cmd/api/main.go
```

Verify the server is running:

```bash
curl http://localhost:8080/health
```

---

## Project Structure

```
payfake/
├── cmd/api/main.go         → entry point
├── internal/
│   ├── config/             → environment config
│   ├── database/           → GORM connection and migrations
│   ├── domain/             → entity structs and constants
│   ├── repository/         → DB access — no business logic here
│   ├── service/            → all business logic lives here
│   ├── handler/            → thin HTTP layer — parse, call service, respond
│   ├── middleware/         → auth, CORS, logging, rate limiting
│   ├── router/             → route and dependency wiring
│   └── response/           → envelope helpers and response codes
├── pkg/
│   ├── keygen/             → API key generation
│   ├── crypto/             → HMAC signing
│   └── uid/                → prefixed ID generation
```

**The layer contract is strict:**

```
handler → service → repository → database
```

- Handlers never call repositories directly
- Services never call handlers
- Repositories contain zero business logic
- Domain structs are imported by all layers, nothing else is

---

## Code Style

- Follow standard Go conventions, `gofmt`, `govet`, `golint`
- Every exported function gets a comment
- Comments explain *why*, not *what* — the code shows what, comments show reasoning
- Sentinel errors in `service/errors.go` — never return raw strings from services
- Response codes in `response/codes.go` — never use raw strings in handlers

---

## Adding a New Endpoint

1. Add the route constant to `response/codes.go`
2. Add the domain type to `internal/domain/` if needed
3. Add the repository method to the appropriate `internal/repository/` file
4. Add the service method with business logic to `internal/service/`
5. Add the handler method to `internal/handler/`
6. Wire the route in `internal/router/router.go`
7. Update `docs/api-reference.md`

---

## Adding a New Simulation Scenario

The simulation engine lives in `internal/service/simulator_service.go`.

`ResolveOutcome()` is the single function that decides every charge outcome.
It follows a priority order:

1. `ForceStatus` — if set, always return this. No randomness.
2. `FailureRate` — random roll against configured probability.
3. Default — succeed.

Channel-specific error codes live in `defaultErrorCode()`. Add new
African payment rail failure modes here with a comment explaining
when they occur in production.

---

## Commit Convention

We use conventional commits:

```
feat: add Flutterwave-compatible charge endpoint
fix: correct MoMo timeout error code for Telecel
docs: add webhook verification example for Node.js
refactor: extract charge resolution into separate method
test: add scenario engine unit tests
```

Commit body bullets explain the decisions made:

```
feat: add bank transfer retry logic

- banks in Ghana often return transient errors on first attempt
- retry up to 3 times with 500ms backoff before marking failed
- add CHARGE_BANK_RETRY_EXHAUSTED error code for final failure
- webhook fires only after all retries are exhausted
```

---

## Pull Request Checklist

- [ ] `go build ./...` passes with no errors
- [ ] `go vet ./...` passes with no warnings
- [ ] New endpoints documented in `docs/api-reference.md`
- [ ] New error codes added to `response/codes.go` with comments
- [ ] Commit messages follow the conventional commit format
- [ ] No secrets or credentials in the diff

---

## Areas That Need Work

- Unit tests for the simulator engine
- Integration tests for the full charge flow
- Flutterwave-compatible API surface
- Nigeria-specific payment channels (USSD, QR)
- Kenya-specific channels (M-Pesa)
- Rate limiting per merchant (currently global only)
- Webhook retry worker (background process for failed deliveries)
- Admin panel for managing multiple merchants

---

## Questions

Open a GitHub issue with the `question` label or reach out directly.
