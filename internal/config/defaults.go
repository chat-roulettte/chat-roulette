package config

import (
	"runtime"
	"time"
)

const (
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
