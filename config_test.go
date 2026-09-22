package jsondb

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectionStringSslRootCert(t *testing.T) {
	c := &Configuration{
		HostName:       "localhost",
		Port:           5432,
		UserName:       "user",
		Password:       "pass",
		DBName:         "db",
		MaxConnections: 10,
		SslMode:        "verify-full",
	}

	assert.NotContains(t, c.ToConnectionString("app"), "sslrootcert")
	assert.NotContains(t, c.String(), "sslrootcert")

	c.SslRootCertFile = "/secret/root ca.crt"

	assert.Contains(t, c.ToConnectionString("app"), "sslrootcert=%2Fsecret%2Froot%20ca.crt")
	assert.Contains(t, c.String(), "sslrootcert=%2Fsecret%2Froot%20ca.crt")

	_, err := pgxpool.ParseConfig(c.ToConnectionString("app"))
	require.ErrorContains(t, err, "unable to read CA file: open /secret/root ca.crt")
}

func TestConnectionStringAppName(t *testing.T) {
	c := &Configuration{
		HostName:       "localhost",
		Port:           5432,
		UserName:       "user",
		Password:       "pass",
		DBName:         "db",
		MaxConnections: 10,
		SslMode:        "prefer",
	}

	cfg, err := pgxpool.ParseConfig(c.ToConnectionString("User management API"))
	require.NoError(t, err)

	assert.Equal(t, "User management API", cfg.ConnConfig.RuntimeParams["application_name"])
}
