package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// setupTestStore sets up an isolated temporary submissions.json store for the test duration.
func setupTestStore(t *testing.T) func() {
	t.Helper()
	tempDir := t.TempDir()
	origStore := defaultStore
	testFile := filepath.Join(tempDir, "submissions.json")
	defaultStore = NewSubmissionsStore(testFile)

	return func() {
		defaultStore = origStore
	}
}

// TestCORSPreflightSubmit verifies that an OPTIONS request to /submit returns status 204
// with appropriate CORS headers (Origin, Methods, Headers).
func TestCORSPreflightSubmit(t *testing.T) {
	mux := setupRoutes()

	req := httptest.NewRequest(http.MethodOptions, "/submit", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204 No Content, got %d", rec.Code)
	}

	if origin := rec.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("expected Access-Control-Allow-Origin: *, got %q", origin)
	}

	methods := rec.Header().Get("Access-Control-Allow-Methods")
	if !strings.Contains(methods, "POST") || !strings.Contains(methods, "OPTIONS") {
		t.Errorf("expected Access-Control-Allow-Methods to contain POST and OPTIONS, got %q", methods)
	}

	headers := rec.Header().Get("Access-Control-Allow-Headers")
	if !strings.Contains(headers, "Content-Type") {
		t.Errorf("expected Access-Control-Allow-Headers to contain Content-Type, got %q", headers)
	}
}

// TestCORSPreflightSubmissions verifies that an OPTIONS request to /submissions returns status 204
// with appropriate CORS headers.
func TestCORSPreflightSubmissions(t *testing.T) {
	mux := setupRoutes()

	req := httptest.NewRequest(http.MethodOptions, "/submissions", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204 No Content, got %d", rec.Code)
	}

	if origin := rec.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("expected Access-Control-Allow-Origin: *, got %q", origin)
	}

	methods := rec.Header().Get("Access-Control-Allow-Methods")
	if !strings.Contains(methods, "GET") || !strings.Contains(methods, "OPTIONS") {
		t.Errorf("expected Access-Control-Allow-Methods to contain GET and OPTIONS, got %q", methods)
	}

	headers := rec.Header().Get("Access-Control-Allow-Headers")
	if !strings.Contains(headers, "Content-Type") {
		t.Errorf("expected Access-Control-Allow-Headers to contain Content-Type, got %q", headers)
	}
}

// TestSubmitMethodNotAllowed verifies that non-POST methods to /submit return HTTP 405
// with a structured JSON error response and the Allow header.
func TestSubmitMethodNotAllowed(t *testing.T) {
	mux := setupRoutes()
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/submit", nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("expected status 405 Method Not Allowed, got %d", rec.Code)
			}

			if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "POST") {
				t.Errorf("expected Allow header to contain POST, got %q", allow)
			}

			var resp APIResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("expected valid JSON response, got error: %v", err)
			}
			if resp.Success {
				t.Errorf("expected success: false, got true")
			}
			if resp.Message == "" {
				t.Errorf("expected non-empty error message")
			}
		})
	}
}

