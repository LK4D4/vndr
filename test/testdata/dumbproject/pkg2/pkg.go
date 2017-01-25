package pkg2

import "github.com/uber-go/zap"

func Print() {
	zap.New(zap.NewJSONEncoder()).Error("test print")
}
