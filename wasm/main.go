package main

import (
	// "encoding/json"
	"encoding/json"
	"fmt"
	. "libble/shared"
	"syscall/js"
)

func main() {
	fmt.Println("Hello from Wasm!!")

	type TestJson struct {
		SomeInt int `json:"some_int"`
	}

	localStorage := js.Global().Get("localStorage")

	storedBook := localStorage.Call("getItem", "book")

	saveBook := func(book Book) {
		bookBytes, err := json.MarshalIndent(book, "", "\t")
		if err != nil {
			fmt.Printf("Failed to marshal book: %v", err)
			return
		}
		bookJson := string(bookBytes)

		localStorage.Call("setItem", "book", bookJson)
		fmt.Printf("Stored Book: %s", bookJson)
	}

	if storedBook.Type() == js.TypeString {
		var book Book
		err := json.Unmarshal([]byte(storedBook.String()), &book)
		if err != nil {
			fmt.Printf("Faild to unmarshal stored json: %v", err)
			return
		}

		book.RatingCount += 1
		saveBook(book)
	} else {
		var book Book
		book.Title = "Hell ya"
		book.RatingCount = 1
		saveBook(book)

	}

	// var test TestJson
	// err != json.Unmarshal(bytes, &test)
	//
	// writeTest := func() {
	// 	bytes, err = json.MarshalIndent(test, "", "\t")
	// 	if err != nil {
	// 		fmt.Printf("Error marshalling json: %v\n", err)
	// 		return
	// 	}
	// 	written, err := file.Write(bytes)
	// 	if err != nil {
	// 		fmt.Printf("Error writing file: %v\n", err)
	// 		return
	// 	}
	// 	fmt.Printf("Successfuly wrote %d bytes to file", written)
	// }
	//
	// if err != nil {
	// 	writeTest()
	// } else {
	// 	test.SomeInt += 1
	// 	fmt.Printf("New value is %d", test.SomeInt)
	// 	writeTest()
	// }
}
