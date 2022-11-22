package config

import (
	"runtime"
	"time"
)

const (
	DefaultChatRouletteInterval = "biweekly"
	DefaultChatRouletteWeekday  = "Monday"
	DefaultChatRouletteHour     = 12 // UTC

	DefaultServerAddr = "0.0.0.0"
	DefaultServerPort = 8080

	DefaultDBMaxOpen     = 20
	DefaultDBMaxIdle     = 10
	DefaultDBMaxLifetime = 60 * time.Minute
	DefaultDBMaxIdletime = 15 * time.Minute
)

var (
	DefaultWorkerConcurrency = runtime.NumCPU()
)
