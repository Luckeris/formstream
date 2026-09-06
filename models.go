package main

// FormSubmission represents the payload sent from a contact form.
type FormSubmission struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Message string `json:"message"`
}

// DiscordMessage represents the payload structure expected by Discord webhook API.
type DiscordMessage struct {
	Content string `json:"content"`
}

// APIResponse represents the standard structured JSON response for API endpoints.
type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}
