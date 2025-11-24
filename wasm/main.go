package main

import (
	"fmt"
	"reflect"
	"syscall/js"
)

// func fetchDaily(userID string) (int, error) {
// 	origin := js.Global().Get("window").Get("location").Get("origin").String()
// 	res, err := http.Get(origin + "/daily/" + userID)
// 	if err != nil {
// 		return -1, fmt.Errorf("Failed fetching daily data for %s: %v", userID, err)
// 	}
// 	if err := res.Header.Get("error"); err != "" {
// 		return -1, fmt.Errorf("Error in header: %v", err)
// 	}
//
// 	bodyString, err := io.ReadAll(res.Body)
// 	if err != nil {
// 		return -1, fmt.Errorf("Error reading response body for Daily request: %v", err)
// 	}
//
// 	var dailyData DailyData
// 	if err = json.Unmarshal(bodyString, &dailyData); err != nil {
// 		return -1, fmt.Errorf("Error unmarshalling daily json: %v", err)
// 	}
//
// 	// TODO: Sync Data
//
// 	return dailyData.DailyQuote, nil
// }

func structToMap(obj interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	val := reflect.ValueOf(obj)

	// Handle pointers if the input is a pointer to a struct
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// Ensure it's a struct
	if val.Kind() != reflect.Struct {
		return nil // Or handle error appropriately
	}

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldValue := val.Field(i)

		// Use the JSON tag name if available, otherwise use the field name
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
			fieldName = jsonTag
		}

		result[fieldName] = fieldValue.Interface()
	}
	return result
}

func main() {
	fmt.Println("Hello from Wasm!!")

	handlePage()

	// Wait for DOM to be ready, then initialize the game
	doc := js.Global().Get("document")
	if doc.Get("readyState").String() == "complete" {
		fmt.Println("Ready state")
		if isPage(PageGame) {
			initGame()
		} else if isPage(PageStart) {
			initStart()
		}
	} else {
		fmt.Println("Listening for DOMContentLoaded")
		js.Global().Call("addEventListener", "DOMContentLoaded",
			js.FuncOf(func(this js.Value, args []js.Value) interface{} {
				fmt.Println("DOMContentLoaded")
				if isPage(PageGame) {
					initGame()
				} else if isPage(PageStart) {
					initStart()
				}
				return nil
			}))
	}

	<-make(chan bool) // Prevents "Uncaught Error: Go program has already exited"
}
