package main

import (
	"fmt"
	"log"
	"net/http"
)

// setupRoutes initializes and returns the HTTP serve mux with all endpoints registered.
func setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/submit", submit)
	mux.HandleFunc("/submissions", submissions)
	mux.HandleFunc("/", home)
	return mux
}

// main starts the HTTP server listening on port 8080.
func main() {
	mux := setupRoutes()

	fmt.Println("Server is running on http://localhost:8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
