package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type FormSubmission struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Message string `json:"message"`
}

func submit(w http.ResponseWriter, r *http.Request) {
	var data FormSubmission

	if r.Method != http.MethodPost {
		fmt.Fprintln(w, "Only POST method is allowed.")

		return
	}

	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)

		return
	}

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
	fmt.Fprintln(w, "Form was succesfully received.")
}

func home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "FormStream API Server")
}

func main() {

	http.HandleFunc("/submit", submit)
	http.HandleFunc("/", home)

	fmt.Println("Server is running on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
