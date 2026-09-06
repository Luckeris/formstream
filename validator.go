package main

import (
	"net/mail"
	"strings"
)

// isValidEmail checks whether an email string complies with RFC 5322/6532,
// satisfies RFC 5321 length bounds, and contains a syntactically valid domain name.
func isValidEmail(email string) bool {
	email = strings.TrimSpace(email)
	if email == "" || len(email) > 254 {
		return false
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return false
	}
	parts := strings.Split(addr.Address, "@")
	if len(parts) != 2 {
		return false
	}

	localPart := parts[0]
	if len(localPart) == 0 || len(localPart) > 64 {
		return false
	}

	domain := parts[1]
	domainParts := strings.Split(domain, ".")
	if len(domainParts) < 2 {
		return false
	}

	// Validate each domain label according to DNS specifications (RFC 1035 / RFC 1123)
	for _, part := range domainParts {
		if len(part) == 0 || len(part) > 63 || strings.HasPrefix(part, "-") || strings.HasSuffix(part, "-") {
			return false
		}
	}

	// Top-level domain must be at least 2 characters and cannot be all digits
	tld := domainParts[len(domainParts)-1]
	if len([]rune(tld)) < 2 {
		return false
	}
	allDigits := true
	for _, r := range tld {
		if r < '0' || r > '9' {
			allDigits = false
			break
		}
	}
	if allDigits {
		return false
	}

	return true
}
