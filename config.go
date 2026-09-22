package jsondb

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"azugo.io/core/config"
	"azugo.io/core/validation"
	"github.com/spf13/viper"
)

// Configuration represents PostgreSQL client configuration section.
type Configuration struct {
	HostName        string        `mapstructure:"hostname" validate:"ip4_addr|ip6_addr|hostname|fqdn"`
	Port            int           `mapstructure:"port" validate:"required,min=1,max=65535"`
	UserName        string        `mapstructure:"username" validate:"required"`
	Password        string        `mapstructure:"password" validate:"required"` //nolint:gosec // intentionally holds a password for database configuration
	DBName          string        `mapstructure:"dbname" validate:"required"`
	MinConnections  int           `mapstructure:"pool_min_conns" validate:"omitempty,min=0"`
	MaxConnections  int           `mapstructure:"pool_max_conns" validate:"required,min=1"`
	MaxConnIdleTime time.Duration `mapstructure:"pool_max_conn_idle_time" validate:"omitempty,min=0"`
	MaxConnLifeTime time.Duration `mapstructure:"pool_max_conn_life_time" validate:"omitempty,min=0"`
	LogLevel        string        `mapstructure:"log_level" validate:"omitempty,oneof=dpanic panic fatal error warn info debug"`
	SslMode         string        `mapstructure:"ssl_mode" validate:"oneof=disable allow prefer require verify-ca verify-full"`
	SslRootCertFile string        `mapstructure:"ssl_root_cert_file" validate:"omitempty,file"`
	// LogBytesLimit is the maximum number of bytes to log for a single argument. Default is 512, maximum is 1000000 (1MB).
	LogBytesLimit int `mapstructure:"log_bytes_limit" validate:"omitempty,min=0,max=1000000"`
	// SlowQueryThreshold is the duration after which a query is considered slow and will be logged. Default is 3 seconds.
	SlowQueryThreshold time.Duration `mapstructure:"slow_query_threshold" validate:"omitempty,min=0"`
	// DebugQueryArgs enables logging of query arguments for all queries. WARNING: This may log sensitive data. Use only in development/debugging.
	DebugQueryArgs bool `mapstructure:"debug_query_args"`
	// DebugSlowQueryArgs enables logging of query arguments for slow queries only. WARNING: This may log sensitive data. Use only in development/debugging.
	DebugSlowQueryArgs bool `mapstructure:"debug_slow_query_args"`
}

// Validate validate server configuration section.
func (c *Configuration) Validate(validate *validation.Validate) error {
	return validate.Struct(c)
}

// escape escapes a connection string value. url.QueryEscape encodes spaces as
// "+", which pgx v5.11+ no longer decodes back to a space.
func escape(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

// ToConnectionString returns postgres client connection string.
func (c *Configuration) ToConnectionString(appName string) string {
	return fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s%s&application_name=%s&pool_min_conns=%d&pool_max_conns=%d&pool_max_conn_lifetime=%s&pool_max_conn_idle_time=%s",
		c.UserName,
		escape(c.Password),
		net.JoinHostPort(c.HostName, strconv.FormatInt(int64(c.Port), 10)),
		c.DBName,
		c.SslMode,
		c.sslRootCertParam(),
		escape(appName),
		c.MinConnections,
		c.MaxConnections,
		c.MaxConnLifeTime.String(),
		c.MaxConnIdleTime.String(),
	)
}

func (c *Configuration) sslRootCertParam() string {
	if c.SslRootCertFile == "" {
		return ""
	}

	return "&sslrootcert=" + escape(c.SslRootCertFile)
}

// String returns postgres client connection string with masked password.
func (c *Configuration) String() string {
	return fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s%s&pool_min_conns=%d&pool_max_conns=%d&pool_max_conn_lifetime=%s&pool_max_conn_idle_time=%s",
		c.UserName,
		strings.Repeat("*", len(c.Password)),
		net.JoinHostPort(c.HostName, strconv.FormatInt(int64(c.Port), 10)),
		c.DBName,
		c.SslMode,
		c.sslRootCertParam(),
		c.MinConnections,
		c.MaxConnections,
		c.MaxConnLifeTime.String(),
		c.MaxConnIdleTime.String(),
	)
}

// Bind configuration section.
func (c *Configuration) Bind(prefix string, v *viper.Viper) {
	psw, _ := config.LoadRemoteSecret("POSTGRES_PASSWORD")

	v.SetDefault(prefix+".port", 5432)
	v.SetDefault(prefix+".password", psw)
	v.SetDefault(prefix+".pool_min_conns", 0)
	v.SetDefault(prefix+".pool_max_conns", 10)
	v.SetDefault(prefix+".pool_max_conn_life_time", time.Hour)
	v.SetDefault(prefix+".pool_max_conn_idle_time", 30*time.Minute)
	v.SetDefault(prefix+".log_level", "warn")
	v.SetDefault(prefix+".ssl_mode", "prefer")
	v.SetDefault(prefix+".log_bytes_limit", 512)
	v.SetDefault(prefix+".slow_query_threshold", 3*time.Second)
	v.SetDefault(prefix+".debug_query_args", false)
	v.SetDefault(prefix+".debug_slow_query_args", false)

	_ = v.BindEnv(prefix+".hostname", "POSTGRES_HOST")
	_ = v.BindEnv(prefix+".port", "POSTGRES_PORT")
	_ = v.BindEnv(prefix+".username", "POSTGRES_USER")
	_ = v.BindEnv(prefix+".password", "POSTGRES_PASSWORD")
	_ = v.BindEnv(prefix+".dbname", "POSTGRES_DB")
	_ = v.BindEnv(prefix+".pool_min_conns", "POSTGRES_POOL_MIN_CONNS")
	_ = v.BindEnv(prefix+".pool_max_conns", "POSTGRES_POOL_MAX_CONNS")
	_ = v.BindEnv(prefix+".pool_max_conn_life_time", "POSTGRES_POOL_LIFE_TIME")
	_ = v.BindEnv(prefix+".pool_max_conn_idle_time", "POSTGRES_POOL_IDLE_TIME")
	_ = v.BindEnv(prefix+".log_level", "POSTGRES_LOGLEVEL")
	_ = v.BindEnv(prefix+".ssl_mode", "POSTGRES_SSLMODE")
	_ = v.BindEnv(prefix+".ssl_root_cert_file", "POSTGRES_SSLROOTCERT_FILE")
	_ = v.BindEnv(prefix+".log_bytes_limit", "POSTGRES_LOG_BYTES_LIMIT")
	_ = v.BindEnv(prefix+".slow_query_threshold", "POSTGRES_SLOW_QUERY_THRESHOLD")
	_ = v.BindEnv(prefix+".debug_query_args", "POSTGRES_DEBUG_QUERY_ARGS")
	_ = v.BindEnv(prefix+".debug_slow_query_args", "POSTGRES_DEBUG_SLOW_QUERY_ARGS")
}
