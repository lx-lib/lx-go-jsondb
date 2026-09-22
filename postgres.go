package jsondb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"azugo.io/core"
	"github.com/goccy/go-json"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type postgresStore struct {
	app     *core.App
	db      *pgxpool.Pool
	stlock  *sync.RWMutex
	started bool
	tasks   []core.Tasker
}

// Option configures a PostgreSQL store during creation.
// Options are applied after default initialization, allowing customization
// of behavior such as logging and query tracing.
type Option func(*traceLog)

// WithTraceLogger returns an Option that overrides the default query trace logger.
// This is useful for testing or integrating with custom logging frameworks.
func WithTraceLogger(l tracelog.Logger) Option {
	return func(tl *traceLog) {
		tl.Logger = l
	}
}

// New creates a new PostgreSQL store.
func New(a *core.App, config *Configuration, opts ...Option) (Store, *pgxpool.Pool, error) {
	c, err := pgxpool.ParseConfig(config.ToConnectionString(a.AppName))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse database connection string: %w", err)
	}

	level := zap.WarnLevel

	if len(config.LogLevel) > 0 {
		level, err = zapcore.ParseLevel(config.LogLevel)
		if err != nil {
			level = zap.WarnLevel
		}
	}

	// Map the zap logger level to the pgx logger
	dblevel := tracelog.LogLevelNone

	switch level {
	case zap.DebugLevel:
		dblevel = tracelog.LogLevelDebug
	case zap.InfoLevel:
		dblevel = tracelog.LogLevelInfo
	case zap.WarnLevel:
		dblevel = tracelog.LogLevelWarn
	case zap.ErrorLevel:
		dblevel = tracelog.LogLevelError
	case zap.DPanicLevel:
		dblevel = tracelog.LogLevelError
	case zap.PanicLevel:
		dblevel = tracelog.LogLevelError
	case zap.FatalLevel:
		dblevel = tracelog.LogLevelError
	case zapcore.InvalidLevel:
	}

	tl := &traceLog{
		TraceLog: &tracelog.TraceLog{
			Logger: &logger{
				app:   a,
				level: level,
			},
			LogLevel: dblevel,
		},
		BytesLimit:         config.LogBytesLimit,
		SlowQueryThreshold: config.SlowQueryThreshold,
		DebugQueryArgs:     config.DebugQueryArgs,
		DebugSlowQueryArgs: config.DebugSlowQueryArgs,
	}

	for _, opt := range opts {
		opt(tl)
	}

	c.ConnConfig.Tracer = tl

	db, err := pgxpool.NewWithConfig(a.BackgroundContext(), c)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	s := &postgresStore{
		app:    a,
		db:     db,
		stlock: &sync.RWMutex{},
	}

	return s, db, nil
}

// IsReady returns true if the store is ready to use.
func (s *postgresStore) IsReady() bool {
	s.stlock.RLock()
	defer s.stlock.RUnlock()

	return s.started
}

// AddTask adds a task to the store.
func (s *postgresStore) AddTask(task core.Tasker) {
	s.tasks = append(s.tasks, task)
}

// Start starts the store and tasks.
func (s *postgresStore) Start(ctx context.Context) error {
	finish := s.app.Instrumenter().Observe(ctx, InstrumentationStart)

	s.stlock.Lock()
	defer s.stlock.Unlock()

	for _, task := range s.tasks {
		if err := task.Start(ctx); err != nil {
			finish(err)

			return err
		}
	}

	s.started = true

	finish(nil)

	return nil
}

// Ping checks if the store is available.
func (s *postgresStore) Ping(ctx context.Context) error {
	finish := s.app.Instrumenter().Observe(ctx, InstrumentationPing)

	err := s.db.Ping(ctx)
	if err != nil {
		finish(err)

		return fmt.Errorf("failed to ping database: %w", err)
	}

	finish(nil)

	return nil
}

// Close the store.
func (s *postgresStore) Close() {
	finish := s.app.Instrumenter().Observe(context.TODO(), InstrumentationClose)
	defer finish(nil)

	s.stlock.Lock()
	s.started = false
	s.stlock.Unlock()

	for _, task := range s.tasks {
		task.Stop()
	}

	s.db.Close()
}

type execResult struct {
	Success bool        `json:"success"`
	Code    string      `json:"code"`
	Error   string      `json:"error"`
	Data    interface{} `json:"data"`
}

