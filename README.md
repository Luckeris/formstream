# FormStream ?

> **Lightweight, zero-dependency Go microservice for handling contact form submissions and streaming instant notifications to Discord.**

[![License: MIT](https://img.shields.io/badge/License-MIT-gold.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.20%2B-00ADD8.svg)](https://go.dev/)

---

## ?? About FormStream

**FormStream** is a fast, self-hosted microservice written in Go that receives contact form submissions from static websites (React, Astro, Next.js, HTML) and:

1. Validates form input fields (email, name, message).
2. Forwards instant alert notifications to your **Discord** channel via Webhooks.
3. Backs up submission history into a local submissions.json data store.

---

## ? Quick Start

### 1. Run the Server

`bash
go run main.go
``n

### 2. Test Form Submission

`bash
curl -X POST http://localhost:8080/submit \
 -H Content-Type: application/json \
 -d '{name:Jan Nov�k,email:jan@example.com,message:Hello from FormStream!}'
``n

---

## ?? License

This project is licensed under the **MIT License** � see the [LICENSE](LICENSE) file for details.
