package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// maxDiscordContentLength defines the maximum character length for a Discord message content (2000 runes).
const maxDiscordContentLength = 2000

// discordHTTPClient is a dedicated HTTP client for sending notifications to Discord with a bounded 5-second timeout.
var discordHTTPClient = &http.Client{
	Timeout: 5 * time.Second,
}

// sendToDiscord parses the data from FormSubmission into the Content of the Discord Message,
// converts into JsonBytes and sends it via HTTP POST to the Discord webhookURL using a dedicated
// HTTP client with a bounded timeout and proper response body closure.
func sendToDiscord(webhookURL string, data FormSubmission) error {
	webhookURL = strings.TrimSpace(webhookURL)
	if !strings.HasPrefix(webhookURL, "http://") && !strings.HasPrefix(webhookURL, "https://") {
		return fmt.Errorf("invalid discord webhook URL: must start with http:// or https://")
	}

	var msg DiscordMessage
	msg.Content = fmt.Sprintf("Name: %s , Message: %s , Email: %s", data.Name, data.Message, data.Email)

	// Discord enforces a 2000-character limit on message content. Truncate safely if needed.
	contentRunes := []rune(msg.Content)
	if len(contentRunes) > maxDiscordContentLength {
		truncSuffix := "... [truncated]"
		msg.Content = string(contentRunes[:maxDiscordContentLength-len([]rune(truncSuffix))]) + truncSuffix
	}

	jsonBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal discord message: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return fmt.Errorf("failed to create discord request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := discordHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to dispatch discord webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Read error body snippet if status is non-success for descriptive error reporting
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		_, _ = io.Copy(io.Discard, resp.Body)
		bodySnippet := strings.TrimSpace(string(bodyBytes))
		if bodySnippet != "" {
			return fmt.Errorf("discord returned non-success status code %d: %s", resp.StatusCode, bodySnippet)
		}
		return fmt.Errorf("discord returned non-success status code: %d", resp.StatusCode)
	}

	// Drain remaining response body on success to ensure underlying TCP connection is reusable
	_, _ = io.Copy(io.Discard, resp.Body)

	return nil
}
