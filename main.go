package main

import (
	"fmt"
	"net/http"
)

func submit(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Endpoint pro formulář")
}

func home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "FormStream API Server")
}

func main() {

	http.HandleFunc("/submit", submit)
	http.HandleFunc("/", home)

	fmt.Println("Server běží na http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
