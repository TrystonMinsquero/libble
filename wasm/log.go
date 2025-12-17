package main

import (
	"fmt"
	"os"
	"syscall/js"

	clog "github.com/charmbracelet/log"
	. "libble/shared"
)

var logg = logger()

func logger() *clog.Logger {
	level := clog.WarnLevel
	if isDebug {
		level = clog.DebugLevel
	}
	logger := clog.NewWithOptions(os.Stderr, clog.Options{
		ReportTimestamp: false,
		Level:           level,
	})
	SetSharedLogger(logger)
	return logger
}

func logErr(contextFmt string, args ...any) {
	context := fmt.Sprintf(contextFmt, args...)
	logg.Error(context)
	console := js.Global().Get("console")
	console.Call("error", context)
}

func log(err error, context string) {
	if err == nil {
		return
	}
	logErr("%s\n%v", context, err)
}
