package common

import (
	"runtime"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	logger = logrus.New()
)

const (
	memoryLogInterval = 10
)

func init() {
	logger.SetLevel(logrus.DebugLevel)

	customFormatter := new(logrus.TextFormatter)

	customFormatter.FullTimestamp = true

	logger.SetFormatter(customFormatter)

	go func() {
		for {
			logMemoryUsage()
			time.Sleep(memoryLogInterval * time.Second)
		}
	}()
}

func logMemoryUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	Debugf("Alloc = %v MiB, TotalAlloc = %v MiB, Sys = %v MiB, NumGC = %v\n",
		bToMb(m.Alloc), bToMb(m.TotalAlloc), bToMb(m.Sys), m.NumGC)
}

func Infof(str string, args ...interface{}) {
	logger.Infof(str, args...)
}

func Errorf(str string, args ...interface{}) {
	logger.Errorf(str, args...)
}

func Fatalf(str string, args ...interface{}) {
	logger.Fatalf(str, args...)
}

func Warnf(str string, args ...interface{}) {
	logger.Warnf(str, args...)
}

func Debugf(str string, args ...interface{}) {
	logger.Debugf(str, args...)
}
