package ch1

import (
	"fmt"
	"log"
	"net/http"
	"sync"
)

const (
	HOST_2 = "localhost"
	PORT_2 = 4000
)

var (
	mu2    sync.Mutex
	count2 int
)

func handler2(w http.ResponseWriter, r *http.Request) {
	mu2.Lock()
	count2++
	mu2.Unlock()
	fmt.Fprintf(w, "URL.Path = %q/\n", r.URL.Path)
}

func counter2(w http.ResponseWriter, r *http.Request) {
	mu2.Lock()
	fmt.Fprintf(w, "Count %d\n", count2)
	mu2.Unlock()
}

func Server2() {
	http.HandleFunc("/", handler2)
	http.HandleFunc("/count", counter2)
	log.Printf("Server is listening on http://%s:%d", HOST_2, PORT_2)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", PORT_2), nil))
}
