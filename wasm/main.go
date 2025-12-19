package main

import (
	"syscall/js"
	"time"
)

func main() {
	debugPrint("You're in debug mode from wasm!")

	handlePage()
	// Wait for DOM to be ready, then initialize the game
	doc := js.Global().Get("document")
	// I tried using onLoaded handler but it didn't work, so I just did this instead
	for doc.Get("readyState").String() != "complete" {
		time.Sleep(time.Millisecond * 1)
	}
	if isPage(PageGame) {
		initGame()
	} else if isPage(PageStart) {
		initStart()
	}

	<-make(chan bool) // Prevents "Uncaught Error: Go program has already exited"
}
