package logging

import (
	"log"
	"strings"
)

// Level 日志级别
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
)

var currentLevel = LevelInfo

// Init 根据配置或环境变量初始化日志级别
// level 字符串支持：DEBUG / INFO（不区分大小写），默认 INFO。
func Init(level string) {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "DEBUG":
		currentLevel = LevelDebug
	default:
		currentLevel = LevelInfo
	}
}

func Debugf(format string, args ...any) {
	if currentLevel <= LevelDebug {
		log.Printf("[DEBUG] "+format, args...)
	}
}

func Infof(format string, args ...any) {
	if currentLevel <= LevelInfo {
		log.Printf("[INFO] "+format, args...)
	}
}

// Errorf 仍然总是输出（即使在 INFO 级别），用于错误日志。
func Errorf(format string, args ...any) {
	log.Printf("[ERROR] "+format, args...)
}


