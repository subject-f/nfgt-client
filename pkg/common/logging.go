package common

import (
	"github.com/sirupsen/logrus"
)

var (
	logger = logrus.New()
)

func init() {
	logger.SetLevel(logrus.DebugLevel)
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
