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
	// ByteLimit is the maximum number of bytes to log for a single argument.
	BytesLimit int
}

const (
	LogLevelError = tracelog.LogLevelError
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

func (t *traceLog) logQueryArgs(args []any) []any {
	// log only if error and debug level. Avoid logging sensitive data in other cases.
	if t.BytesLimit == 0 || len(args) == 0 || t.LogLevel < LogLevelError || t.TraceLog.LogLevel != tracelog.LogLevelDebug {
		return nil
	}

	logArgs := make([]any, 0, len(args))

	for _, a := range args {
		switch v := a.(type) {
		case []byte:
			if len(v) < t.BytesLimit {
				a = hex.EncodeToString(v)
			} else {
				a = fmt.Sprintf("%x (truncated %d bytes)", v[:t.BytesLimit], len(v)-t.BytesLimit)
			}
		case string:
			if len(v) > t.BytesLimit {
				var l int = 0
				for w := 0; l < t.BytesLimit; l += w {
					_, w = utf8.DecodeRuneInString(v[l:])
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

func (t *traceLog) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, tracelogQueryCtxKey, &traceQueryData{
		startTime: time.Now(),
		sql:       data.SQL,
		args:      data.Args,
	})
}

func (t *traceLog) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	queryData, ok := ctx.Value(tracelogQueryCtxKey).(*traceQueryData)
	if !ok {
		return
	}

	endTime := time.Now()
	interval := endTime.Sub(queryData.startTime)

	if data.Err != nil {
		if t.LogLevel >= LogLevelError {
			t.log(ctx, conn, LogLevelError, "Query", map[string]any{"sql": queryData.sql, "args": t.logQueryArgs(queryData.args), "err": data.Err, "time": interval})
		}

		return
	}

	if t.LogLevel >= LogLevelInfo {
		t.log(ctx, conn, LogLevelInfo, "Query", map[string]any{"sql": queryData.sql, "args": t.logQueryArgs(queryData.args), "time": interval, "commandTag": data.CommandTag.String()})
	}
}
