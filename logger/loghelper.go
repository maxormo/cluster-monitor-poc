package logger

import "fmt"

type Logger interface {
	Printfln(format string, a ...interface{})
	PrintErr(err error)
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

func (log logger) PrintErr(err error) {
	if err == nil {
		return
	}
	format := "[" + log.component + "] %s\n"
	fmt.Printf(format, err.Error())
}
