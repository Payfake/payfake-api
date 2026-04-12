# Scenario Testing

The scenario engine is Payfake's core feature — it lets you control exactly
what happens to every charge without changing your application code.

---

## The Scenario Config

Every merchant has a scenario config with four fields:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `failure_rate` | float | `0.0` | Probability of failure. `0.0` = never, `1.0` = always |
| `delay_ms` | integer | `0` | Artificial delay before resolving. Max 30000 |
| `force_status` | string | `""` | Override all charges to this exact status |
| `error_code` | string | `""` | Error code when `force_status` is `failed` |

---

## Resolution Priority

The simulator resolves every charge in this order:

```
1. force_status set?  → always return that status. ignore everything else.
2. failure_rate roll? → random roll. if roll < failure_rate → fail.
3. default            → succeed.
```

`delay_ms` is always applied regardless of which path is taken.

---

## Common Test Scenarios

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

Statistically 3 in 10 charges will fail. Useful for testing your
retry logic and failure UI under realistic conditions.

### Simulate slow network (2 second delay)

```bash
curl -X PUT http://localhost:8080/api/v1/control/scenario \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{"delay_ms": 2000}'
```

Tests your loading states, timeout handling, and user experience
under slow network conditions.

### Force all charges to fail with insufficient funds

```bash
curl -X PUT http://localhost:8080/api/v1/control/scenario \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "force_status": "failed",
    "error_code": "CHARGE_INSUFFICIENT_FUNDS"
  }'
```

### Simulate MoMo provider outage

```bash
curl -X PUT http://localhost:8080/api/v1/control/scenario \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "force_status": "failed",
    "error_code": "CHARGE_MOMO_PROVIDER_UNAVAILABLE"
  }'
```

### Force a specific transaction (bypass scenario engine)

```bash
curl -X POST http://localhost:8080/api/v1/control/transactions/TXN_xxx/force \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "status": "failed",
    "error_code": "CHARGE_DO_NOT_HONOR"
  }'
```

Use this when you need one specific transaction to have a specific outcome
without affecting all other charges.

---

## Available Error Codes

### Card
| Code | Real-world meaning |
|------|--------------------|
| `CHARGE_INVALID_CARD` | Bad card number |
| `CHARGE_CARD_EXPIRED` | Card past expiry date |
| `CHARGE_INVALID_CVV` | Wrong security code |
| `CHARGE_INVALID_PIN` | Wrong PIN |
| `CHARGE_INSUFFICIENT_FUNDS` | Not enough money |
| `CHARGE_DO_NOT_HONOR` | Bank declined — most common in Ghana |
| `CHARGE_NOT_PERMITTED` | Online payments disabled on card |
| `CHARGE_LIMIT_EXCEEDED` | Daily limit hit |
| `CHARGE_NETWORK_ERROR` | Timeout between processor and bank |

### Mobile Money
| Code | Real-world meaning |
|------|--------------------|
| `CHARGE_MOMO_TIMEOUT` | Customer ignored the prompt |
| `CHARGE_MOMO_INVALID_NUMBER` | Number not on this network |
| `CHARGE_MOMO_LIMIT_EXCEEDED` | Wallet limit reached |
| `CHARGE_MOMO_PROVIDER_UNAVAILABLE` | MTN/Vodafone/AirtelTigo system down |

### Bank Transfer
| Code | Real-world meaning |
|------|--------------------|
| `CHARGE_BANK_INVALID_ACCOUNT` | Account doesn't exist |
| `CHARGE_BANK_TRANSFER_FAILED` | Bank rejected the transfer |

---

## Reset

Always reset your scenario config after a test run so you don't accidentally
leave failure modes active:

```bash
curl -X POST http://localhost:8080/api/v1/control/scenario/reset \
  -H "Authorization: Bearer <jwt>"
```
