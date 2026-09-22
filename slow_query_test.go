package jsondb

import (
	"context"
	"testing"
	"time"

	"azugo.io/core/config"
	"azugo.io/core/server"
	"github.com/jackc/pgx/v5/tracelog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type slowResult struct {
	Delayed int `json:"delayed"`
}

type slowParams struct {
	DelayMs int `json:"delayMs"`
}

func newStoreWithObserver(t *testing.T, threshold time.Duration) (Store, *observer.ObservedLogs, func()) {
	t.Helper()

	core, logs := observer.New(zap.WarnLevel)
	obsLogger := zap.New(core)

	conf := &TestConfiguration{Configuration: config.New()}

	a, err := server.New(nil, server.Options{Configuration: conf})
	require.NoError(t, err)

	_ = a.Start()

	conf.Postgres.SlowQueryThreshold = threshold

	store, _, err := New(a, conf.Postgres, WithTraceLogger(&zapTraceLogger{obsLogger}))
	require.NoError(t, err)

	require.NoError(t, store.Start(a.BackgroundContext()))

	return store, logs, func() {
		store.Close()
		a.Stop()
	}
}

// zapTraceLogger adapts *zap.Logger to tracelog.Logger.
type zapTraceLogger struct{ log *zap.Logger }

func (l *zapTraceLogger) Log(_ context.Context, level tracelog.LogLevel, msg string, data map[string]any) {
	fields := make([]zap.Field, 0, len(data))
	for k, v := range data {
		fields = append(fields, zap.Any(k, v))
	}

	switch level {
	case tracelog.LogLevelError:
		l.log.Error(msg, fields...)
	case tracelog.LogLevelWarn:
		l.log.Warn(msg, fields...)
	default:
		l.log.Info(msg, fields...)
	}
}

// TestSlowQuery_GlobalThreshold verifies that a SlowQuery warning is emitted
// when the query exceeds the global threshold.
func TestSlowQuery_GlobalThreshold(t *testing.T) {
	t.Parallel()
	store, logs, stop := newStoreWithObserver(t, 50*time.Millisecond)
	defer stop()

	result := &slowResult{}
	err := store.Exec(t.Context(), "public.test_slow", &slowParams{DelayMs: 100}, result)
	require.NoError(t, err)
	assert.Equal(t, 100, result.Delayed)

	assert.Equal(t, 1, logs.FilterMessage("SlowQuery").Len(), "expected one SlowQuery log entry")
}

// TestSlowQuery_PerCallOverride verifies that WithSlowQueryThreshold suppresses
// the SlowQuery warning when the per-call threshold is high enough.
func TestSlowQuery_PerCallOverride(t *testing.T) {
	t.Parallel()
	store, logs, stop := newStoreWithObserver(t, 10*time.Millisecond)
	defer stop()

	ctx := WithSlowQueryThreshold(t.Context(), 5*time.Second)

	result := &slowResult{}
	err := store.Exec(ctx, "public.test_slow", &slowParams{DelayMs: 50}, result)
	require.NoError(t, err)
	assert.Equal(t, 50, result.Delayed)

	assert.Equal(t, 0, logs.FilterMessage("SlowQuery").Len(), "expected no SlowQuery log entry with per-call override")
}
