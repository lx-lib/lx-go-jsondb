package jsondb

import (
	"context"
	"sync"

	"azugo.io/core"
	zapadapter "github.com/jackc/pgx-zap"
	"github.com/jackc/pgx/v5/tracelog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// wrapCoreWithLevel wraps the core with new minimum log level.
func wrapCoreWithLevel(level zapcore.Level) zap.Option {
	return zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		return &coreWithLevel{
			Core:  core,
			level: level,
		}
	})
}

type coreWithLevel struct {
	zapcore.Core
	level zapcore.Level
}

func (c *coreWithLevel) Enabled(level zapcore.Level) bool {
	return c.level.Enabled(level)
}

func (c *coreWithLevel) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if !c.level.Enabled(e.Level) {
		return ce
	}

	return ce.AddCore(e, c.Core)
}

type logger struct {
	app    *core.App
	init   sync.Once
	level  zapcore.Level
	logger tracelog.Logger
}

func (l *logger) Log(ctx context.Context, level tracelog.LogLevel, msg string, data map[string]any) {
	l.init.Do(func() {
		l.logger = zapadapter.NewLogger(l.app.Log().Named("store").WithOptions(wrapCoreWithLevel(l.level)))
	})

	l.logger.Log(ctx, level, msg, data)
}
