package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"syscall/js"
)

// Set from ldflags during build
var mode string
var APIOrigin string

var isDebug = mode == "debug"

func removeData(key string) {
	localStorage := js.Global().Get("localStorage")
	localStorage.Call("removeItem", key)
}

func saveData(key string, value string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Failed to save to local storage: %v\n", r)
		}
	}()
	err = nil
	localStorage := js.Global().Get("localStorage")
	localStorage.Call("setItem", key, value)
	debugPrint("Saving %v", key)
	return err
}

func compress(b []byte) ([]byte, error) {
	var buffer bytes.Buffer
	compresser := gzip.NewWriter(&buffer)
	defer compresser.Close()
	if _, err := compresser.Write(b); err != nil {
		return buffer.Bytes(), err
	}
	if err := compresser.Close(); err != nil {
		return buffer.Bytes(), err
	}
	return buffer.Bytes(), nil
}

func saveJson(key string, data any) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return errors.Join(fmt.Errorf("Failed to marshal %s", key), err)
	}
	saveData(key, string(jsonBytes))
	return nil
}

func saveCompressedJson(key string, data any) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return errors.Join(fmt.Errorf("Failed to marshal %s", key), err)
	}

	compressedBytes, err := compress(jsonBytes)
	if err != nil {
		return errors.Join(fmt.Errorf("Failed to compress %s", key), err)
	}
	saveData(key, string(compressedBytes))
	return nil
}

func loadData(key string) (stored string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Failed to save to local storage: %v\n", r)
		}
	}()
	localStorage := js.Global().Get("localStorage")
	value := localStorage.Call("getItem", key)
	if value.Type() == js.TypeString {
		stored = value.String()
	}
	return stored, err
}

func loadJson(key string, data any) error {
	savedString, err := loadData(key)
	if err != nil {
		return err
	}
	if savedString == "" {
		return fmt.Errorf("No data was stored at %s", key)
	}
	err = json.Unmarshal([]byte(savedString), data)
	if err != nil {
		return fmt.Errorf("Failed to unmarshel stored json at %s\n%v", key, err)
	}
	return nil
}

func fetch(path string, data any) error {
	return fetchMethod(path, data, "GET", nil)
}

func post(path string, data any, body any) error {
	return fetchMethod(path, data, "POST", body)
}

func put(path string, data any, body any) error {
	return fetchMethod(path, data, "PUT", body)
}

func fetchMethod(path string, data any, method string, body any) (err error) {
	url := strings.TrimSuffix(APIOrigin, "/") + "/" + strings.TrimPrefix(path, "/")

	// Build fetch options
	fetchOpts := map[string]any{
		"method": method,
		"headers": map[string]any{
			"Content-Type": "application/json",
		},
	}

	// Add body for POST/PUT requests
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("Failed to marshal request body: %v", err)
		}
		fetchOpts["body"] = string(jsonBytes)
	}

	// Call fetch
	promise := js.Global().Call("fetch", url, fetchOpts)

	// Create channels for async response
	resultChan := make(chan error, 1)
	defer func() {
		if r := recover(); r != nil {
			debugPrint("Recoving from %v", r)
			resultChan <- fmt.Errorf("Failed to fetch at %s: %v\n", url, r)
		}
	}()

	// Handle response
	promise.Call("then", js.FuncOf(func(this js.Value, args []js.Value) any {
		response := args[0]
		status := response.Get("status").Int()

		// Check for error header
		headers := response.Get("headers")
		errorHeader := headers.Call("get", "error")
		if errorHeader.Truthy() {
			resultChan <- fmt.Errorf("Error in header for %s\n%s", url, errorHeader.String())
			return nil
		}

		// Get response text
		textPromise := response.Call("text")
		textPromise.Call("then", js.FuncOf(func(this js.Value, args []js.Value) any {
			bodyText := args[0].String()

			if status >= 200 && status < 300 {
				if err := json.Unmarshal([]byte(bodyText), data); err != nil {
					resultChan <- fmt.Errorf("Error unmarshalling json for %s\n%v", url, err)
					return nil
				}
				resultChan <- nil
			} else {
				var errorResponse map[string]any
				reqErr := fmt.Errorf("Request to %s via %s failed with %d", url, method, status)
				if json.Unmarshal([]byte(bodyText), &errorResponse) == nil {
					if errStr, ok := errorResponse["error"].(string); ok {
						resultChan <- fmt.Errorf("%v\n%s", reqErr, errStr)
						return nil
					}
				}
				resultChan <- reqErr
			}
			return nil
		}))

		textPromise.Call("catch", js.FuncOf(func(this js.Value, args []js.Value) any {
			resultChan <- fmt.Errorf("Error reading response body for %s\n%v", url, args[0].String())
			return nil
		}))

		return nil
	}))

	// Handle fetch error
	promise.Call("catch", js.FuncOf(func(this js.Value, args []js.Value) any {
		resultChan <- fmt.Errorf("Failed fetching data for %s\n%v", url, args[0].String())
		return nil
	}))

	// Wait for result
	return <-resultChan
}

func shareText(text string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Failed to share text: %v\n", r)
		}
	}()

	navigator := js.Global().Get("navigator")

	platform := navigator.Get("platform").String()
	if strings.Contains(platform, "Win") {
		return fmt.Errorf("Don't use share on windows, it's bad")
	}

	share := navigator.Get("share")
	if !share.Truthy() {
		return fmt.Errorf("navigator.share is not available")
	}

	content := js.ValueOf(map[string]any{"text": text})

	if !navigator.Call("canShare", content).Bool() {
		return fmt.Errorf("Unable to share content %v", content)
	}

	promise := navigator.Call("share", content)

	promise.Call("then", js.FuncOf(func(this js.Value, args []js.Value) any {
		return nil
	}))
	promise.Call("catch", js.FuncOf(func(this js.Value, args []js.Value) any {
		err = fmt.Errorf("Failed to share text")
		return nil
	}))
	return err
}

func copyToClipboard(text string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Failed to copy to clipboard: %v\n", r)
		}
	}()

	navigator := js.Global().Get("navigator")
	clipboard := navigator.Get("clipboard")

	// Create a promise callback
	promise := clipboard.Call("writeText", text)

	// Handle success
	promise.Call("then", js.FuncOf(func(this js.Value, args []js.Value) any {
		return nil
	}))

	// Handle error
	promise.Call("catch", js.FuncOf(func(this js.Value, args []js.Value) any {
		err = fmt.Errorf("Failed to copy to clipboard")
		return nil
	}))
	return err
}
