package logger

import "fmt"

func Printfln(format string, a ...interface{}) {
	fmt.Printf(format+"\n", a...)
}
