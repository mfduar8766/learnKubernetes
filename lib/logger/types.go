package logger

type ILogger interface {
	LogInfo(payload *LoggerPayload)
	LogInfof(format string, a ...any)
	LogWarning(payload *LoggerPayload)
	LogWarnf(format string, a ...any)
	LogDebug(payload *LoggerPayload)
	LogError(payload *LoggerPayload)
	LogErrorf(format string, a ...any)
	LogFatal(payload *LoggerPayload)
	LogFatalf(format string, a ...any)
	SetMasks(masks []string)
	Close()
}
