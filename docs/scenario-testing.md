# Scenario Testing

The scenario engine controls what happens to every charge without
changing your application code.

---

## The Scenario Config

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `failure_rate` | float | `0.0` | Probability of failure. `0.0` = never, `1.0` = always |
| `delay_ms` | integer | `0` | Artificial delay before resolving. Max 30000 |
| `force_status` | string | `""` | Override all charges to this exact status |
| `error_code` | string | `""` | Error code returned when `force_status` is `failed` |

---

## Resolution Priority

```
1. force_status set? → always return that status. ignore everything else.
2. failure_rate roll? → random. if roll < failure_rate → fail.
3. default           → succeed.
```

`delay_ms` always applies regardless of outcome.

---

## Common Scenarios

### Everything succeeds (default)
```bash
curl -X POST http://localhost:8080/api/v1/control/scenario/reset \
  -H "Authorization: Bearer <jwt>"
```

### 30% random failure rate
```bash
curl -X PUT http://localhost:8080/api/v1/control/scenario \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{"failure_rate": 0.3}'
```

### Simulate slow network (2 second delay)
```bash
curl -X PUT http://localhost:8080/api/v1/control/scenario \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{"delay_ms": 2000}'
```

### Force PIN failure
```bash
curl -X PUT http://localhost:8080/api/v1/control/scenario \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{"force_status": "failed", "error_code": "CHARGE_INVALID_PIN"}'
```

### Force OTP failure
```bash
curl -X PUT http://localhost:8080/api/v1/control/scenario \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{"force_status": "failed", "error_code": "CHARGE_INVALID_OTP"}'
```

### Force insufficient funds
```bash
curl -X PUT http://localhost:8080/api/v1/control/scenario \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{"force_status": "failed", "error_code": "CHARGE_INSUFFICIENT_FUNDS"}'
```

### Simulate MoMo timeout (customer ignores prompt)
```bash
curl -X PUT http://localhost:8080/api/v1/control/scenario \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{"force_status": "failed", "error_code": "CHARGE_MOMO_TIMEOUT"}'
```

### Simulate slow MoMo approval (5 second wait)
```bash
curl -X PUT http://localhost:8080/api/v1/control/scenario \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{"delay_ms": 5000}'
```

### Simulate MoMo provider outage
```bash
curl -X PUT http://localhost:8080/api/v1/control/scenario \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{"force_status": "failed", "error_code": "CHARGE_MOMO_PROVIDER_UNAVAILABLE"}'
```

### Force a specific transaction (bypass scenario engine)
```bash
curl -X POST http://localhost:8080/api/v1/control/transactions/TXN_xxx/force \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{"status": "failed", "error_code": "CHARGE_DO_NOT_HONOR"}'
```

---

## Error Codes

### Card
| Code | Real-world meaning |
|------|--------------------|
| `CHARGE_INVALID_CARD` | Bad card number |
| `CHARGE_CARD_EXPIRED` | Card past expiry |
| `CHARGE_INVALID_CVV` | Wrong CVV |
| `CHARGE_INVALID_PIN` | Wrong PIN entered |
| `CHARGE_INSUFFICIENT_FUNDS` | Not enough money |
| `CHARGE_DO_NOT_HONOR` | Bank declined — most common in Ghana |
| `CHARGE_NOT_PERMITTED` | Online payments disabled on card |
| `CHARGE_LIMIT_EXCEEDED` | Daily limit exceeded |
| `CHARGE_NETWORK_ERROR` | Simulated timeout |
| `CHARGE_INVALID_OTP` | Wrong OTP entered |

### Mobile Money
| Code | Real-world meaning |
|------|--------------------|
| `CHARGE_MOMO_TIMEOUT` | Customer ignored prompt |
| `CHARGE_MOMO_INVALID_NUMBER` | Number not on network |
| `CHARGE_MOMO_LIMIT_EXCEEDED` | Wallet limit reached |
| `CHARGE_MOMO_PROVIDER_UNAVAILABLE` | Network down |

### Bank
| Code | Real-world meaning |
|------|--------------------|
| `CHARGE_BANK_INVALID_ACCOUNT` | Account doesn't exist |
| `CHARGE_BANK_TRANSFER_FAILED` | Bank rejected transfer |

---

## Reset

Always reset after a test run:

```bash
curl -X POST http://localhost:8080/api/v1/control/scenario/reset \
  -H "Authorization: Bearer <jwt>"
```