// TestSubmitValidation covers table-driven validation scenarios: empty bodies, malformed JSON,
// missing required fields, and various invalid email formats.
func TestSubmitValidation(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	mux := setupRoutes()

	tests := []struct {
		name        string
		payload     string
		expectError string
	}{
		{
			name:        "empty request body",
			payload:     "",
			expectError: "Invalid request payload",
		},
		{
			name:        "malformed json",
			payload:     `{"name": "Jan", "email": }`,
			expectError: "Invalid request payload",
		},
		{
			name:        "missing name",
			payload:     `{"name": "", "email": "jan@example.com", "message": "hello"}`,
			expectError: "Name cannot be empty",
		},
		{
			name:        "whitespace name",
			payload:     `{"name": "   ", "email": "jan@example.com", "message": "hello"}`,
			expectError: "Name cannot be empty",
		},
		{
			name:        "missing message",
			payload:     `{"name": "Jan Novák", "email": "jan@example.com", "message": ""}`,
			expectError: "Message cannot be empty",
		},
		{
			name:        "whitespace message",
			payload:     `{"name": "Jan Novák", "email": "jan@example.com", "message": "   \n\t  "}`,
			expectError: "Message cannot be empty",
		},
		{
			name:        "missing email",
			payload:     `{"name": "Jan Novák", "email": "", "message": "hello"}`,
			expectError: "Email has to be in a valid format",
		},
		{
			name:        "invalid email without at sign",
			payload:     `{"name": "Jan Novák", "email": "janexample.com", "message": "hello"}`,
			expectError: "Email has to be in a valid format",
		},
		{
			name:        "invalid email without domain",
			payload:     `{"name": "Jan Novák", "email": "jan@", "message": "hello"}`,
			expectError: "Email has to be in a valid format",
		},
		{
			name:        "invalid email without username",
			payload:     `{"name": "Jan Novák", "email": "@example.com", "message": "hello"}`,
			expectError: "Email has to be in a valid format",
		},
		{
			name:        "invalid email without dot in domain",
			payload:     `{"name": "Jan Novák", "email": "jan@localhost", "message": "hello"}`,
			expectError: "Email has to be in a valid format",
		},
		{
			name:        "invalid email with leading dot in domain",
			payload:     `{"name": "Jan Novák", "email": "jan@.com", "message": "hello"}`,
			expectError: "Email has to be in a valid format",
		},
		{
			name:        "invalid email with trailing dot in domain",
			payload:     `{"name": "Jan Novák", "email": "jan@com.", "message": "hello"}`,
			expectError: "Email has to be in a valid format",
		},
		{
			name:        "invalid email with spaces",
			payload:     `{"name": "Jan Novák", "email": "jan novak@example.com", "message": "hello"}`,
			expectError: "Email has to be in a valid format",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(tc.payload))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400 Bad Request, got %d: %s", rec.Code, rec.Body.String())
			}

			var resp APIResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode JSON error response: %v", err)
			}

			if resp.Success {
				t.Errorf("expected success: false, got true")
			}
			if !strings.Contains(resp.Message, tc.expectError) {
				t.Errorf("expected message to contain %q, got %q", tc.expectError, resp.Message)
			}
		})
	}
}

// TestSubmitBodyTooLarge verifies that request bodies exceeding the 1MB limit are rejected
// with HTTP 413 StatusRequestEntityTooLarge and a structured JSON response.
func TestSubmitBodyTooLarge(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	mux := setupRoutes()

	// Construct a payload slightly exceeding 1MB
	oversizedMessage := strings.Repeat("A", maxRequestBodySize+1024)
	payload := fmt.Sprintf(`{"name": "Jan Novák", "email": "jan@example.com", "message": %q}`, oversizedMessage)

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413 Request Entity Too Large, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp APIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}
	if resp.Success {
		t.Errorf("expected success: false, got true")
	}
	if !strings.Contains(resp.Message, "exceeds 1MB limit") {
		t.Errorf("expected 1MB limit message, got %q", resp.Message)
	}
}

// TestSubmitValidAndPersistence verifies that a valid form submission is received,
// returns HTTP 200 with structured JSON, and is persisted safely into the store.
func TestSubmitValidAndPersistence(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	// Ensure no webhook URL is set so Discord dispatch is cleanly skipped
	origURL := os.Getenv("DISCORD_WEBHOOK_URL")
	_ = os.Unsetenv("DISCORD_WEBHOOK_URL")
	defer func() {
		if origURL != "" {
			_ = os.Setenv("DISCORD_WEBHOOK_URL", origURL)
		}
	}()

	mux := setupRoutes()

	submissionPayload := `{"name": " Jan Novák ", "email": "jan@example.com", "message": "Hello from tests! "}`
	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(submissionPayload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	if origin := rec.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("expected Access-Control-Allow-Origin: *, got %q", origin)
	}

	var resp APIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected success: true, got false")
	}

	// Verify persistence in the store
	subs, err := defaultStore.GetAll()
	if err != nil {
		t.Fatalf("failed to retrieve submissions from store: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 stored submission, got %d", len(subs))
	}
	if subs[0].Name != "Jan Novák" {
		t.Errorf("expected trimmed name 'Jan Novák', got %q", subs[0].Name)
	}
	if subs[0].Email != "jan@example.com" {
		t.Errorf("expected email 'jan@example.com', got %q", subs[0].Email)
	}
	if subs[0].Message != "Hello from tests!" {
		t.Errorf("expected trimmed message 'Hello from tests!', got %q", subs[0].Message)
	}
}