// handlePgErrConstraint handles constraint errors and returns errExec model.
func handlePgErrConstraint(rowsErr error, result interface{}) error {
	pgErr := &pgconn.PgError{}
	if errors.As(rowsErr, &pgErr) && pgErr.Code == "P0001" {
		r := &execResult{
			Data: result,
		}

		msgBytes := []byte(pgErr.Message)
		if err := json.Unmarshal(msgBytes, r); err != nil {
			return fmt.Errorf("failed to execute procedure: %w", rowsErr)
		}

		return ExecError{
			Code:    r.Code,
			Message: r.Error,
		}
	}

	return fmt.Errorf("failed to execute procedure: %w", rowsErr)
}

// Tx represents database transaction.
type Tx struct {
	context.Context
	mu   sync.Mutex
	done bool
}

// Begin starts a new transaction and wraps it in the context.
func (s *postgresStore) Begin(ctx context.Context) (*Tx, error) {
	if !s.IsReady() {
		return nil, ErrStoreNotReady
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}

	ctx = wrapTxCtx(ctx, tx)

	return &Tx{Context: ctx}, nil
}

// Rollback retrieves the transaction from context and rolls it back.
func (s *Tx) Rollback() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.done {
		return nil
	}

	_, tx := FetchTxCtx(s)
	if tx == nil {
		return errors.New("no transaction found in context")
	}

	if err := tx.Rollback(s); err != nil {
		return err
	}

	s.done = true

	return nil
}

// Commit retrieves the transaction from context and commits it.
func (s *Tx) Commit() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.done {
		return nil
	}

	_, tx := FetchTxCtx(s)
	if tx == nil {
		return errors.New("no transaction found in context")
	}

	if err := tx.Commit(s); err != nil {
		return err
	}

	s.done = true

	return nil
}

// Exec executes an database method.
func (s *postgresStore) Exec(ctx context.Context, method string, params interface{}, result interface{}) error {
	if !s.IsReady() {
		return ErrStoreNotReady
	}

	var (
		err  error
		data []byte
		inTx = false
		tx   *Tx
		pgTx pgx.Tx
	)

	if params != nil {
		data, err = json.Marshal(params)
		if err != nil {
			return fmt.Errorf("failed to marshal params to JSON: %w", err)
		}
	} else {
		data = []byte("{}")
	}

	var schema, proc string

	parts := strings.Split(method, ".")
	if len(parts) == 2 {
		schema, proc = parts[0], parts[1]
	} else {
		schema, proc = "public", parts[0]
	}

	if _, pgTx = FetchTxCtx(ctx); pgTx != nil {
		inTx = true
		tx = &Tx{Context: ctx}
	} else {
		tx, err = s.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		_, pgTx = FetchTxCtx(tx)
		if pgTx == nil {
			return errors.New("failed to fetch transaction from context")
		}
	}

	finish := s.app.Instrumenter().Observe(tx, InstrumentationExec, method) //nolint:contextcheck // Tx embeds the original context

	rows, err := pgTx.Query(ctx, fmt.Sprintf(`CALL "%s"."%s"($1, $2)`, schema, proc), string(data), "{}")
	if err != nil {
		err = handlePgErrConstraint(err, result)

		if !inTx {
			_ = tx.Rollback() //nolint:contextcheck // Tx embeds context
		}

		finish(err)

		return err
	}

	if !rows.Next() {
		rows.Close()
		err = handlePgErrConstraint(rows.Err(), result)

		if !inTx {
			_ = tx.Rollback() //nolint:contextcheck // Tx embeds context
		}

		finish(err)

		return err
	}

	var op []byte

	err = rows.Scan(&op)
	if err != nil {
		rows.Close()

		err = fmt.Errorf("failed to scan procedure result: %w", err)

		if !inTx {
			_ = tx.Rollback() //nolint:contextcheck // Tx embeds context
		}

		finish(err)

		return err
	}

	rows.Close()

	r := &execResult{
		Data: result,
	}
	if err := json.Unmarshal(op, r); err != nil {
		err = fmt.Errorf("failed to unmarshal procedure result: %w", err)

		if !inTx {
			_ = tx.Rollback() //nolint:contextcheck // Tx embeds context
		}

		finish(err)

		return err
	}

	if !inTx {
		if err := tx.Commit(); err != nil { //nolint:contextcheck // Tx embeds context
			err = fmt.Errorf("failed to commit transaction: %w", err)
			finish(err)

			return err
		}
	}

	if !r.Success {
		finish(nil)

		return ExecError{
			Code:    r.Code,
			Message: r.Error,
		}
	}

	finish(nil)

	return nil
}
