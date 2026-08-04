<div align="center">
  <img src="assets/logo.svg" alt="FormStream" width="220">
# FormStream ⚡
</div>
> **Lightweight, zero-dependency Go microservice for handling contact form submissions and streaming instant notifications to Discord via Webhooks.**

[![License: MIT](https://img.shields.io/badge/License-MIT-gold.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.20%2B-00ADD8.svg)](https://go.dev/)

---

## About

**FormStream** is a fast, self-hosted microservice written in Go that receives contact form submissions from frontend websites (React, Astro, Next.js, HTML) and:

1. Validates input fields (name, email format, non-empty message).
2. Forwards instant alert notifications to your **Discord** channel via Webhooks.
3. Backs up submission history into a local `submissions.json` data store.

---

## Environment Setup

FormStream uses the `DISCORD_WEBHOOK_URL` environment variable for Discord notifications. If not set, Discord notifications are cleanly skipped.

### Windows (PowerShell)

```powershell
$env:DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/YOUR_WEBHOOK_ID/YOUR_WEBHOOK_TOKEN"
go run main.go
```

### Linux / macOS

```bash
export DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/YOUR_WEBHOOK_ID/YOUR_WEBHOOK_TOKEN"
go run main.go
```

---

## Testing

### PowerShell

```powershell
Invoke-RestMethod -Uri "http://localhost:8080/submit" -Method Post -ContentType "application/json" -Body '{"name":"Jan Novák","email":"jan@example.com","message":"Hello from FormStream!"}'
```

### cURL

```bash
curl -X POST http://localhost:8080/submit \
  -H "Content-Type: application/json" \
  -d '{"name":"Jan Novák","email":"jan@example.com","message":"Hello from FormStream!"}'
```

---

## Frontend Integration (JavaScript `fetch`)

Connect any HTML contact form or React/Next.js frontend directly to FormStream:

```javascript
async function sendForm(name, email, message) {
  const response = await fetch("http://localhost:8080/submit", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ name, email, message }),
  });

  if (response.ok) {
    console.log("Form submitted successfully!");
  } else {
    const errorText = await response.text();
    console.error("Submission failed:", errorText);
  }
}
```

---

## License

This project is licensed under the **MIT License** — see the [LICENSE](LICENSE) file for details.
