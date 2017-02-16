package pkg2

import "go.uber.org/zap"

func Print() {
	l, _ := zap.NewProduction()
	l.Error("test print")
}
