package eventbus

import (
	"github.com/ThreeDotsLabs/watermill"
	"go.uber.org/zap"
)

// ZapLoggerAdapter adapts zap.Logger to Watermill's LoggerAdapter interface.
type ZapLoggerAdapter struct {
	logger *zap.Logger
}

// NewZapLoggerAdapter creates a new ZapLoggerAdapter.
func NewZapLoggerAdapter(logger *zap.Logger) *ZapLoggerAdapter {
	return &ZapLoggerAdapter{
		logger: logger,
	}
}

func (l *ZapLoggerAdapter) Error(msg string, err error, fields watermill.LogFields) {
	zapFields := l.convertFields(fields)
	zapFields = append(zapFields, zap.Error(err))
	l.logger.Error(msg, zapFields...)
}

func (l *ZapLoggerAdapter) Info(msg string, fields watermill.LogFields) {
	l.logger.Info(msg, l.convertFields(fields)...)
}

func (l *ZapLoggerAdapter) Debug(msg string, fields watermill.LogFields) {
	l.logger.Debug(msg, l.convertFields(fields)...)
}

func (l *ZapLoggerAdapter) Trace(msg string, fields watermill.LogFields) {
	l.logger.Debug(msg, l.convertFields(fields)...)
}

func (l *ZapLoggerAdapter) With(fields watermill.LogFields) watermill.LoggerAdapter {
	return &ZapLoggerAdapter{
		logger: l.logger.With(l.convertFields(fields)...),
	}
}

func (l *ZapLoggerAdapter) convertFields(fields watermill.LogFields) []zap.Field {
	zapFields := make([]zap.Field, 0, len(fields))
	for key, value := range fields {
		zapFields = append(zapFields, zap.Any(key, value))
	}
	return zapFields
}

var _ watermill.LoggerAdapter = (*ZapLoggerAdapter)(nil)
