package logger

import (
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// Logger 全局日志实例
	Logger *zap.Logger
	// Sugar 全局 Sugar Logger（更简洁的 API）
	Sugar *zap.SugaredLogger
)

// Config 日志配置
type Config struct {
	Level      string // 日志级别: debug, info, warn, error
	OutputPath string // 输出路径（留空则输出到控制台）
	MaxSize    int    // 单个日志文件最大大小（MB）
	MaxBackups int    // 保留的旧日志文件数量
	MaxAge     int    // 保留的旧日志文件天数
	Compress   bool   // 是否压缩旧日志
	DevMode    bool   // 开发模式（更友好的输出格式）
}

// InitLogger 初始化 zap logger
func InitLogger(cfg Config) error {
	// 设置日志级别
	var level zapcore.Level
	switch cfg.Level {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	default:
		level = zapcore.InfoLevel
	}

	// 配置编码器
	var encoderConfig zapcore.EncoderConfig
	if cfg.DevMode {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder // 彩色日志级别
	} else {
		encoderConfig = zap.NewProductionEncoderConfig()
		encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	}

	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder // 时间格式
	encoderConfig.EncodeDuration = zapcore.SecondsDurationEncoder
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder // 调用者信息

	// 配置输出
	var writeSyncer zapcore.WriteSyncer
	if cfg.OutputPath != "" {
		// 确保日志目录存在
		logDir := filepath.Dir(cfg.OutputPath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return err
		}

		// 输出到文件（可以添加日志轮转，如使用 lumberjack）
		file, err := os.OpenFile(cfg.OutputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		writeSyncer = zapcore.AddSync(file)
	} else {
		// 输出到控制台
		writeSyncer = zapcore.AddSync(os.Stdout)
	}

	// 创建 core
	var encoder zapcore.Encoder
	if cfg.DevMode {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	core := zapcore.NewCore(
		encoder,
		writeSyncer,
		level,
	)

	// 创建 logger
	Logger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1), zap.AddStacktrace(zapcore.ErrorLevel))
	Sugar = Logger.Sugar()

	return nil
}

// InitDefaultLogger 使用默认配置初始化 logger
func InitDefaultLogger() error {
	return InitLogger(Config{
		Level:   "info",
		DevMode: true,
	})
}

// InitProductionLogger 使用生产环境配置初始化 logger
func InitProductionLogger(logPath string) error {
	return InitLogger(Config{
		Level:      "info",
		OutputPath: logPath,
		MaxSize:    100,
		MaxBackups: 7,
		MaxAge:     30,
		Compress:   true,
		DevMode:    false,
	})
}

// Debug 记录 debug 级别日志
func Debug(msg string, fields ...zap.Field) {
	Logger.Debug(msg, fields...)
}

// Info 记录 info 级别日志
func Info(msg string, fields ...zap.Field) {
	Logger.Info(msg, fields...)
}

// Warn 记录 warn 级别日志
func Warn(msg string, fields ...zap.Field) {
	Logger.Warn(msg, fields...)
}

// Error 记录 error 级别日志
func Error(msg string, fields ...zap.Field) {
	Logger.Error(msg, fields...)
}

// Fatal 记录 fatal 级别日志并退出程序
func Fatal(msg string, fields ...zap.Field) {
	Logger.Fatal(msg, fields...)
}

// Debugf 使用格式化字符串记录 debug 日志
func Debugf(template string, args ...interface{}) {
	Sugar.Debugf(template, args...)
}

// Infof 使用格式化字符串记录 info 日志
func Infof(template string, args ...interface{}) {
	Sugar.Infof(template, args...)
}

// Warnf 使用格式化字符串记录 warn 日志
func Warnf(template string, args ...interface{}) {
	Sugar.Warnf(template, args...)
}

// Errorf 使用格式化字符串记录 error 日志
func Errorf(template string, args ...interface{}) {
	Sugar.Errorf(template, args...)
}

// Fatalf 使用格式化字符串记录 fatal 日志并退出
func Fatalf(template string, args ...interface{}) {
	Sugar.Fatalf(template, args...)
}

// Sync 刷新缓冲区
func Sync() error {
	if Logger != nil {
		return Logger.Sync()
	}
	return nil
}