// TestGetSubmissionsEndpoint verifies GET /submissions returns empty array when empty,
// and returns all stored submissions as JSON when populated.
func TestGetSubmissionsEndpoint(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	mux := setupRoutes()

	// 1. Initially empty store should return empty JSON array []
	req := httptest.NewRequest(http.MethodGet, "/submissions", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", rec.Code)
	}
	if origin := rec.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("expected Access-Control-Allow-Origin: *, got %q", origin)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var emptyList []FormSubmission
	if err := json.Unmarshal(rec.Body.Bytes(), &emptyList); err != nil {
		t.Fatalf("failed to decode empty submissions array: %v", err)
	}
	if len(emptyList) != 0 {
		t.Fatalf("expected 0 submissions, got %d", len(emptyList))
	}

	// 2. Populate two submissions
	err := defaultStore.Save(FormSubmission{
		Name:    "Alice",
		Email:   "alice@example.com",
		Message: "First submission",
	})
	if err != nil {
		t.Fatalf("failed to save first submission: %v", err)
	}

	err = defaultStore.Save(FormSubmission{
		Name:    "Bob",
		Email:   "bob@example.com",
		Message: "Second submission",
	})
	if err != nil {
		t.Fatalf("failed to save second submission: %v", err)
	}

	// 3. Inspect via GET /submissions
	req = httptest.NewRequest(http.MethodGet, "/submissions", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", rec.Code)
	}

	var populatedList []FormSubmission
	if err := json.Unmarshal(rec.Body.Bytes(), &populatedList); err != nil {
		t.Fatalf("failed to decode populated submissions array: %v", err)
	}
	if len(populatedList) != 2 {
		t.Fatalf("expected 2 submissions, got %d", len(populatedList))
	}
	if populatedList[0].Name != "Alice" || populatedList[1].Name != "Bob" {
		t.Errorf("unexpected submission contents: %+v", populatedList)
	}
}

// TestSubmissionsMethodNotAllowed verifies non-GET methods on /submissions return 405 Method Not Allowed.
func TestSubmissionsMethodNotAllowed(t *testing.T) {
	mux := setupRoutes()
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/submissions", nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("expected status 405 Method Not Allowed, got %d", rec.Code)
			}
			if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
				t.Errorf("expected Allow header to contain GET, got %q", allow)
			}
		})
	}
}

// TestDiscordWebhookSuccess verifies that sendToDiscord correctly formats the Discord message,
// dispatches it to the webhook URL, and drains the response body.
func TestDiscordWebhookSuccess(t *testing.T) {
	var receivedBody DiscordMessage
	receivedRequest := false

	mockDiscord := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedRequest = true
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type: application/json, got %q", ct)
		}
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read request body: %v", err)
		}
		_ = json.Unmarshal(bodyBytes, &receivedBody)

		w.WriteHeader(http.StatusNoContent)
	}))
	defer mockDiscord.Close()

	sub := FormSubmission{
		Name:    "Jan Novák",
		Email:   "jan@example.com",
		Message: "Hello Discord!",
	}

	err := sendToDiscord(mockDiscord.URL, sub)
	if err != nil {
		t.Fatalf("expected sendToDiscord to succeed, got %v", err)
	}
	if !receivedRequest {
		t.Fatalf("expected mock server to receive request")
	}

	expectedContent := "Name: Jan Novák , Message: Hello Discord! , Email: jan@example.com"
	if receivedBody.Content != expectedContent {
		t.Errorf("expected Discord content %q, got %q", expectedContent, receivedBody.Content)
	}
}

