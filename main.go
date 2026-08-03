package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

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

// Function that parses the data from FormSubmission into the Content of the Discord Message, converts into JsonBytes and sends it via http.Post on your webhookURL
func sendToDiscord(webhookURL string, data FormSubmission) {
	var msg DiscordMessage
	msg.Content = fmt.Sprintf("Name: %s , Message: %s , Email: %s", data.Name, data.Message, data.Email)
	jsonBytes, err := json.Marshal(msg)
	if err != nil {
		return
	}
	http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonBytes))

}

// Function that handles all submiting information of form, verifies the actual correct information format so the app doesnt crash.
func submit(w http.ResponseWriter, r *http.Request) {
	var data FormSubmission
	webhookURL := os.Getenv("DISCORD_WEBHOOK_URL")

	//If a user tries to call different method than POST
	if r.Method != http.MethodPost {
		fmt.Fprintln(w, "Only POST method is allowed.")

		return
	}

	//Here we decode data and if the payload is incorrect, it gives us an error
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)

		return
	}

	//In the functions below we check for the correct formatting of the data, we also check if the data is not empty
	if strings.TrimSpace(data.Name) == "" {
		http.Error(w, "Name cannot be empty", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(data.Message) == "" {
		http.Error(w, "Message cannot be empty", http.StatusBadRequest)
		return
	}

	if !strings.Contains(data.Email, "@") {
		http.Error(w, "Email has to be in a valid format example@example.example", http.StatusBadRequest)
		return
	}

	fmt.Printf("Received from form -> Name: %s, Email: %s, Message: %s\n", data.Name, data.Email, data.Message)

	//If the webhookURL isnt empty, we send the data to the webhook, if it is, we dont
	if strings.TrimSpace(webhookURL) != "" {
		sendToDiscord(webhookURL, data)
	}
	fmt.Fprintln(w, "Form was succesfully received.")
}

// Default HOME Endpoint
func home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "FormStream API Server")
}

func main() {

	http.HandleFunc("/submit", submit)
	http.HandleFunc("/", home)

	fmt.Println("Server is running on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
