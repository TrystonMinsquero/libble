package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"syscall/js"
)

func logErr(context string) {
	console := js.Global().Get("console")
	console.Call("error", context)
	fmt.Printf("Error: %v\n", context)
}

func log(err error, context string) {
	if err == nil {
		return
	}
	logErr(context + "\n" + err.Error())
}

func saveData(key string, value string) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Failed to save to local storage: %v\n", r)
			// TODO: maybe do something else?
		}
	}()
	localStorage := js.Global().Get("localStorage")
	localStorage.Call("setItem", key, value)
	fmt.Printf("Stored %s: %s\n", key, value)
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

	// compressedBytes, err := compress(jsonBytes)
	// if err != nil {
	// 	return errors.Join(fmt.Errorf("Failed to compress %s", key), err)
	// }
	// saveData(key, string(compressedBytes))

	return nil
}

func loadData(key string) string {
	localStorage := js.Global().Get("localStorage")
	value := localStorage.Call("getItem", key)
	if value.Type() == js.TypeString {
		return value.String()
	}
	return ""
}

func loadJson(key string, data any) error {
	savedString := loadData(key)
	if savedString == "" {
		return fmt.Errorf("No data was stored at %s", key)
	}
	err := json.Unmarshal([]byte(savedString), data)
	if err != nil {
		return fmt.Errorf("Failed to unmarshel stored json at %s\n%v", err)
	}
	return nil
}

func fetch(path string, data any, method string) error {
	// NOTE: This will probably change once the server lives somewhere else
	origin := js.Global().Get("window").Get("location").Get("origin").String()
	url, err := url.JoinPath(origin, path)
	if err != nil {
		return fmt.Errorf("Failed parsing path '%s'", path)
	}

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return fmt.Errorf("Failed to create request for %s\n%v", url, err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("Failed fetching data for %s\n%v", url, err)
	}
	if errStr := res.Header.Get("error"); errStr != "" {
		return fmt.Errorf("Error in header for %s\n%s", url, errStr)
	}

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("Error reading response body for %s\n%v", url, err)
	}

	status := res.StatusCode
	if status >= 200 && status < 300 {
		if err = json.Unmarshal(bodyBytes, data); err != nil {
			return fmt.Errorf("Error unmarshalling json for %s\n%v", url, err)
		}
	} else {
		var errorResponse map[string]interface{}
		reqErr := fmt.Errorf("Request to %s via %s failed with %d", url, method, status)
		if json.Unmarshal(bodyBytes, &errorResponse) == nil {
			if errStr, ok := errorResponse["error"].(string); ok {
				return fmt.Errorf("%v\n%s", reqErr, errStr)
			}
		}
		return reqErr
	}

	return nil
}
