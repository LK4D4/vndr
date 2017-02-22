package pkg2

import "github.com/uber-go/zap"

func Print() {
	l, _ := zap.NewProduction()
	l.Error("test print")
}
