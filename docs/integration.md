# Integration Guide

How to integrate Payfake into your existing application in under 5 minutes.
The core idea: Payfake mirrors the Paystack API exactly, so you change
one environment variable and your existing Paystack integration just works.

---

## The One-Line Change

```bash
# Before (real Paystack)
PAYSTACK_BASE_URL=https://api.paystack.co

# After (Payfake locally)
PAYSTACK_BASE_URL=http://localhost:8080
```

That's it for most integrations. Your existing code, your existing SDK,
your existing webhook handler — all unchanged.

---

## Start Payfake

```bash
git clone https://github.com/payfake/payfake
cd payfake
cp .env.example .env
docker-compose up --build
```

Server running at `http://localhost:8080`.

---

## Get Your Test Credentials

```bash
# Register a merchant account
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"business_name": "My App", "email": "dev@myapp.com", "password": "secret123"}'

# Save the token from the response, then get your keys
curl http://localhost:8080/api/v1/auth/keys \
  -H "Authorization: Bearer <token>"
```

Add your keys to `.env`:

```bash
PAYSTACK_SECRET_KEY=sk_test_xxx
PAYSTACK_PUBLIC_KEY=pk_test_xxx
PAYSTACK_BASE_URL=http://localhost:8080
```

---

## Django Integration

If you're using the official Paystack Python library or a custom wrapper:

```python
# settings.py
PAYSTACK_SECRET_KEY = os.environ["PAYSTACK_SECRET_KEY"]
PAYSTACK_BASE_URL = os.environ.get("PAYSTACK_BASE_URL", "https://api.paystack.co")

# paystack_client.py
import requests

class PaystackClient:
    def __init__(self):
        self.base_url = settings.PAYSTACK_BASE_URL
        self.secret_key = settings.PAYSTACK_SECRET_KEY
        self.session = requests.Session()
        self.session.headers.update({
            "Authorization": f"Bearer {self.secret_key}",
            "Content-Type": "application/json",
        })

    def initialize_transaction(self, email, amount, **kwargs):
        resp = self.session.post(f"{self.base_url}/transaction/initialize", json={
            "email": email,
            "amount": amount,
            **kwargs,
        })
        return resp.json()

    def verify_transaction(self, reference):
        resp = self.session.get(f"{self.base_url}/transaction/verify/{reference}")
        return resp.json()
```

Your views stay unchanged — just swap the base URL in your client constructor.

**Webhook verification in Django:**

```python
# views.py
import hashlib
import hmac
from django.http import HttpResponse
from django.views.decorators.csrf import csrf_exempt

@csrf_exempt
def paystack_webhook(request):
    # Verification works identically for Payfake and real Paystack
    signature = request.headers.get("X-Paystack-Signature", "")
    secret_key = settings.PAYSTACK_SECRET_KEY

    expected = hmac.new(
        secret_key.encode(),
        request.body,
        hashlib.sha512,
    ).hexdigest()

    if not hmac.compare_digest(expected, signature):
        return HttpResponse(status=401)

    import json
    event = json.loads(request.body)

    if event["event"] == "charge.success":
        reference = event["data"]["reference"]
        # update your order
        pass

    return HttpResponse(status=200)
```

---

## Node.js / Express Integration

```js
// config.js
export const paystackConfig = {
  secretKey: process.env.PAYSTACK_SECRET_KEY,
  baseURL: process.env.PAYSTACK_BASE_URL ?? "https://api.paystack.co",
}

// paystack.js
import axios from "axios"
import { paystackConfig } from "./config.js"

const client = axios.create({
  baseURL: paystackConfig.baseURL,
  headers: {
    Authorization: `Bearer ${paystackConfig.secretKey}`,
    "Content-Type": "application/json",
  },
})

export const initializeTransaction = (email, amount, options = {}) =>
  client.post("/transaction/initialize", { email, amount, ...options })

export const verifyTransaction = (reference) =>
  client.get(`/transaction/verify/${reference}`)
```

**Webhook handler:**

```js
import { createHmac } from "crypto"
import express from "express"

const app = express()

app.post("/webhook/paystack",
  express.raw({ type: "application/json" }),
  (req, res) => {
    const signature = req.headers["x-paystack-signature"]
    const expected = createHmac("sha512", process.env.PAYSTACK_SECRET_KEY)
      .update(req.body)
      .digest("hex")

    if (expected !== signature) return res.sendStatus(401)

    const event = JSON.parse(req.body)

    switch (event.event) {
      case "charge.success":
        // update order
        break
      case "charge.failed":
        // notify customer
        break
    }

    res.sendStatus(200)
  }
)
```

---

## Go Integration

```go
// paystack/client.go
package paystack

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
)

type Client struct {
    baseURL   string
    secretKey string
    http      *http.Client
}

func New() *Client {
    baseURL := os.Getenv("PAYSTACK_BASE_URL")
    if baseURL == "" {
        baseURL = "https://api.paystack.co"
    }
    return &Client{
        baseURL:   baseURL,
        secretKey: os.Getenv("PAYSTACK_SECRET_KEY"),
        http:      &http.Client{},
    }
}

func (c *Client) InitializeTransaction(email string, amount int64) (map[string]any, error) {
    body, _ := json.Marshal(map[string]any{"email": email, "amount": amount})
    req, _ := http.NewRequest("POST",
        c.baseURL+"/transaction/initialize",
        bytes.NewBuffer(body))
    req.Header.Set("Authorization", "Bearer "+c.secretKey)
    req.Header.Set("Content-Type", "application/json")

    resp, err := c.http.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result map[string]any
    json.NewDecoder(resp.Body).Decode(&result)
    return result, nil
}
```

