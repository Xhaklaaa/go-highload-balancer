package logger

import (
	"log"
)

type Logger interface {
	Errorf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
}

type DefaultLogger struct{}

func (l *DefaultLogger) Errorf(format string, args ...interface{}) {
	log.Printf("[ERROR] "+format, args...)
}

func (l *DefaultLogger) Infof(format string, args ...interface{}) {
	log.Printf("[INFO] "+format, args...)
}

func (l *DefaultLogger) Warnf(format string, args ...interface{}) {
	log.Printf("[WARN] "+format, args...)
}

func (l *DefaultLogger) Fatalf(format string, args ...interface{}) {
	log.Printf("[FATAL] "+format, args...)
}
