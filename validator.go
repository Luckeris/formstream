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

	lastAt := strings.LastIndex(addr.Address, "@")
	if lastAt <= 0 || lastAt >= len(addr.Address)-1 {
		return false
	}

	localPart := addr.Address[:lastAt]
	domain := addr.Address[lastAt+1:]

	if len(localPart) == 0 || len(localPart) > 64 {
		return false
	}

	// Unquoted local parts must not have leading/trailing dots or consecutive dots
	if !strings.HasPrefix(localPart, "\"") || !strings.HasSuffix(localPart, "\"") {
		if strings.HasPrefix(localPart, ".") || strings.HasSuffix(localPart, ".") || strings.Contains(localPart, "..") {
			return false
		}
	}

	domainParts := strings.Split(domain, ".")
	if len(domainParts) < 2 {
		return false
	}

	// Validate each domain label according to DNS specifications (RFC 1035 / RFC 1123 / RFC 5890)
	for _, part := range domainParts {
		if len(part) == 0 || len(part) > 63 || strings.HasPrefix(part, "-") || strings.HasSuffix(part, "-") {
			return false
		}
		for _, r := range part {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r > 127) {
				return false
			}
		}
	}

	// Top-level domain must be at least 2 characters, cannot be all digits,
	// and if not punycode (xn--...), must consist purely of letters or international characters
	tld := domainParts[len(domainParts)-1]
	tldRunes := []rune(tld)
	if len(tldRunes) < 2 {
		return false
	}
	if strings.HasPrefix(tld, "xn--") {
		if len(tld) < 5 {
			return false
		}
	} else {
		for _, r := range tldRunes {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r > 127) {
				return false
			}
		}
	}

	return true
}
