package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"syscall/js"
)

func saveData(key string, value string) {
	localStorage := js.Global().Get("localStorage")
	localStorage.Call("setItem", key, value)
	fmt.Printf("Stored %s: %s\n", key, value)
}

func saveJson(key string, data any) {
	bytes, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		fmt.Printf("Failed to marshal %s: %v", key, err)
		return
	}
	saveData(key, string(bytes))
}

func loadData(key string) string {
	localStorage := js.Global().Get("localStorage")
	value := localStorage.Call("getItem", key)
	if value.Type() == js.TypeString {
		return value.String()
	}
	return ""
}

func loadJson(key string, data any) {
	savedString := loadData(key)
	err := json.Unmarshal([]byte(savedString), data)
	if err != nil {
		fmt.Printf("Failed to unmarshal stored json: %v", err)
		return
	}
}

func fetch(path string, data any) error {
	origin := js.Global().Get("window").Get("location").Get("origin").String()
	url, err := url.JoinPath(origin, path)
	if err != nil {
		return fmt.Errorf("Failed parsing path")
	}

	res, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("Failed fetching data for %s: %v", url, err)
	}
	if err := res.Header.Get("error"); err != "" {
		return fmt.Errorf("Error in header for %s: %v", path, err)
	}

	bodyString, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("Error reading response body for Daily request: %v", err)
	}

	if err = json.Unmarshal(bodyString, data); err != nil {
		return fmt.Errorf("Error unmarshalling json: %v", err)
	}

	return nil
}
