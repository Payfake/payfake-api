# SDK Guides

Payfake has official SDKs for Go, Python, JavaScript and Rust.
All four cover the full API surface including the multi-step charge flows.

---

## Installation

### Go
```bash
go get github.com/payfake/payfake-go
```

### Python
```bash
pip install payfake
```

### JavaScript
```bash
npm install payfake-js
```

### Rust
```toml
[dependencies]
payfake = { git = "https://github.com/payfake/payfake-rust" }
```

---

## Initialization

### Go
```go
client := payfake.New(payfake.Config{
    SecretKey: "sk_test_xxx",
    BaseURL:   "http://localhost:8080",
})
```

### Python
```python
from payfake import Client
client = Client(secret_key="sk_test_xxx", base_url="http://localhost:8080")
```

### JavaScript
```js
import { createClient } from "payfake-js"
const client = createClient({ secretKey: "sk_test_xxx", baseURL: "http://localhost:8080" })
```

### Rust
```rust
let client = Client::new(Config {
    secret_key: "sk_test_xxx".to_string(),
    base_url: Some("http://localhost:8080".to_string()),
    timeout_secs: None,
});
```

---

## Full Card Flow (Go example)

```go
ctx := context.Background()

// Initialize
tx, err := client.Transaction.Initialize(ctx, payfake.InitializeInput{
    Email:    "customer@example.com",
    Amount:   10000,
    Currency: "GHS",
})

// Charge, local Verve card
charge, err := client.Charge.Card(ctx, payfake.ChargeCardInput{
    AccessCode: tx.AccessCode,
    CardNumber: "5061000000000000",
    CardExpiry: "12/26",
    CVV:        "123",
    Email:      "customer@example.com",
})
// charge.Status == "send_pin"

// Submit PIN
step2, err := client.Charge.SubmitPIN(ctx, payfake.SubmitPINInput{
    Reference: tx.Reference,
    PIN:       "1234",
})
// step2.Status == "send_otp"
// Read OTP from /control/logs

// Submit OTP
step3, err := client.Charge.SubmitOTP(ctx, payfake.SubmitOTPInput{
    Reference: tx.Reference,
    OTP:       "482931",
})
// step3.Status == "success"

// Verify
verified, err := client.Transaction.Verify(ctx, tx.Reference)
fmt.Println(verified.Status) // "success"
```

---

## Error Handling

### Go
```go
if payfake.IsCode(err, payfake.CodeChargeFailed) {
    // handle charge failure
}
```

### Python
```python
except PayfakeError as e:
    if e.is_code(PayfakeError.CODE_CHARGE_FAILED):
        pass
```

### JavaScript
```js
} catch (err) {
    if (err instanceof PayfakeError && err.isCode(PayfakeError.CODE_CHARGE_FAILED)) {}
}
```

### Rust
```rust
Err(e) if e.is_code(codes::CHARGE_FAILED) => {}
```

---

## SDK Repositories

| Language | Repository |
|----------|-----------|
| Go | [payfake-go](https://github.com/payfake/payfake-go) |
| Python | [payfake-python](https://github.com/payfake/payfake-python) |
| JavaScript | [payfake-js](https://github.com/payfake/payfake-js) |
| Rust | [payfake-rust](https://github.com/payfake/payfake-rust) |
