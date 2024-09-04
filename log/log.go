package log

import (
	"github.com/rs/zerolog"
	"go.uber.org/zap"
)

type Logger interface {
	Error(err error)
	Warn(err error)
}

type Noop struct{}

func (n Noop) Error(_ error) {}
func (n Noop) Warn(_ error)  {}

type zapWrapper struct {
	logger *zap.Logger
}

func NewFromZap(logger *zap.Logger) Logger {
	return zapWrapper{logger: logger}
}

func (z zapWrapper) Error(err error) {
	if err == nil {
		return
	}

	z.logger.Error(err.Error())
}

func (z zapWrapper) Warn(err error) {
	if err == nil {
		return
	}

	z.logger.Warn(err.Error())
}

type zeroLogWrapper struct {
	logger zerolog.Logger
}

func NewFromZerolog(logger zerolog.Logger) Logger {
	return zeroLogWrapper{logger: logger}
}

func (z zeroLogWrapper) Error(err error) {
	if err == nil {
		return
	}

	z.logger.Error().Msg(err.Error())
}

func (z zeroLogWrapper) Warn(err error) {
	if err == nil {
		return
	}

	z.logger.Warn().Msg(err.Error())
}
