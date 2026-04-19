# Webhooks

Payfake fires webhooks to your configured URL after every terminal
transaction state change — identical to Paystack webhooks.

---

## Setting Your Webhook URL

Via the dashboard API:

```bash
curl -X POST http://localhost:8080/api/v1/merchant/webhook \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{"webhook_url": "https://yourapp.com/webhook"}'
```

Test your endpoint works:

```bash
curl -X POST http://localhost:8080/api/v1/merchant/webhook/test \
  -H "Authorization: Bearer <jwt>"
```

Response:
```json
{
  "data": {
    "webhook_url": "https://yourapp.com/webhook",
    "success": true,
    "status_code": 200,
    "payload": { "event": "charge.success", "data": { ... } }
  }
}
```

---

## Events

| Event | Fired when |
|-------|-----------|
| `charge.success` | Charge completes successfully |
| `charge.failed` | Charge fails at any step |
| `transfer.success` | Transfer completes |
| `transfer.failed` | Transfer fails |
| `refund.processed` | Refund processed |

---

## Payload Shape

```json
{
  "event": "charge.success",
  "data": {
    "id": "TXN_xxxxxxxxxxxx",
    "reference": "your-ref-001",
    "amount": 10000,
    "currency": "GHS",
    "status": "success",
    "channel": "card",
    "fees": 150,
    "paid_at": "2026-04-12T00:00:00Z",
    "created_at": "2026-04-12T00:00:00Z",
    "customer": {
      "email": "customer@example.com",
      "first_name": "Kofi",
      "last_name": "Mensah"
    },
    "metadata": {}
  }
}
```

---

## Verifying Signatures

Payfake signs every webhook with your secret key using HMAC-SHA512.
The signature is in `X-Paystack-Signature`, identical to Paystack.

**Always verify before processing.**

### Go
```go
func verifyWebhook(body []byte, signature, secretKey string) bool {
    mac := hmac.New(sha512.New, []byte(secretKey))
    mac.Write(body)
    expected := hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(expected), []byte(signature))
}
```

### Python
```python
import hashlib, hmac

def verify_webhook(body: bytes, signature: str, secret_key: str) -> bool:
    expected = hmac.new(secret_key.encode(), body, hashlib.sha512).hexdigest()
    return hmac.compare_digest(expected, signature)
```

### JavaScript
```js
import { createHmac } from "crypto"

function verifyWebhook(body, signature, secretKey) {
    const expected = createHmac("sha512", secretKey)
        .update(body).digest("hex")
    return expected === signature
}
```

**Important for Express:** Use `express.raw()` not `express.json()` for the
webhook route — verify signature against raw bytes, not parsed JSON.

---

## Retry Logic

Payfake retries failed webhooks up to 3 times. View attempts:

```bash
curl http://localhost:8080/api/v1/control/webhooks/<id>/attempts \
  -H "Authorization: Bearer <jwt>"
```

Manually retry:

```bash
curl -X POST http://localhost:8080/api/v1/control/webhooks/<id>/retry \
  -H "Authorization: Bearer <jwt>"
```

---

## MoMo Webhooks

MoMo charges return `pay_offline` immediately — the final outcome
arrives via webhook after the customer approves the USSD prompt.

```
POST /charge/mobile_money → send_otp
POST /charge/submit_otp   → pay_offline   ← don't trust this as success
                                 │
                          delay_ms passes
                                 │
                    POST your_webhook_url
                    { event: "charge.success" }  ← now update your order
```

Never mark an order as paid based on `pay_offline`. Always wait for the webhook.

---

## Testing Locally

Use a tunneling tool:

```bash
# ngrok
ngrok http 3000

# cloudflared
cloudflared tunnel --url http://localhost:3000
```

Set the tunnel URL as your webhook URL.