// TestDiscordWebhookTimeout verifies that discordHTTPClient times out on slow webhook endpoints
// without hanging indefinitely, and closes response bodies.
func TestDiscordWebhookTimeout(t *testing.T) {
	// Create a slow server that delays longer than client timeout
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer slowServer.Close()

	// Temporarily configure a very short timeout
	origClient := discordHTTPClient
	discordHTTPClient = &http.Client{
		Timeout: 50 * time.Millisecond,
	}
	defer func() {
		discordHTTPClient = origClient
	}()

	sub := FormSubmission{
		Name:    "Slow Tester",
		Email:   "slow@example.com",
		Message: "Testing timeout",
	}

	err := sendToDiscord(slowServer.URL, sub)
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}
}

// TestDiscordWebhookNonSuccessStatus verifies that sendToDiscord returns an error
// when Discord returns a non-2xx status code.
func TestDiscordWebhookNonSuccessStatus(t *testing.T) {
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message": "Unknown Webhook"}`, http.StatusNotFound)
	}))
	defer errorServer.Close()

	sub := FormSubmission{
		Name:    "Error Tester",
		Email:   "error@example.com",
		Message: "Testing error status",
	}

	err := sendToDiscord(errorServer.URL, sub)
	if err == nil {
		t.Fatalf("expected error for non-2xx status, got nil")
	}
}

// TestSubmissionsStoreConcurrency verifies that concurrent writes and reads to SubmissionsStore
// do not cause race conditions or corrupt the JSON data store.
func TestSubmissionsStoreConcurrency(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	const numWriters = 20
	const writesPerWorker = 5

	var wg sync.WaitGroup
	wg.Add(numWriters)

	for i := 0; i < numWriters; i++ {
		workerID := i
		go func() {
			defer wg.Done()
			for j := 0; j < writesPerWorker; j++ {
				err := defaultStore.Save(FormSubmission{
					Name:    fmt.Sprintf("User-%d-%d", workerID, j),
					Email:   fmt.Sprintf("user-%d-%d@example.com", workerID, j),
					Message: fmt.Sprintf("Message from worker %d iteration %d", workerID, j),
				})
				if err != nil {
					t.Errorf("concurrent Save failed: %v", err)
				}

				// Interleaved read to exercise concurrent RLock
				_, readErr := defaultStore.GetAll()
				if readErr != nil {
					t.Errorf("concurrent GetAll failed: %v", readErr)
				}
			}
		}()
	}

	wg.Wait()

	allSubs, err := defaultStore.GetAll()
	if err != nil {
		t.Fatalf("failed to read all submissions after concurrent writes: %v", err)
	}

	expectedCount := numWriters * writesPerWorker
	if len(allSubs) != expectedCount {
		t.Fatalf("expected %d stored submissions, got %d", expectedCount, len(allSubs))
	}
}

// TestHomeEndpoint verifies the root / endpoint and 404 behavior on unknown paths.
func TestHomeEndpoint(t *testing.T) {
	mux := setupRoutes()

	// GET / returns 200 OK
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK for /, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "FormStream API Server") {
		t.Errorf("expected body to contain 'FormStream API Server', got %q", rec.Body.String())
	}

	// GET /unknown returns 404 Not Found
	reqNotFound := httptest.NewRequest(http.MethodGet, "/unknown-endpoint", nil)
	recNotFound := httptest.NewRecorder()
	mux.ServeHTTP(recNotFound, reqNotFound)

	if recNotFound.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 Not Found for /unknown-endpoint, got %d", recNotFound.Code)
	}

	// OPTIONS / returns 204 No Content
	reqOptions := httptest.NewRequest(http.MethodOptions, "/", nil)
	recOptions := httptest.NewRecorder()
	mux.ServeHTTP(recOptions, reqOptions)

	if recOptions.Code != http.StatusNoContent {
		t.Fatalf("expected status 204 No Content for OPTIONS /, got %d", recOptions.Code)
	}
}

