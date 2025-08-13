package jsondb

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"azugo.io/core"
	"azugo.io/core/paginator"
	"github.com/valyala/fasthttp"
)

const (
	InstrumentationStart = "store-start"
	InstrumentationClose = "store-close"
	InstrumentationPing  = "store-ping"
	InstrumentationExec  = "store-exec"
)

// ErrStoreNotReady represents error when call is being made before store is ready to accept requests.
var ErrStoreNotReady = errors.New("store not ready")

// ExecError is an error that is returned when a method returns error.
type ExecError struct {
	Code    string
	Message string
}

// Error returns the error message.
func (e ExecError) Error() string {
	return e.Message
}

// IsExecError returns true if the error is an error returned from database method call.
func IsExecError(err error) bool {
	return errors.As(err, &ExecError{})
}

func (e ExecError) SafeError() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e ExecError) StatusCode() int {
	if strings.HasSuffix(e.Code, ":not_found") {
		return fasthttp.StatusNotFound
	}

	return fasthttp.StatusUnprocessableEntity
}

// Store interface for data store.
type Store interface {
	// Start starts the store
	Start(context context.Context) error
	// IsReady returns true if the store is ready to accept requests
	IsReady() bool
	// Close closes the store
	Close()
	// AddTask adds a task to the store
	AddTask(task core.Tasker)
	// Ping returns error if the store is not reachable
	Ping(ctx context.Context) error
	// Exec executes a method on the store
	Exec(ctx context.Context, method string, params interface{}, data interface{}) error
}

// Paging is used to get a paged list of items.
type Paging struct {
	// Page is the page number
	Page int `json:"page"`
	// PageSize is the number of items per page
	PageSize int `json:"perPage"`
}

// NewPaging creates a new paging object from Azugo paginator.
func NewPaging(p *paginator.Paginator) *Paging {
	return &Paging{
		Page:     p.Current(),
		PageSize: p.PageSize(),
	}
}

// InstrExec returns method name if the operation is store exec.
func InstrExec(op string, args ...any) (string, bool) {
	if op != InstrumentationExec || len(args) != 1 {
		return "", false
	}

	key, ok := args[0].(string)

	return key, ok
}
