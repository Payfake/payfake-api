# API Reference

Base URL: `http://localhost:8080`

All responses follow the envelope:
```json
{ "status": "success|error", "message": "...", "data": {}, "metadata": {}, "code": "..." }
```

---

## Auth `/api/v1/auth`

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/register` | None | Create merchant account |
| POST | `/login` | None | Login and get JWT |
| POST | `/logout` | JWT | Logout |
| GET | `/keys` | JWT | Get current API keys |
| POST | `/keys/regenerate` | JWT | Regenerate key pair |

---

## Transaction `/api/v1/transaction`

Auth: `Bearer sk_test_xxx`

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/initialize` | Create pending transaction |
| GET | `/verify/:reference` | Verify by reference |
| GET | `/` | List transactions |
| GET | `/:id` | Fetch by ID |
| POST | `/:id/refund` | Refund a transaction |

---

## Charge `/api/v1/charge`

Auth: `Bearer sk_test_xxx`

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/card` | Charge a card |
| POST | `/mobile_money` | Initiate MoMo charge |
| POST | `/bank` | Initiate bank transfer |
| GET | `/:reference` | Fetch charge by reference |

---

## Customer `/api/v1/customer`

Auth: `Bearer sk_test_xxx`

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/` | Create customer |
| GET | `/` | List customers |
| GET | `/:code` | Fetch by code |
| PUT | `/:code` | Update customer |
| GET | `/:code/transactions` | Customer transactions |

---

## Control `/api/v1/control`

Auth: `Bearer <jwt>`

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/scenario` | Get scenario config |
| PUT | `/scenario` | Update scenario config |
| POST | `/scenario/reset` | Reset to defaults |
| GET | `/webhooks` | List webhook events |
| POST | `/webhooks/:id/retry` | Retry webhook delivery |
| GET | `/webhooks/:id/attempts` | Delivery attempt log |
| POST | `/transactions/:ref/force` | Force transaction outcome |
| GET | `/logs` | Request/response logs |
| DELETE | `/logs` | Clear logs |

---

## Public Checkout `/api/v1/public`

Auth: None (access_code in body)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/transaction/:access_code` | Load transaction for checkout |
| POST | `/charge/card` | Browser-safe card charge |
| POST | `/charge/mobile_money` | Browser-safe MoMo charge |
| POST | `/charge/bank` | Browser-safe bank charge |
