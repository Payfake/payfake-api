package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// CheckoutPage serves the hosted payment page.
// This is what the customer's browser opens when the developer
// redirects them to the authorization_url from /transaction/initialize.
// It renders an HTML page with a payment form — card details, MoMo prompt
// or bank transfer — and submits to the public charge endpoints.
// The access_code in the URL is how we know which transaction is being paid.
func CheckoutPage() gin.HandlerFunc {
	return func(c *gin.Context) {
		accessCode := c.Param("access_code")
		if accessCode == "" {
			c.String(http.StatusBadRequest, "Invalid payment link")
			return
		}

		// Serve the checkout HTML with the access_code baked in.
		// The frontend JS reads it from the window object and sends it
		// with every charge request, no secret key ever leaves the server.
		html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Payfake Checkout</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
            background: #0a0a0a;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            color: #fff;
        }

        .checkout-card {
            background: #111;
            border: 1px solid #222;
            border-radius: 16px;
            padding: 40px;
            width: 100%%;
            max-width: 420px;
            box-shadow: 0 25px 60px rgba(0,0,0,0.5);
        }

        .logo {
            display: flex;
            align-items: center;
            gap: 10px;
            margin-bottom: 32px;
        }

        .logo-mark {
            width: 36px;
            height: 36px;
            background: linear-gradient(135deg, #00ff88, #00ccff);
            border-radius: 8px;
            display: flex;
            align-items: center;
            justify-content: center;
            font-weight: 900;
            font-size: 14px;
            color: #000;
        }

        .logo-text {
            font-size: 18px;
            font-weight: 700;
            color: #fff;
        }

        .logo-badge {
            font-size: 11px;
            background: #1a1a1a;
            border: 1px solid #333;
            color: #888;
            padding: 2px 8px;
            border-radius: 20px;
            margin-left: auto;
        }

        .amount-block {
            background: #0d0d0d;
            border: 1px solid #1e1e1e;
            border-radius: 12px;
            padding: 20px;
            margin-bottom: 28px;
            text-align: center;
        }

        .amount-label {
            font-size: 12px;
            color: #555;
            text-transform: uppercase;
            letter-spacing: 1px;
            margin-bottom: 6px;
        }

        .amount-value {
            font-size: 32px;
            font-weight: 700;
            color: #00ff88;
        }

        .tabs {
            display: flex;
            gap: 8px;
            margin-bottom: 24px;
            background: #0d0d0d;
            padding: 4px;
            border-radius: 10px;
            border: 1px solid #1e1e1e;
        }

        .tab {
            flex: 1;
            padding: 10px;
            border: none;
            background: transparent;
            color: #555;
            border-radius: 8px;
            cursor: pointer;
            font-size: 13px;
            font-weight: 500;
            transition: all 0.2s;
        }

        .tab.active {
            background: #1a1a1a;
            color: #fff;
            border: 1px solid #333;
        }

        .tab-panel { display: none; }
        .tab-panel.active { display: block; }

        .field {
            margin-bottom: 16px;
        }

        label {
            display: block;
            font-size: 12px;
            color: #666;
            margin-bottom: 6px;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }

        input, select {
            width: 100%%;
            padding: 14px 16px;
            background: #0d0d0d;
            border: 1px solid #222;
            border-radius: 10px;
            color: #fff;
            font-size: 15px;
            outline: none;
            transition: border-color 0.2s;
        }

        input:focus, select:focus {
            border-color: #00ff88;
        }

        input::placeholder { color: #333; }

        .card-row {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 12px;
        }

        .provider-grid {
            display: grid;
            grid-template-columns: repeat(3, 1fr);
            gap: 8px;
            margin-bottom: 16px;
        }

        .provider-btn {
            padding: 12px 8px;
            background: #0d0d0d;
            border: 1px solid #222;
            border-radius: 10px;
            color: #666;
            cursor: pointer;
            font-size: 12px;
            font-weight: 600;
            text-align: center;
            transition: all 0.2s;
        }

        .provider-btn.selected {
            border-color: #00ff88;
            color: #00ff88;
            background: rgba(0,255,136,0.05);
        }

        .pay-btn {
            width: 100%%;
            padding: 16px;
            background: linear-gradient(135deg, #00ff88, #00ccff);
            border: none;
            border-radius: 12px;
            color: #000;
            font-size: 16px;
            font-weight: 700;
            cursor: pointer;
            margin-top: 8px;
            transition: opacity 0.2s, transform 0.1s;
        }

        .pay-btn:hover { opacity: 0.9; }
        .pay-btn:active { transform: scale(0.99); }
        .pay-btn:disabled { opacity: 0.4; cursor: not-allowed; }

        .status-block {
            display: none;
            text-align: center;
            padding: 20px 0;
        }

        .status-icon {
            font-size: 48px;
            margin-bottom: 12px;
        }

        .status-title {
            font-size: 20px;
            font-weight: 700;
            margin-bottom: 6px;
        }

        .status-msg {
            font-size: 14px;
            color: #666;
        }

        .status-block.success .status-title { color: #00ff88; }
        .status-block.failed  .status-title { color: #ff4466; }
        .status-block.pending .status-title { color: #ffaa00; }

        .secure-note {
            text-align: center;
            font-size: 11px;
            color: #333;
            margin-top: 20px;
        }

        .spinner {
            display: inline-block;
            width: 16px;
            height: 16px;
            border: 2px solid #000;
            border-top-color: transparent;
            border-radius: 50%%;
            animation: spin 0.7s linear infinite;
            vertical-align: middle;
            margin-right: 6px;
        }

        @keyframes spin { to { transform: rotate(360deg); } }
    </style>
</head>
<body>

<div class="checkout-card">
    <div class="logo">
        <div class="logo-mark">PF</div>
        <span class="logo-text">Payfake</span>
        <span class="logo-badge">Simulator</span>
    </div>

    <div class="amount-block">
        <div class="amount-label">Amount Due</div>
        <div class="amount-value" id="amountDisplay">Loading...</div>
    </div>

    <div id="paymentForm">
        <div class="tabs">
            <button class="tab active" onclick="switchTab('card')">💳 Card</button>
            <button class="tab" onclick="switchTab('momo')">📱 MoMo</button>
            <button class="tab" onclick="switchTab('bank')">🏦 Bank</button>
        </div>

        <!-- Card Panel -->
        <div class="tab-panel active" id="panel-card">
            <div class="field">
                <label>Card Number</label>
                <input type="text" id="cardNumber" placeholder="4111 1111 1111 1111" maxlength="19"
                    oninput="formatCardNumber(this)">
            </div>
            <div class="card-row">
                <div class="field">
                    <label>Expiry</label>
                    <input type="text" id="cardExpiry" placeholder="MM/YY" maxlength="5"
                        oninput="formatExpiry(this)">
                </div>
                <div class="field">
                    <label>CVV</label>
                    <input type="text" id="cardCVV" placeholder="123" maxlength="4">
                </div>
            </div>
            <div class="field">
                <label>Email</label>
                <input type="email" id="cardEmail" placeholder="customer@example.com">
            </div>
            <button class="pay-btn" id="cardPayBtn" onclick="payWithCard()">Pay Now</button>
        </div>

        <!-- MoMo Panel -->
        <div class="tab-panel" id="panel-momo">
            <div class="field">
                <label>Network</label>
                <div class="provider-grid">
                    <div class="provider-btn selected" onclick="selectProvider(this, 'mtn')">MTN</div>
                    <div class="provider-btn" onclick="selectProvider(this, 'vodafone')">Vodafone</div>
                    <div class="provider-btn" onclick="selectProvider(this, 'airteltigo')">AirtelTigo</div>
                </div>
            </div>
            <div class="field">
                <label>Phone Number</label>
                <input type="tel" id="momoPhone" placeholder="+233241234567">
            </div>
            <div class="field">
                <label>Email</label>
                <input type="email" id="momoEmail" placeholder="customer@example.com">
            </div>
            <button class="pay-btn" id="momoPayBtn" onclick="payWithMomo()">Send Prompt</button>
        </div>

        <!-- Bank Panel -->
        <div class="tab-panel" id="panel-bank">
            <div class="field">
                <label>Bank Code</label>
                <select id="bankCode">
                    <option value="GCB">GCB Bank</option>
                    <option value="EBG">Ecobank Ghana</option>
                    <option value="SCB">Standard Chartered</option>
                    <option value="ABG">Access Bank Ghana</option>
                    <option value="CAL">CAL Bank</option>
                    <option value="FBL">FBNBank Ghana</option>
                </select>
            </div>
            <div class="field">
                <label>Account Number</label>
                <input type="text" id="bankAccount" placeholder="1234567890">
            </div>
            <div class="field">
                <label>Email</label>
                <input type="email" id="bankEmail" placeholder="customer@example.com">
            </div>
            <button class="pay-btn" id="bankPayBtn" onclick="payWithBank()">Pay Now</button>
        </div>
    </div>

    <!-- Status display — shown after charge attempt -->
    <div class="status-block" id="statusBlock">
        <div class="status-icon" id="statusIcon"></div>
        <div class="status-title" id="statusTitle"></div>
        <div class="status-msg" id="statusMsg"></div>
    </div>

    <div class="secure-note"> Simulated payment — no real money moves</div>
</div>

<script>
    // The access code is baked in server-side — the frontend never
    // sees or needs the secret key. The access code is single-use
    // and tied to this specific transaction.
    const ACCESS_CODE = "%s"
    const BASE_URL = window.location.origin
    let selectedProvider = "mtn"
    let txData = null

    // Fetch the transaction details to display the amount.
    // We call the public transaction info endpoint using the access code.
    async function loadTransaction() {
        try {
            const resp = await fetch(BASE_URL + "/api/v1/public/transaction/" + ACCESS_CODE)
            const data = await resp.json()
            if (data.status === "success") {
                txData = data.data
                const amount = (txData.amount / 100).toFixed(2)
                document.getElementById("amountDisplay").textContent =
                    txData.currency + " " + amount
            }
        } catch (e) {
            document.getElementById("amountDisplay").textContent = "---"
        }
    }

    function switchTab(tab) {
        document.querySelectorAll(".tab").forEach((t, i) => {
            t.classList.toggle("active", ["card","momo","bank"][i] === tab)
        })
        document.querySelectorAll(".tab-panel").forEach(p => p.classList.remove("active"))
        document.getElementById("panel-" + tab).classList.add("active")
    }

    function selectProvider(el, provider) {
        document.querySelectorAll(".provider-btn").forEach(b => b.classList.remove("selected"))
        el.classList.add("selected")
        selectedProvider = provider
    }

    function formatCardNumber(input) {
        let v = input.value.replace(/\D/g, "").substring(0, 16)
        input.value = v.replace(/(.{4})/g, "$1 ").trim()
    }

    function formatExpiry(input) {
        let v = input.value.replace(/\D/g, "").substring(0, 4)
        if (v.length >= 3) v = v.substring(0,2) + "/" + v.substring(2)
        input.value = v
    }

    function setLoading(btnId, loading) {
        const btn = document.getElementById(btnId)
        btn.disabled = loading
        btn.innerHTML = loading
            ? '<span class="spinner"></span> Processing...'
            : btn.dataset.label || btn.innerHTML
    }

    function showStatus(type, icon, title, msg) {
        document.getElementById("paymentForm").style.display = "none"
        const block = document.getElementById("statusBlock")
        block.className = "status-block " + type
        block.style.display = "block"
        document.getElementById("statusIcon").textContent = icon
        document.getElementById("statusTitle").textContent = title
        document.getElementById("statusMsg").textContent = msg
    }

    async function charge(endpoint, body) {
        const resp = await fetch(BASE_URL + "/api/v1/public/charge/" + endpoint, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            // access_code authenticates the request — no secret key needed.
            body: JSON.stringify({ ...body, access_code: ACCESS_CODE })
        })
        return resp.json()
    }

    async function payWithCard() {
        const number = document.getElementById("cardNumber").value.replace(/\s/g, "")
        const expiry = document.getElementById("cardExpiry").value
        const cvv    = document.getElementById("cardCVV").value
        const email  = document.getElementById("cardEmail").value

        if (!number || !expiry || !cvv || !email) {
            alert("Please fill in all card details")
            return
        }

        setLoading("cardPayBtn", true)
        try {
            const data = await charge("card", {
                card_number: number,
                card_expiry: expiry,
                cvv, email,
            })
            handleChargeResult(data)
        } catch (e) {
            showStatus("failed", "❌", "Something went wrong", "Please try again")
        } finally {
            setLoading("cardPayBtn", false)
        }
    }

    async function payWithMomo() {
        const phone = document.getElementById("momoPhone").value
        const email = document.getElementById("momoEmail").value

        if (!phone || !email) {
            alert("Please fill in all details")
            return
        }

        setLoading("momoPayBtn", true)
        try {
            const data = await charge("mobile_money", {
                phone, provider: selectedProvider, email
            })
            // MoMo always returns pending — tell the customer to check their phone.
            showStatus("pending", "📱", "Check Your Phone",
                "A payment prompt has been sent to " + phone + ". Approve it to complete payment.")
            // Poll for the final status since MoMo resolves asynchronously.
            pollStatus()
        } catch (e) {
            showStatus("failed", "❌", "Something went wrong", "Please try again")
            setLoading("momoPayBtn", false)
        }
    }

    async function payWithBank() {
        const bankCode = document.getElementById("bankCode").value
        const account  = document.getElementById("bankAccount").value
        const email    = document.getElementById("bankEmail").value

        if (!account || !email) {
            alert("Please fill in all details")
            return
        }

        setLoading("bankPayBtn", true)
        try {
            const data = await charge("bank", {
                bank_code: bankCode, account_number: account, email
            })
            handleChargeResult(data)
        } catch (e) {
            showStatus("failed", "❌", "Something went wrong", "Please try again")
        } finally {
            setLoading("bankPayBtn", false)
        }
    }

    function handleChargeResult(data) {
        if (data.status === "success" && data.data?.transaction?.status === "success") {
            showStatus("success", "✅", "Payment Successful",
                "Your payment was processed. You will be redirected shortly.")
            // Redirect to the callback_url after 2 seconds.
            // The developer's backend verifies via GET /transaction/verify/:reference.
            if (txData?.callback_url) {
                setTimeout(() => {
                    window.location.href = txData.callback_url +
                        "?reference=" + txData.reference +
                        "&status=success"
                }, 2000)
            }
        } else {
            const errMsg = data.errors?.[0]?.message || data.message || "Payment declined"
            showStatus("failed", "❌", "Payment Failed", errMsg)
        }
    }

    // Poll transaction status every 3 seconds for MoMo.
    // Stops after 10 attempts (30 seconds) — if not resolved by then
    // tell the customer to wait for a confirmation message.
    async function pollStatus(attempts = 0) {
        if (attempts >= 10) {
            showStatus("pending", "⏳", "Still Processing",
                "Your payment is being processed. Check your email for confirmation.")
            return
        }
        setTimeout(async () => {
            try {
                const resp = await fetch(
                    BASE_URL + "/api/v1/public/transaction/" + ACCESS_CODE
                )
                const data = await resp.json()
                const status = data.data?.status
                if (status === "success") {
                    showStatus("success", "✅", "Payment Successful",
                        "Your MoMo payment was approved.")
                    if (txData?.callback_url) {
                        setTimeout(() => {
                            window.location.href = txData.callback_url +
                                "?reference=" + data.data.reference +
                                "&status=success"
                        }, 2000)
                    }
                } else if (status === "failed") {
                    showStatus("failed", "❌", "Payment Failed",
                        "Your MoMo payment was declined.")
                } else {
                    pollStatus(attempts + 1)
                }
            } catch (e) {
                pollStatus(attempts + 1)
            }
        }, 3000)
    }

    loadTransaction()
</script>
</body>
</html>`, accessCode)

		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, html)
	}
}
