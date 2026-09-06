package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

// maxRequestBodySize defines the maximum allowed size for request bodies (1MB).
const maxRequestBodySize = 1024 * 1024

// sendJSONError sends a structured JSON error response with the given status code and message.
func sendJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(APIResponse{
		Success: false,
		Message: message,
		Error:   message,
	})
}

// sendJSONSuccess sends a structured JSON success response with the given status code and message.
func sendJSONSuccess(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(APIResponse{
		Success: true,
		Message: message,
	})
}

// submit handles incoming form submissions, validates fields, persists to storage, and dispatches to Discord.
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

	// Fast rejection if declared Content-Length exceeds 1MB limit
	if r.ContentLength > maxRequestBodySize {
		sendJSONError(w, http.StatusRequestEntityTooLarge, "Request body exceeds 1MB limit")
		return
	}

	// Enforce 1MB request body limit
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)

	var data FormSubmission
	webhookURL := os.Getenv("DISCORD_WEBHOOK_URL")

	// Here we decode data and if the payload is incorrect or too large, it gives us an error
	dec := json.NewDecoder(r.Body)
	err := dec.Decode(&data)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) || strings.Contains(err.Error(), "request body too large") {
			sendJSONError(w, http.StatusRequestEntityTooLarge, "Request body exceeds 1MB limit")
			return
		}
		sendJSONError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Reject extraneous trailing tokens or payload garbage after the JSON object
	var extra json.RawMessage
	if err := dec.Decode(&extra); err != io.EOF {
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

	// Persist the valid submission safely into submissions.json
	if err := defaultStore.Save(data); err != nil {
		log.Printf("Error saving submission to storage: %v", err)
		sendJSONError(w, http.StatusInternalServerError, "Failed to persist submission.")
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

// submissions handles requests to inspect stored form submissions during testing.
func submissions(w http.ResponseWriter, r *http.Request) {
	// Enable CORS for all incoming origins
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Handle CORS preflight OPTIONS request
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Reject non-GET requests
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET, OPTIONS")
		sendJSONError(w, http.StatusMethodNotAllowed, "Only GET method is allowed.")
		return
	}

	subs, err := defaultStore.GetAll()
	if err != nil {
		log.Printf("Error retrieving submissions: %v", err)
		sendJSONError(w, http.StatusInternalServerError, "Failed to retrieve submissions.")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(subs); err != nil {
		log.Printf("Error encoding submissions response: %v", err)
	}
}

// home handles the root endpoint and returns a status greeting.
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