// TestSubmissionsStoreCorruptedFileRecovery verifies that when submissions.json contains
// corrupted or invalid JSON prior to write, Save() safely backs it up to a .corrupted-* file,
// preserves the data, and successfully persists the new submission so future requests succeed.
func TestSubmissionsStoreCorruptedFileRecovery(t *testing.T) {
	tempDir := t.TempDir()
	origStore := defaultStore
	testFile := filepath.Join(tempDir, "submissions.json")
	store := NewSubmissionsStore(testFile)
	defaultStore = store
	defer func() {
		defaultStore = origStore
	}()

	// 1. Write corrupted JSON to the file
	corruptedData := []byte(`[{"name": "broken", "email":`)
	if err := os.WriteFile(testFile, corruptedData, 0600); err != nil {
		t.Fatalf("failed to write corrupted test file: %v", err)
	}

	// 2. GetAll() should return an error because JSON is malformed
	_, err := store.GetAll()
	if err == nil {
		t.Fatalf("expected GetAll to return error for corrupted JSON, got nil")
	}

	// 3. GET /submissions endpoint should return 500
	mux := setupRoutes()
	req := httptest.NewRequest(http.MethodGet, "/submissions", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500 for corrupted submissions file, got %d", rec.Code)
	}

	// 4. Save() should detect corruption, back up the file, and persist the new submission
	validSub := FormSubmission{
		Name:    "Recovery Tester",
		Email:   "recovery@example.com",
		Message: "Testing recovery from corrupted file",
	}
	if err := store.Save(validSub); err != nil {
		t.Fatalf("expected Save to succeed and recover from corrupted file, got: %v", err)
	}

	// 5. GetAll() should now succeed and return the new submission
	subs, err := store.GetAll()
	if err != nil {
		t.Fatalf("expected GetAll to succeed after recovery: %v", err)
	}
	if len(subs) != 1 || subs[0].Name != validSub.Name {
		t.Fatalf("expected 1 recovered submission, got %+v", subs)
	}

	// 6. Verify the backup file was created and contains the original corrupted data
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("failed to read temp dir: %v", err)
	}
	var backupFound bool
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "submissions.json.corrupted-") {
			backupFound = true
			data, err := os.ReadFile(filepath.Join(tempDir, entry.Name()))
			if err != nil {
				t.Fatalf("failed to read backup file: %v", err)
			}
			if string(data) != string(corruptedData) {
				t.Errorf("backup file content mismatch: expected %q, got %q", corruptedData, data)
			}
			break
		}
	}
	if !backupFound {
		t.Fatalf("expected corrupted file backup to exist in %s", tempDir)
	}
}

// TestSubmitWithDiscordErrorsAndNetworkFailures verifies that /submit still responds with 200 OK
// and safely persists the submission even when Discord returns 5xx server errors or is unreachable.
func TestSubmitWithDiscordErrorsAndNetworkFailures(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	mux := setupRoutes()

	// Scenario 1: Discord returns 500 Internal Server Error
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message": "Discord server error"}`, http.StatusInternalServerError)
	}))
	defer errorServer.Close()

	origURL := os.Getenv("DISCORD_WEBHOOK_URL")
	_ = os.Setenv("DISCORD_WEBHOOK_URL", errorServer.URL)
	defer func() {
		if origURL != "" {
			_ = os.Setenv("DISCORD_WEBHOOK_URL", origURL)
		} else {
			_ = os.Unsetenv("DISCORD_WEBHOOK_URL")
		}
	}()

	payload1 := `{"name": "Resilient 1", "email": "resilient1@example.com", "message": "Test 500 Discord"}`
	req1 := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(payload1))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	mux.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK even when Discord returns 500, got %d: %s", rec1.Code, rec1.Body.String())
	}

	// Scenario 2: Discord endpoint is an unreachable port (network failure)
	_ = os.Setenv("DISCORD_WEBHOOK_URL", "http://127.0.0.1:59999/unreachable")
	payload2 := `{"name": "Resilient 2", "email": "resilient2@example.com", "message": "Test unreachable Discord"}`
	req2 := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(payload2))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK even when Discord is unreachable, got %d: %s", rec2.Code, rec2.Body.String())
	}

	// Verify both submissions were safely persisted in the store
	subs, err := defaultStore.GetAll()
	if err != nil {
		t.Fatalf("failed to retrieve stored submissions: %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("expected 2 persisted submissions, got %d", len(subs))
	}
}

