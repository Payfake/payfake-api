# Webhooks

Payfake fires webhooks to your configured `webhook_url` after every terminal
transaction state change — identical to how Paystack webhooks work.

---

## Setting Your Webhook URL

Update your merchant's webhook URL via the dashboard or directly in the DB.
A dashboard endpoint for this will be added in a future release — for now
update it in the `merchants` table:

```sql
UPDATE merchants SET webhook_url = 'https://yourapp.com/webhook' WHERE id = 'MRC_xxx';
```

---

## Events

| Event | Fired when |
|-------|-----------|
| `charge.success` | Card, MoMo or bank charge completes successfully |
| `charge.failed` | Charge fails for any reason |
| `transfer.success` | Transfer completes |
| `transfer.failed` | Transfer fails |
| `refund.processed` | Refund processed on a transaction |

---

## Payload Shape

```json
{
  "event": "charge.success",
  "data": {
    "id": "TXN_A1B2C3D4E5F6",
    "reference": "your-reference-001",
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
The signature is in the `X-Paystack-Signature` header identical to
Paystack's scheme so your existing verification code works unchanged.

**Always verify signatures before processing webhooks.**
An unverified webhook could be spoofed by anyone who knows your endpoint URL.

### Go

```go
func handleWebhook(w http.ResponseWriter, r *http.Request) {
    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "bad request", 400)
        return
    }

    signature := r.Header.Get("X-Paystack-Signature")
    if !verifySignature(body, signature, os.Getenv("PAYFAKE_SECRET_KEY")) {
        http.Error(w, "invalid signature", 401)
        return
    }

    // safe to process
    w.WriteHeader(200)
}

func verifySignature(body []byte, signature, secretKey string) bool {
    mac := hmac.New(sha512.New, []byte(secretKey))
    mac.Write(body)
    expected := hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(expected), []byte(signature))
}
```

### Python (Django example)

```python
import hashlib
import hmac
from django.http import HttpResponse

def webhook(request):
    body = request.body
    signature = request.headers.get("X-Paystack-Signature", "")
    secret_key = os.environ["PAYFAKE_SECRET_KEY"]

    expected = hmac.new(
        secret_key.encode(),
        body,
        hashlib.sha512
    ).hexdigest()

    if not hmac.compare_digest(expected, signature):
        return HttpResponse(status=401)

    # safe to process
    import json
    event = json.loads(body)

    if event["event"] == "charge.success":
        reference = event["data"]["reference"]
        # update your order status

    return HttpResponse(status=200)
```

### JavaScript (Express example)

```js
import { createHmac } from "crypto"
import express from "express"

const app = express()

app.post("/webhook", express.raw({ type: "application/json" }), (req, res) => {
  const signature = req.headers["x-paystack-signature"]
  const secretKey = process.env.PAYFAKE_SECRET_KEY

  const expected = createHmac("sha512", secretKey)
    .update(req.body)
    .digest("hex")

  if (expected !== signature) {
    return res.status(401).send("Invalid signature")
  }

  const event = JSON.parse(req.body)

  if (event.event === "charge.success") {
    const { reference } = event.data
    // update your order status
  }

  res.status(200).send("ok")
})
```

**Important for Express:** Use `express.raw()` not `express.json()` for the
webhook route. You must verify the signature against the raw bytes, parsing
to JSON first changes the byte representation and breaks the signature check.

---

## Retry Logic

Payfake attempts delivery up to 3 times for failed webhooks.
If your endpoint returns a non-2xx response or times out (10 second timeout)
the attempt is recorded as failed and retried.

View delivery attempts:

```bash
curl http://localhost:8080/api/v1/control/webhooks/<webhook_id>/attempts \
  -H "Authorization: Bearer <jwt>"
```

Manually retry a failed webhook:

```bash
curl -X POST http://localhost:8080/api/v1/control/webhooks/<webhook_id>/retry \
  -H "Authorization: Bearer <jwt>"
```

---

## Testing Webhooks Locally

Use a tunneling tool to expose your local webhook endpoint:

```bash
# ngrok
ngrok http 3000

# cloudflared
cloudflared tunnel --url http://localhost:3000
```

Then update your merchant's `webhook_url` to the tunnel URL.

---

## MoMo Webhooks

Mobile money charges are asyncm the charge endpoint returns `pending`
immediately and the outcome arrives via webhook after the simulated delay.

Your backend must handle webhooks for MoMo to work correctly:

```
POST /charge/mobile_money → { status: "pending" }   ← don't trust this alone
                                    │
                          delay_ms passes
                                    │
                          POST your_webhook_url
                          { event: "charge.success" or "charge.failed" }
                                    │
                          now update your order
```

Never mark an order as paid based on the `pending` response.
Always wait for the `charge.success` webhook.
