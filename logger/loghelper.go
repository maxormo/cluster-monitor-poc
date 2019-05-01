package logger

import "fmt"

type Logger interface {
	Printfln(format string, a ...interface{})
}

type logger struct {
	component string
}

func GetLogger(component string) Logger {
	return logger{component: component}
}

func (log logger) Printfln(format string, a ...interface{}) {
	format = "[" + log.component + "] " + format + "\n"
	fmt.Printf(format, a...)
}
