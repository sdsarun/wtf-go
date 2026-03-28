package ch1

import (
	"fmt"
	"log"
	"net/http"
)

func handler3(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "%s %s %s\n", r.Method, r.URL, r.Proto)
	for k, v := range r.Header {
		fmt.Fprintf(w, "Header[%q] = %q\n", k, v)
	}
	fmt.Fprintf(w, "Host = %q\n", r.Host)
	fmt.Fprintf(w, "RemoteAddr = %q\n", r.RemoteAddr)
	if err := r.ParseForm(); err != nil {
		log.Print(err)
	}
	for k, v := range r.Form {
		fmt.Fprintf(w, "Form[%q] = %q\n", k, v)
	}
}

func Server3() {
	http.HandleFunc("/", handler3)
	log.Printf("Server is listening on http://%s:%d", HOST_2, PORT_2)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", PORT_2), nil))
}
