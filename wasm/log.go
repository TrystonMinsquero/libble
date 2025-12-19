package main

import (
	"fmt"
	"syscall/js"
)

func debugPrint(format string, args ...any) {
	if isDebug {
		fmt.Printf(format+"\n", args...)
	}
}

func logErr(contextFmt string, args ...any) {
	context := fmt.Sprintf(contextFmt, args...)
	console := js.Global().Get("console")
	console.Call("error", context)
}

func log(err error, context string) {
	if err == nil {
		return
	}
	logErr("%s\n%v", context, err)
}
