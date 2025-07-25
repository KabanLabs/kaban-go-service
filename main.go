package main

import (
	"fmt"
	"net/http"
)

func main() {
	server := &http.Server{
		Addr:    ":3000",
		Handler: http.HandlerFunc(basicHandler),
	}

	err := server.ListenAndServe()

	if err != nil {
		fmt.Println("Failed to start server", err)
	}

	fmt.Println("Server started on port 3000")
}

func basicHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(" Hello World! "))
}