// TestDiscordWebhookContentTruncation verifies that messages exceeding Discord's 2000-rune limit
// are safely truncated to fit within Discord API constraints so the notification does not fail.
func TestDiscordWebhookContentTruncation(t *testing.T) {
	var receivedContent string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msg DiscordMessage
		_ = json.NewDecoder(r.Body).Decode(&msg)
		receivedContent = msg.Content
		w.WriteHeader(http.StatusNoContent)
	}))
	defer mockServer.Close()

	longMessage := strings.Repeat("X", 3000)
	sub := FormSubmission{
		Name:    "Long Tester",
		Email:   "long@example.com",
		Message: longMessage,
	}

	if err := sendToDiscord(mockServer.URL, sub); err != nil {
		t.Fatalf("expected sendToDiscord to succeed with long content, got: %v", err)
	}

	if len([]rune(receivedContent)) > maxDiscordContentLength {
		t.Errorf("expected received content length <= %d, got %d", maxDiscordContentLength, len([]rune(receivedContent)))
	}
	if !strings.HasSuffix(receivedContent, "... [truncated]") {
		t.Errorf("expected content to end with truncation notice, got %q", receivedContent[len(receivedContent)-30:])
	}
}

// TestDiscordWebhookDetailedErrors verifies that sendToDiscord validates URL schemes
// and extracts the response body snippet on 5xx or error responses.
func TestDiscordWebhookDetailedErrors(t *testing.T) {
	// Invalid scheme
	sub := FormSubmission{
		Name:    "Scheme Tester",
		Email:   "scheme@example.com",
		Message: "Testing invalid scheme",
	}
	err := sendToDiscord("ftp://discord.com/webhook", sub)
	if err == nil || !strings.Contains(err.Error(), "must start with http:// or https://") {
		t.Fatalf("expected invalid scheme error, got: %v", err)
	}

	// 500 error with descriptive snippet
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error": "Gateway Timeout from upstream"}`))
	}))
	defer server.Close()

	err = sendToDiscord(server.URL, sub)
	if err == nil {
		t.Fatalf("expected error for 502 Bad Gateway, got nil")
	}
	if !strings.Contains(err.Error(), "502") || !strings.Contains(err.Error(), "Gateway Timeout") {
		t.Errorf("expected error to include status 502 and body snippet, got: %v", err)
	}
}

// TestSubmitNonASCIIAndInternational verifies that non-ASCII Unicode characters (diacritics, emoji)
// are properly received, persisted, and retrieved without corruption.
func TestSubmitNonASCIIAndInternational(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	origURL := os.Getenv("DISCORD_WEBHOOK_URL")
	_ = os.Unsetenv("DISCORD_WEBHOOK_URL")
	defer func() {
		if origURL != "" {
			_ = os.Setenv("DISCORD_WEBHOOK_URL", origURL)
		}
	}()

	mux := setupRoutes()

	payload := `{"name": "Tomáš Dvořák", "email": "tomas.dvorak@example.com", "message": "Příliš žluťoučký kůň úpěl ďábelské ódy. 🚀 ✨"}`
	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	subs, err := defaultStore.GetAll()
	if err != nil {
		t.Fatalf("failed to get stored submissions: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 stored submission, got %d", len(subs))
	}
	if subs[0].Name != "Tomáš Dvořák" {
		t.Errorf("expected name 'Tomáš Dvořák', got %q", subs[0].Name)
	}
	if subs[0].Message != "Příliš žluťoučký kůň úpěl ďábelské ódy. 🚀 ✨" {
		t.Errorf("expected Unicode message to match, got %q", subs[0].Message)
	}

	// Verify GET /submissions endpoint returns exact Unicode string
	reqGet := httptest.NewRequest(http.MethodGet, "/submissions", nil)
	recGet := httptest.NewRecorder()
	mux.ServeHTTP(recGet, reqGet)

	if !strings.Contains(recGet.Body.String(), "Tomáš Dvořák") {
		t.Errorf("expected GET /submissions response to contain 'Tomáš Dvořák', got %s", recGet.Body.String())
	}
}

