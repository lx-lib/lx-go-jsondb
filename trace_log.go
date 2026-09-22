package jsondb

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/tracelog"
)

type traceLog struct {
	*tracelog.TraceLog
	// BytesLimit is the maximum number of bytes to log for a single argument.
	BytesLimit int
	// SlowQueryThreshold is the duration after which a query is considered slow.
	SlowQueryThreshold time.Duration
	// DebugQueryArgs enables logging of query arguments for all queries. WARNING: This may log sensitive data.
	DebugQueryArgs bool
	// DebugSlowQueryArgs enables logging of query arguments for slow queries only. WARNING: This may log sensitive data.
	DebugSlowQueryArgs bool
}

const (
	LogLevelError = tracelog.LogLevelError
	LogLevelWarn  = tracelog.LogLevelWarn
	LogLevelInfo  = tracelog.LogLevelInfo
)

type traceQueryData struct {
	startTime time.Time
	sql       string
	args      []any
}

func (tl *traceLog) log(ctx context.Context, conn *pgx.Conn, lvl tracelog.LogLevel, msg string, data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}

	pgConn := conn.PgConn()
	if pgConn != nil {
		pid := pgConn.PID()
		if pid != 0 {
			data["pid"] = pid
		}
	}

	tl.Logger.Log(ctx, lvl, msg, data)
}

func (tl *traceLog) truncateArgs(args []any) []any {
	if tl.BytesLimit == 0 || len(args) == 0 {
		return nil
	}

	logArgs := make([]any, 0, len(args))

	for _, a := range args {
		switch v := a.(type) {
		case []byte:
			if len(v) < tl.BytesLimit {
				a = hex.EncodeToString(v)
			} else {
				a = fmt.Sprintf("%x (truncated %d bytes)", v[:tl.BytesLimit], len(v)-tl.BytesLimit)
			}
		case string:
			if len(v) > tl.BytesLimit {
				l := 0
				for l < tl.BytesLimit {
					_, w := utf8.DecodeRuneInString(v[l:])
					l += w
				}

				if len(v) > l {
					a = fmt.Sprintf("%s (truncated %d bytes)", v[:l], len(v)-l)
				}
			}
		}

		logArgs = append(logArgs, a)
	}

	return logArgs
}

const tracelogQueryCtxKey contextKey = iota

func (tl *traceLog) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, tracelogQueryCtxKey, &traceQueryData{
		startTime: time.Now(),
		sql:       data.SQL,
		args:      data.Args,
	})
}

func (tl *traceLog) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	queryData, ok := ctx.Value(tracelogQueryCtxKey).(*traceQueryData)
	if !ok {
		return
	}

	elapsed := time.Since(queryData.startTime)

	if data.Err != nil {
		if tl.LogLevel >= LogLevelError {
			logData := map[string]any{
				"sql":     queryData.sql,
				"err":     data.Err,
				"elapsed": fmt.Sprintf("%dms", elapsed.Milliseconds()),
			}

			if tl.DebugQueryArgs {
				logData["args"] = tl.truncateArgs(queryData.args)
			}

			tl.log(ctx, conn, LogLevelError, "Query", logData)
		}

		return
	}

	if slowThreshold := tl.slowThreshold(ctx); slowThreshold > 0 && elapsed >= slowThreshold {
		logData := map[string]any{
			"elapsed":   fmt.Sprintf("%dms", elapsed.Milliseconds()),
			"threshold": fmt.Sprintf("%dms", slowThreshold.Milliseconds()),
			"sql":       queryData.sql,
		}

		if tl.DebugSlowQueryArgs || tl.DebugQueryArgs {
			logData["args"] = tl.truncateArgs(queryData.args)
		}

		tl.log(ctx, conn, LogLevelWarn, "SlowQuery", logData)

		return
	}

	if tl.LogLevel >= LogLevelInfo {
		logData := map[string]any{
			"sql":        queryData.sql,
			"elapsed":    fmt.Sprintf("%dms", elapsed.Milliseconds()),
			"commandTag": data.CommandTag.String(),
		}

		if tl.DebugQueryArgs {
			logData["args"] = tl.truncateArgs(queryData.args)
		}

		tl.log(ctx, conn, LogLevelInfo, "Query", logData)
	}
}

func (tl *traceLog) slowThreshold(ctx context.Context) time.Duration {
	if d, ok := ctx.Value(slowQryCtxKey).(time.Duration); ok && d > 0 {
		return d
	}

	return tl.SlowQueryThreshold
}
