# Security Policy

## Supported Versions

Payfake is currently in active development. Security fixes are applied to
the latest version only.

| Version | Supported |
|---------|-----------|
| 0.1.x   | ✅ Yes    |

---

## Reporting a Vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Report security issues directly via email. Include:

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix if you have one

You will receive a response within 48 hours. If the vulnerability is confirmed
we will release a fix and credit you in the changelog unless you prefer to
remain anonymous.

---

## Security Design Decisions

**Secret keys never touch the frontend.**
The public checkout endpoints (`/api/v1/public/*`) authenticate via `access_code`
— a short-lived single-use token tied to one transaction. The merchant's `sk_test_`
key is never sent to or required by the browser.

**Webhook signatures.**
Every webhook Payfake fires is signed with the merchant's secret key using
HMAC-SHA512 identical to Paystack's scheme. Developers should always verify
the `X-Paystack-Signature` header before processing webhook events.

**Password hashing.**
Merchant passwords are hashed with bcrypt at cost factor 12. Plain text passwords
are never stored or logged.

**Constant-time comparisons.**
Password verification uses `bcrypt.CompareHashAndPassword` and webhook signature
verification uses `hmac.Equal`, both are constant-time to prevent timing attacks.

**Same error messages for auth failures.**
Login returns the same error message whether the email doesn't exist or the
password is wrong. This prevents email enumeration attacks.

**JWT separate from API keys.**
Dashboard sessions use short-lived JWTs. API keys are long-lived but rotating
a key doesn't invalidate dashboard sessions and vice versa.

**No sensitive data in logs.**
The request logger captures request and response bodies but the auth middleware
strips Authorization headers before they reach the logger. Card numbers are
never stored in full, only the last 4 digits.

---

## Scope

Payfake is a **simulator** — it handles no real money and stores no real
financial data. The security boundary is protecting merchant accounts and
preventing cross-merchant data access, not protecting real financial transactions.

That said, security issues that would allow:
- One merchant to access another merchant's data
- Bypassing authentication on protected endpoints
- Injecting malicious scenario configs
- Leaking secret keys through any endpoint

...are all in scope and should be reported.
