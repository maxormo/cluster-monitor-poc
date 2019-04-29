package logger

import "fmt"

type Logger struct {
	Component string
}

func (log Logger) Printfln(format string, a ...interface{}) {
	format = "[" + log.Component + "] " + format + "\n"
	fmt.Printf(format, a...)
}
