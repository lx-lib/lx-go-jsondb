package jsondb

import (
	"testing"

	"azugo.io/core/config"
	"azugo.io/core/server"
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
	a.Start()
	defer a.Stop()

	store, _, err := New(a, conf.Postgres)
	require.NoError(t, err)

	store.Start(a.BackgroundContext())

	resp := &Result{}
	err = store.Exec(a.BackgroundContext(), "public.test1", make(map[string]string), resp)
	require.NoError(t, err)
	assert.Equal(t, "value", resp.Property)
}
