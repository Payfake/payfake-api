# SDK Guides

Payfake has official SDKs for Go, Python, JavaScript and Rust.
All four SDKs cover the full API surface and follow language-idiomatic
conventions.

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
import payfake "github.com/payfake/payfake-go"

client := payfake.New(payfake.Config{
    SecretKey: "sk_test_xxx",
    BaseURL:   "http://localhost:8080",
})
```

### Python
```python
from payfake import Client

client = Client(
    secret_key="sk_test_xxx",
    base_url="http://localhost:8080",
)
```

### JavaScript
```js
import { createClient } from "payfake-js"

const client = createClient({
  secretKey: "sk_test_xxx",
  baseURL: "http://localhost:8080",
})
```

### Rust
```rust
use payfake::{Client, Config};

let client = Client::new(Config {
    secret_key: "sk_test_xxx".to_string(),
    base_url: Some("http://localhost:8080".to_string()),
    timeout_secs: None,
});
```

---

## Error Handling

All SDKs use a typed error type. Switch on the error code
for programmatic handling, never parse the message string.

### Go
```go
_, err := client.Transaction.Initialize(ctx, input)
if err != nil {
    if payfake.IsCode(err, payfake.CodeReferenceTaken) {
        // handle duplicate reference
    }
}
```

### Python
```python
from payfake.errors import PayfakeError

try:
    client.transaction.initialize(input)
except PayfakeError as e:
    if e.is_code(PayfakeError.CODE_REFERENCE_TAKEN):
        # handle duplicate reference
        pass
```

### JavaScript
```js
try {
  await client.transaction.initialize(input)
} catch (err) {
  if (err instanceof PayfakeError && err.isCode(PayfakeError.CODE_REFERENCE_TAKEN)) {
    // handle duplicate reference
  }
}
```

### Rust
```rust
match client.transaction.initialize(input).await {
    Err(e) if e.is_code(codes::REFERENCE_TAKEN) => {
        // handle duplicate reference
    }
    Err(e) => return Err(e.into()),
    Ok(tx) => { /* ... */ }
}
```

---

## SDK Repositories

| Language | Repository | Package Registry |
|----------|-----------|-----------------|
| Go | [payfake-go](https://github.com/payfake/payfake-go) | pkg.go.dev |
| Python | [payfake-python](https://github.com/payfake/payfake-python) | PyPI |
| JavaScript | [payfake-js](https://github.com/payfake/payfake-js) | npm |
| Rust | [payfake-rust](https://github.com/payfake/payfake-rust) | crates.io |
