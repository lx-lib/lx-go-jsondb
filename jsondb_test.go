package jsondb

import (
	"fmt"
	"testing"
	"time"

	"azugo.io/core/config"
	"azugo.io/core/server"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Result struct {
	Property string `json:"property" validate:"len=100" example:"test"`
}

type TestConfiguration struct {
	*config.Configuration `mapstructure:",squash"`

	Postgres *Configuration `mapstructure:"postgres"`
}

// Core returns the core configuration.
func (c *TestConfiguration) Core() *config.Configuration {
	return c.Configuration
}

// Loaded is called when the configuration is loaded.
func (c *TestConfiguration) Loaded(core *config.Configuration) {
	c.Configuration = core
}

func (c *TestConfiguration) Bind(_ string, v *viper.Viper) {
	c.Postgres = config.Bind(c.Postgres, "postgres", v)
}

func TestPostgresStoreExec(t *testing.T) {
	conf := &TestConfiguration{
		Configuration: config.New(),
	}

	a, err := server.New(nil, server.Options{
		Configuration: conf,
	})
	require.NoError(t, err)

	_ = a.Start()
	defer a.Stop()

	store, _, err := New(a, conf.Postgres)
	require.NoError(t, err)

	_ = store.Start(a.BackgroundContext())

	resp := &Result{}
	err = store.Exec(a.BackgroundContext(), "public.test1", make(map[string]string), resp)
	require.NoError(t, err)
	assert.Equal(t, "value", resp.Property)
}

// TestPostgresConnWait demonstrates connection retry logic with exponential backoff.
// Note: In standard unit tests this will succeed immediately. This test is primarily useful for:
//   - Integration testing with database restarts
//   - Demonstrating retry patterns
//   - Testing actual connection resilience
//
// If connection fails after 12 seconds, it likely indicates a real configuration or infrastructure issue.
func TestPostgresConnWait(t *testing.T) {
	conf := &TestConfiguration{
		Configuration: config.New(),
	}

	a, err := server.New(nil, server.Options{
		Configuration: conf,
	})
	require.NoError(t, err)

	var (
		store Store
		db    *pgxpool.Pool
	)

	ctx := a.BackgroundContext()

	const (
		maxWaitTime     = 12 * time.Second // Maximum time to wait for database to become available
		backoffInterval = 3 * time.Second  // Time to wait between retry attempts
		maxAttempts     = 4                // Maximum number of connection attempts (maxWaitTime / backoffInterval)
	)

	var backoff time.Duration

	attempt := 0

	for backoff < maxWaitTime {
		attempt++

		if backoff > 0 {
			a.Log().Warn(fmt.Sprintf("Connection attempt %d/%d: waiting %s for database to become accessible",
				attempt, maxAttempts, backoff))
			time.Sleep(backoff)
		}

		store, db, err = New(a, conf.Postgres)
		if err == nil {
			store.Start(ctx)

			if store.IsReady() && store.Ping(ctx) == nil {
				a.Log().Info(fmt.Sprintf("Database connection established on attempt %d", attempt))

				break
			}
		}

		backoff += backoffInterval
	}

	require.True(t, store.IsReady(), "store is not ready after %d attempts (%s)", maxAttempts, maxWaitTime)

	_, err = db.Exec(ctx, "select 1")
	require.NoError(t, err)

	_ = a.Start()
	defer a.Stop()

	store.Start(ctx)
	defer store.Close()

	// Perform actual database operation test
	resp := &Result{}
	err = store.Exec(ctx, "public.test1", make(map[string]string), resp)
	require.NoError(t, err)
	assert.Equal(t, "value", resp.Property)
}
