package main

import (
	"github.com/LK4D4/dumbproject/pkg1"
	"github.com/LK4D4/dumbproject/pkg2"
	logging "github.com/op/go-logging"
)

func Print() {
	logging.MustGetLogger("main").Error("test print")
}

func main() {
	pkg1.Print()
	pkg2.Print()
}
