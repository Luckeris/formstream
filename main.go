package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"
)

// maxRequestBodySize defines the maximum allowed size for request bodies (1MB).
const maxRequestBodySize = 1024 * 1024

// discordHTTPClient is a dedicated HTTP client for sending notifications to Discord with a bounded 5-second timeout.
var discordHTTPClient = &http.Client{
	Timeout: 5 * time.Second,
}

// Structure of FormSubmission Message
type FormSubmission struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Message string `json:"message"`
}

// Structure of a Discord Message
type DiscordMessage struct {
	Content string `json:"content"`
}

// APIResponse represents the standard structured JSON response for API endpoints.
type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

// Helper function to send structured JSON error responses with appropriate status code and headers.
func sendJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(APIResponse{
		Success: false,
		Message: message,
		Error:   message,
	})
}

// Helper function to send structured JSON success responses.
func sendJSONSuccess(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(APIResponse{
		Success: true,
		Message: message,
	})
}

// isValidEmail checks whether an email string complies with RFC 5322 and contains a valid domain.
func isValidEmail(email string) bool {
	email = strings.TrimSpace(email)
	if email == "" {
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
	domain := parts[1]
	if !strings.Contains(domain, ".") || strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") || strings.Contains(domain, "..") {
		return false
	}
	return true
}

// Function that parses the data from FormSubmission into the Content of the Discord Message,
// converts into JsonBytes and sends it via HTTP POST to the Discord webhookURL using a dedicated
// HTTP client with a bounded timeout and proper response body closure.
func sendToDiscord(webhookURL string, data FormSubmission) error {
	var msg DiscordMessage
	msg.Content = fmt.Sprintf("Name: %s , Message: %s , Email: %s", data.Name, data.Message, data.Email)
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

	// Drain remaining response body to ensure underlying TCP connection is reusable
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord returned non-success status code: %d", resp.StatusCode)
	}

	return nil
}

// Function that handles all submiting information of form, verifies the actual correct information format so the app doesnt crash.
func submit(w http.ResponseWriter, r *http.Request) {
	// Enable CORS for all incoming origins
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Handle CORS preflight OPTIONS request
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// If a user tries to call different method than POST
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST, OPTIONS")
		sendJSONError(w, http.StatusMethodNotAllowed, "Only POST method is allowed.")
		return
	}

	// Enforce 1MB request body limit
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)

	var data FormSubmission
	webhookURL := os.Getenv("DISCORD_WEBHOOK_URL")

	// Here we decode data and if the payload is incorrect or too large, it gives us an error
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) || strings.Contains(err.Error(), "request body too large") {
			sendJSONError(w, http.StatusRequestEntityTooLarge, "Request body exceeds 1MB limit")
			return
		}
		sendJSONError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// In the functions below we check for the correct formatting of the data, we also check if the data is not empty
	data.Name = strings.TrimSpace(data.Name)
	data.Email = strings.TrimSpace(data.Email)
	data.Message = strings.TrimSpace(data.Message)

	if data.Name == "" {
		sendJSONError(w, http.StatusBadRequest, "Name cannot be empty")
		return
	}

	if data.Message == "" {
		sendJSONError(w, http.StatusBadRequest, "Message cannot be empty")
		return
	}

	if !isValidEmail(data.Email) {
		sendJSONError(w, http.StatusBadRequest, "Email has to be in a valid format example@example.example")
		return
	}

	fmt.Printf("Received from form -> Name: %s, Email: %s, Message: %s\n", data.Name, data.Email, data.Message)

	// If the webhookURL isnt empty, we send the data to the webhook, if it is, we dont
	if strings.TrimSpace(webhookURL) != "" {
		if err := sendToDiscord(webhookURL, data); err != nil {
			log.Printf("Warning: failed to dispatch discord webhook: %v", err)
		}
	}
	sendJSONSuccess(w, http.StatusOK, "Form was successfully received.")
}

// Default HOME Endpoint
func home(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	fmt.Fprintln(w, "FormStream API Server")
}

func main() {

	http.HandleFunc("/submit", submit)
	http.HandleFunc("/", home)

	fmt.Println("Server is running on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
