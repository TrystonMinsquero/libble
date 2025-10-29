package main

import (
	"encoding/json"
	"fmt"
	"io"
	. "libble/shared"
	"net/http"
	"syscall/js"
)

func saveData(key string, value string) {
	localStorage := js.Global().Get("localStorage")
	localStorage.Call("setItem", "book", value)
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
		fmt.Printf("Faild to unmarshal stored json: %v", err)
		return
	}
}

func fetchDaily(userID string) (int, error) {
	origin := js.Global().Get("window").Get("location").Get("origin").String()
	res, err := http.Get(origin + "/daily/" + userID)
	if err != nil {
		return -1, fmt.Errorf("Failed fetching daily data for %s: %v", userID, err)
	}
	if err := res.Header.Get("error"); err != "" {
		return -1, fmt.Errorf("Error in header: %v", err)
	}

	bodyString, err := io.ReadAll(res.Body)
	if err != nil {
		return -1, fmt.Errorf("Error reading response body for Daily request: %v", err)
	}

	var dailyData DailyData
	if err = json.Unmarshal(bodyString, &dailyData); err != nil {
		return -1, fmt.Errorf("Error unmarshalling daily json: %v", err)
	}

	// TODO: Sync Data

	return dailyData.DailyQuote, nil
}

func main() {
	fmt.Println("Hello from Wasm!!")

	type TestJson struct {
		SomeInt int `json:"some_int"`
	}

	userID := loadData("userId")
	if userID != "" {
		if dailyQuoteIndex, err := fetchDaily(userID); err != nil {
			// TODO: Fallback to localStorage
			fmt.Println(err)
		} else {
			fmt.Printf("Daily quote index: %d\n", dailyQuoteIndex)
		}
	}

	<-make(chan bool) // Prevents "Uncaught Error: Go program has already exited"
}
