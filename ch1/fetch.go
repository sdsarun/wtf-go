package ch1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

var client = &http.Client{
	Timeout: 10 * time.Second,
}

func GetJSON(url string, target any) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(target)
}

func Fetch() {
	// for _, url := range os.Args[1:] {
	// 	if !strings.HasPrefix(url, "https") || strings.HasPrefix(url, "http") {
	// 		url = "http://" + url
	// 	}
	// 	resp, err := http.Get(url)
	// 	if err != nil {
	// 		fmt.Fprintf(os.Stderr, "fetch: %v\n", err)
	// 		os.Exit(1)
	// 	}
	// 	b, err := io.Copy(os.Stdout, resp.Body)
	// 	resp.Body.Close()
	// 	if err != nil {
	// 		fmt.Fprintf(os.Stderr, "fetch: reading %s: %v\n", url, err)
	// 		os.Exit(1)
	// 	}
	// 	fmt.Printf("%s\n", b)
	// 	fmt.Println("Status:", resp.Status)
	// 	fmt.Println("HttpStatusCode:", resp.StatusCode)
	// }

	var data interface{}
	err := GetJSON("https://fakestoreapi.com/products", &data)
	if err != nil {
		fmt.Println("ERROR")
	}
	pretty, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(pretty))
}