Or use the Payfake Go SDK directly — it supports the same base URL override:

```go
import payfake "github.com/payfake/payfake-go"

client := payfake.New(payfake.Config{
    SecretKey: os.Getenv("PAYSTACK_SECRET_KEY"),
    BaseURL:   os.Getenv("PAYSTACK_BASE_URL"), // http://localhost:8080 locally
})
```

---

## Using the Payfake SDKs Directly

If you're building a new project and want the best developer experience,
use the Payfake SDKs — they support the full multi-step charge flow:

| Language | Install |
|----------|---------|
| Go | `go get github.com/payfake/payfake-go` |
| Python | `pip install payfake` |
| JavaScript | `npm install payfake-js` |
| Rust | `payfake = { git = "..." }` |

See [SDK Guides](./sdks.md) for usage.

---

## Testing the Charge Flow

Once Payfake is running and your app is pointing at it:

### Card payment (local Verve card)

Use card number `5061000000000000` to trigger the PIN → OTP flow:

```bash
# 1. Your app calls initialize
# 2. Customer is redirected to authorization_url
# 3. Checkout page opens, customer enters card details
# 4. Status returns "send_pin" → customer enters any 4-digit PIN
# 5. Status returns "send_otp" → get OTP:

curl "http://localhost:8080/api/v1/control/otp-logs?reference=TXN_xxx" \
  -H "Authorization: Bearer <jwt>"

# 6. Customer enters OTP → payment succeeds
# 7. Webhook fires to your backend
# 8. Your backend calls verify to confirm
```

### International card (Visa/Mastercard)

Use card number `4111111111111111` to trigger the 3DS flow:

```bash
# Status returns "open_url" with three_ds_url
# Customer is redirected to three_ds_url (React checkout app /simulate/3ds page)
# Customer clicks "Authenticate"
# Payment resolves based on scenario config
```

### Mobile money

```bash
# Status returns "send_otp"
# Get OTP from /control/otp-logs
# Submit OTP → status "pay_offline"
# Payment resolves after delay_ms
# Poll GET /api/v1/public/transaction/verify/:reference until success or failed
```

---

## Simulating Failures

Test your error handling without waiting for real failures:

```bash
# Get JWT
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "dev@myapp.com", "password": "secret123"}'

# Force insufficient funds on all charges
curl -X PUT http://localhost:8080/api/v1/control/scenario \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{"force_status": "failed", "error_code": "CHARGE_INSUFFICIENT_FUNDS"}'

# Run your test
# ...

# Reset when done
curl -X POST http://localhost:8080/api/v1/control/scenario/reset \
  -H "Authorization: Bearer <jwt>"
```

See [Scenario Testing](./scenario-testing.md) for all available error codes.

---

## Setting Up Webhooks Locally

Your webhook handler needs to be reachable by Payfake. Use a tunnel:

```bash
# ngrok
ngrok http 3000
# → https://abc123.ngrok.io

# cloudflared
cloudflared tunnel --url http://localhost:3000
# → https://abc123.trycloudflare.com
```

Set the tunnel URL as your webhook URL:

```bash
curl -X POST http://localhost:8080/api/v1/merchant/webhook \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{"webhook_url": "https://abc123.ngrok.io/webhook/paystack"}'
```

Test it works:

```bash
curl -X POST http://localhost:8080/api/v1/merchant/webhook/test \
  -H "Authorization: Bearer <jwt>"
```

---

## Switching Between Payfake and Real Paystack

No code changes needed. Just swap the environment variable:

```bash
# Development — Payfake
PAYSTACK_BASE_URL=http://localhost:8080
PAYSTACK_SECRET_KEY=sk_test_xxx  # your Payfake key

# Production — Real Paystack
PAYSTACK_BASE_URL=https://api.paystack.co
PAYSTACK_SECRET_KEY=sk_live_xxx  # your real Paystack key
```

The only difference is the base URL and the key. Everything else —
your initialization code, your webhook handler, your verify call —
is identical between environments.

---

## Common Gotchas

**Authorization URL points to localhost**
The `authorization_url` returned by `/transaction/initialize` uses the
`FRONTEND_URL` from your `.env`. Make sure it's set correctly:

```bash
FRONTEND_URL=http://localhost:5173  # your React checkout app
```

**Webhook signature mismatch**
Make sure you're using the Payfake secret key (not a real Paystack key)
when verifying webhook signatures. The key Payfake uses to sign is the
`sk_test_xxx` key you got from `/auth/keys`.

**OTP not arriving**
OTPs are never sent via SMS in Payfake. Read them from the API:

```bash
curl "http://localhost:8080/api/v1/control/otp-logs?reference=TXN_xxx" \
  -H "Authorization: Bearer <jwt>"
```

**MoMo stuck on pay_offline**
The resolution delay is controlled by `delay_ms` in your scenario config.
Default is 0 — resolves instantly. If it seems stuck, check your scenario:

```bash
curl http://localhost:8080/api/v1/control/scenario \
  -H "Authorization: Bearer <jwt>"
```
