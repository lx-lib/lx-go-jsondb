package jsondb

import (
	"context"
	"reflect"

	"github.com/jackc/pgx/v5"
)

type (
	contextKey int8
	stringer   interface {
		String() string
	}
)

func contextName(c context.Context) string {
	if s, ok := c.(stringer); ok {
		return s.String()
	}

	return reflect.TypeOf(c).String()
}

type txCtx struct {
	context.Context
	parentCtx context.Context
	Tx        pgx.Tx
}

func (c *txCtx) String() string {
	return contextName(c.Context) + ".WithTransaction"
}

func (c *txCtx) RequestContext() context.Context {
	return c.parentCtx
}

var ctxKey contextKey

// FetchTxCtx and use transaction passed by context if exists.
// it stores contextData values to struct for fast repeated access.
func FetchTxCtx(ctx context.Context) (context.Context, pgx.Tx) {
	if tctx, ok := ctx.Value(ctxKey).(*txCtx); ok {
		return tctx.parentCtx, tctx.Tx
	}

	return nil, nil
}

// wrapTxCtx wraps transaction inside context.
func wrapTxCtx(ctx context.Context, tx pgx.Tx) *txCtx {
	tctx := &txCtx{
		parentCtx: ctx,
		Tx:        tx,
	}
	tctx.Context = context.WithValue(ctx, ctxKey, tctx)

	return tctx
}
