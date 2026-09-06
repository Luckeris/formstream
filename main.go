package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

// setupRoutes initializes and returns the HTTP serve mux with all endpoints registered.
func setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/submit", submit)
	mux.HandleFunc("/submissions", submissions)
	mux.HandleFunc("/", home)
	return mux
}

// main starts the HTTP server listening on port 8080 with production connection timeouts.
func main() {
	mux := setupRoutes()

	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	fmt.Println("Server is running on http://localhost:8080")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed to start: %v", err)
	}
}
