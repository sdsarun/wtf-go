package ch1

import (
	"fmt"
	"log"
	"net/http"
)

const (
	HOST_1 = "localhost"
	PORT_1 = 4000
)

func handler1(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "URL.Path = %q/\n", r.URL.Path)
}

func Server1() {
	http.HandleFunc("/", handler1)
	log.Printf("Server is listening on http://%s:%d", HOST_1, PORT_1)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", PORT_1), nil))
}
