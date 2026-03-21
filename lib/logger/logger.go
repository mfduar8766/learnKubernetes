package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"syscall"
	"time"
)

type LogLevel int8

const (
	info = iota
	warning
	debug
	error
	fatal
)

func (l LogLevel) String() string {
	switch l {
	case info:
		return "info"
	case warning:
		return "warning"
	case debug:
		return "debug"
	case error:
		return "error"
	case fatal:
		return "fatal"
	}
	return ""
}

func (l LogLevel) GetIndex() int8 {
	return int8(l)
}

type loggerPayload struct {
	Level    string      `json:"level"`
	Source   string      `json:"source"`
	Time     string      `json:"time"`
	Message  string      `json:"message,omitempty"`
	Value    interface{} `json:"value,omitempty"`
	FileName string      `json:"fileName,omitempty"`
	Method   string      `json:"method,omitempty"`
}

type LoggerPayload struct {
	Message  string `json:"message,omitempty"`
	Value    any    `json:"value,omitempty"`
	FileName string `json:"fileName,omitempty"`
	Method   string `json:"method,omitempty"`
}

type Logger struct {
	date          string
	serviceName   string
	loggerMessage *loggerPayload
	file          *os.File
	terminateChan chan bool
	mask          []string
}

func getDate() string {
	return time.Now().UTC().Format("2006-01-02:15:04:05.000")
}

func NewLogger(serviceName string) *Logger {
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	var date = time.Now().Local().UTC().String()
	var logger *Logger = &Logger{
		date:          date,
		serviceName:   serviceName,
		terminateChan: make(chan bool, 1),
		loggerMessage: &loggerPayload{
			Level:  LogLevel(0).String(),
			Source: serviceName,
			Time:   date,
		},
		mask: make([]string, 0),
	}
	logger.file = logger.createDir()
	return logger
}

func (l *Logger) createFile() string {
	const regex = `^\S*`
	regEx := regexp.MustCompile(regex)
	return fmt.Sprintf("%s.log", regEx.FindString(l.date))
}

func (l *Logger) createDir() *os.File {
	path, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	var dirName = filepath.Join(path, "Logs")
	if _, err := os.Stat(dirName); os.IsNotExist(err) {
		err = os.MkdirAll(dirName, os.ModePerm)
		if err != nil {
			panic(err)
		}
	}
	file, err := os.OpenFile(filepath.Join(filepath.Join(dirName), filepath.Base(l.createFile())), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		file.Close()
		panic(err)
	}
	return file
}

// TODO: Implement this
func (l *Logger) SetMasks(masks []string) {
	l.mask = masks
}

func (l *Logger) Close() {
	if l.file != nil {
		l.file.Close()
	}
}

func (l *Logger) LogInfo(payload *LoggerPayload) {
	defer l.writeToFile()
	l.setLoggerData(LogLevel.String(0), payload)
}

func (l *Logger) LogInfof(format string, a ...any) {
	defer l.writeToFile()
	l.setLoggerData(LogLevel.String(0), &LoggerPayload{
		Message: fmt.Sprintf(format, a...),
	})
}

func (l *Logger) LogWarning(payload *LoggerPayload) {
	defer l.writeToFile()
	l.setLoggerData(LogLevel.String(1), payload)
}

func (l *Logger) LogWarnf(format string, a ...any) {
	defer l.writeToFile()
	l.setLoggerData(LogLevel.String(1), &LoggerPayload{
		Message: fmt.Sprintf(format, a...),
	})
}

func (l *Logger) LogDebug(payload *LoggerPayload) {
	defer l.writeToFile()
	l.setLoggerData(LogLevel.String(2), payload)
}

func (l *Logger) LogError(payload *LoggerPayload) {
	defer l.writeToFile()
	l.setLoggerData(LogLevel.String(3), payload)
}

func (l *Logger) LogErrorf(format string, a ...any) {
	defer l.writeToFile()
	l.setLoggerData(LogLevel.String(3), &LoggerPayload{
		Message: fmt.Sprintf(format, a...),
	})
}

func (l *Logger) LogFatal(payload *LoggerPayload) {
	defer l.writeToFile()
	l.setLoggerData(LogLevel.String(4), payload)
}

func (l *Logger) LogFatalf(format string, a ...any) {
	defer l.writeToFile()
	l.setLoggerData(LogLevel.String(4), &LoggerPayload{
		Message: fmt.Sprintf(format, a...),
	})
}

func (l *Logger) setLoggerData(level string, payload *LoggerPayload) {
	l.loggerMessage.Time = getDate()
	l.loggerMessage.Level = level
	if len(payload.FileName) > 0 {
		l.loggerMessage.FileName = payload.FileName
	}
	if len(payload.Method) > 0 {
		l.loggerMessage.Method = payload.Method
	}
	if payload.Value != nil {
		l.loggerMessage.Value = payload.Value
		if len(payload.Message) > 0 {
			l.loggerMessage.Message = payload.Message
		}
	}
	if len(payload.Message) > 0 {
		l.loggerMessage.Message = payload.Message
		if payload.Value != nil {
			l.loggerMessage.Value = payload.Value
		}
	}
}

func (l *Logger) writeToFile() {
	data, err := json.Marshal(l.loggerMessage)
	if err != nil {
		fmt.Printf("Error marshalling log: %v\n", err)
		return
	}

	// Print to console and file
	fmt.Println(string(data))
	if l.file != nil {
		l.file.Write(append(data, '\n')) // Add newline for readability in the file
	}

	l.loggerMessage.Value = nil
	l.loggerMessage.Message = ""
	l.loggerMessage.FileName = ""
	l.loggerMessage.Method = ""
}