// TestEmailValidationExtended tests valid and invalid email forms including punycode, internationalized, DNS label rules, and RFC length limits.
func TestEmailValidationExtended(t *testing.T) {
	validEmails := []string{
		"user@example.com",
		"user.name+tag@sub.domain.co.uk",
		"user@xn--mnchen-3ya.de",
		"user@münchen.de", // RFC 6532 internationalized email address
		"test_user@domain.org",
	}
	for _, email := range validEmails {
		if !isValidEmail(email) {
			t.Errorf("expected email %q to be valid, got false", email)
		}
	}

	invalidEmails := []string{
		"user@",
		"@domain.com",
		"user@domain",
		"user@.domain.com",
		"user@domain.com.",
		"user@domain..com",
		"user name@domain.com",
		"user@-example.com",
		"user@example.-com",
		"user@example.com-",
		"user@-.com",
		"user@example.c",
		"user@example.123",
		strings.Repeat("a", 245) + "@example.com", // Total length > 254
		strings.Repeat("a", 65) + "@example.com",  // Local part > 64
	}
	for _, email := range invalidEmails {
		if isValidEmail(email) {
			t.Errorf("expected email %q to be invalid, got true", email)
		}
	}
}

// TestSubmitTrailingDataRejected verifies that payloads containing extraneous data
// after the main JSON object are rejected with HTTP 400 Bad Request.
func TestSubmitTrailingDataRejected(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	mux := setupRoutes()

	payload := `{"name": "Jan Novák", "email": "jan@example.com", "message": "hello"} extra_trailing_garbage`
	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request for trailing garbage, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestSubmitTrailingDataExceedingLimit verifies that trailing data exceeding the 1MB limit
// triggers an HTTP 413 StatusRequestEntityTooLarge response.
func TestSubmitTrailingDataExceedingLimit(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	mux := setupRoutes()

	// Valid JSON followed by >1MB of trailing spaces
	payload := `{"name": "Jan Novák", "email": "jan@example.com", "message": "hello"}` + strings.Repeat(" ", maxRequestBodySize+100)
	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413 for trailing data exceeding 1MB limit, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestSubmitContentLengthHeaderTooLarge verifies fast rejection when Content-Length header > 1MB.
func TestSubmitContentLengthHeaderTooLarge(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	mux := setupRoutes()

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(`{"name": "A"}`))
	req.ContentLength = maxRequestBodySize + 1
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413 for oversized ContentLength header, got %d", rec.Code)
	}
}

// TestHomeMethodNotAllowed verifies non-GET/HEAD methods on the home endpoint return 405 Method Not Allowed.
func TestHomeMethodNotAllowed(t *testing.T) {
	mux := setupRoutes()
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/", nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("expected status 405 Method Not Allowed for %s /, got %d", method, rec.Code)
			}
			if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
				t.Errorf("expected Allow header to contain GET, got %q", allow)
			}
		})
	}
}

// TestDiscordWebhookWhitespaceURL verifies that leading and trailing whitespace on the webhook URL
// is cleanly trimmed so notifications dispatch successfully.
func TestDiscordWebhookWhitespaceURL(t *testing.T) {
	received := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	sub := FormSubmission{
		Name:    "Whitespace URL Tester",
		Email:   "whitespace@example.com",
		Message: "Testing URL whitespace trimming",
	}

	err := sendToDiscord("   "+server.URL+"   \n", sub)
	if err != nil {
		t.Fatalf("expected sendToDiscord to succeed with padded URL, got: %v", err)
	}
	if !received {
		t.Fatalf("expected mock server to receive request")
	}
}

// TestStoreFilePermissions verifies that saved submissions.json has readable file permissions.
func TestStoreFilePermissions(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "submissions.json")
	store := NewSubmissionsStore(testFile)

	err := store.Save(FormSubmission{
		Name:    "Perm Tester",
		Email:   "perm@example.com",
		Message: "Testing file permissions",
	})
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("os.Stat failed: %v", err)
	}

	// Verify the file has read permissions set for owner and others (0644)
	mode := info.Mode().Perm()
	if mode&0444 == 0 {
		t.Errorf("expected readable permissions, got %o", mode)
	}
}

