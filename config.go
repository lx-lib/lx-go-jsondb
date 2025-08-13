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
	Password        string        `mapstructure:"password" validate:"required"`
	DBName          string        `mapstructure:"dbname" validate:"required"`
	MinConnections  int           `mapstructure:"pool_min_conns" validate:"omitempty,min=0"`
	MaxConnections  int           `mapstructure:"pool_max_conns" validate:"required,min=1"`
	MaxConnIdleTime time.Duration `mapstructure:"pool_max_conn_idle_time" validate:"omitempty,min=0"`
	MaxConnLifeTime time.Duration `mapstructure:"pool_max_conn_life_time" validate:"omitempty,min=0"`
	LogLevel        string        `mapstructure:"log_level" validate:"omitempty,oneof=dpanic panic fatal error warn info debug"`
	SslMode         string        `mapstructure:"ssl_mode" validate:"oneof=disable allow prefer require verify-ca verify-full"`
	// LogBytesLimit is the maximum number of bytes to log for a single argument. Default is 512, maximum is 1000000 (1MB).
	LogBytesLimit int `mapstructure:"log_bytes_limit" validate:"omitempty,min=0,max=1000000"`
}

// Validate validate server configuration section.
func (c *Configuration) Validate(validate *validation.Validate) error {
	return validate.Struct(c)
}

// ToConnectionString returns postgres client connection string.
func (c *Configuration) ToConnectionString(appName string) string {
	return fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s&application_name=%s&pool_min_conns=%d&pool_max_conns=%d&pool_max_conn_lifetime=%s&pool_max_conn_idle_time=%s",
		c.UserName,
		url.QueryEscape(c.Password),
		net.JoinHostPort(c.HostName, strconv.FormatInt(int64(c.Port), 10)),
		c.DBName,
		c.SslMode,
		appName,
		c.MinConnections,
		c.MaxConnections,
		c.MaxConnLifeTime.String(),
		c.MaxConnIdleTime.String(),
	)
}

// String returns postgres client connection string with masked password.
func (c *Configuration) String() string {
	return fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s&pool_min_conns=%d&pool_max_conns=%d&pool_max_conn_lifetime=%s&pool_max_conn_idle_time=%s",
		c.UserName,
		strings.Repeat("*", len(c.Password)),
		net.JoinHostPort(c.HostName, strconv.FormatInt(int64(c.Port), 10)),
		c.DBName,
		c.SslMode,
		c.MinConnections,
		c.MaxConnections,
		c.MaxConnLifeTime.String(),
		c.MaxConnIdleTime.String(),
	)
}

// Bind configuration section.
func (c *Configuration) Bind(prefix string, v *viper.Viper) {
	psw, _ := config.LoadRemoteSecret("POSTGRES_PASSWORD")

	v.SetDefault(fmt.Sprintf("%s.port", prefix), 5432)
	v.SetDefault(fmt.Sprintf("%s.password", prefix), psw)
	v.SetDefault(fmt.Sprintf("%s.pool_min_conns", prefix), 0)
	v.SetDefault(fmt.Sprintf("%s.pool_max_conns", prefix), 10)
	v.SetDefault(fmt.Sprintf("%s.pool_max_conn_life_time", prefix), time.Hour)
	v.SetDefault(fmt.Sprintf("%s.pool_max_conn_idle_time", prefix), 30*time.Minute)
	v.SetDefault(fmt.Sprintf("%s.log_level", prefix), "warn")
	v.SetDefault(fmt.Sprintf("%s.ssl_mode", prefix), "prefer")
	v.SetDefault(fmt.Sprintf("%s.log_bytes_limit", prefix), 512)

	_ = v.BindEnv(fmt.Sprintf("%s.hostname", prefix), "POSTGRES_HOST")
	_ = v.BindEnv(fmt.Sprintf("%s.port", prefix), "POSTGRES_PORT")
	_ = v.BindEnv(fmt.Sprintf("%s.username", prefix), "POSTGRES_USER")
	_ = v.BindEnv(fmt.Sprintf("%s.password", prefix), "POSTGRES_PASSWORD")
	_ = v.BindEnv(fmt.Sprintf("%s.dbname", prefix), "POSTGRES_DB")
	_ = v.BindEnv(fmt.Sprintf("%s.pool_min_conns", prefix), "POSTGRES_POOL_MIN_CONNS")
	_ = v.BindEnv(fmt.Sprintf("%s.pool_max_conns", prefix), "POSTGRES_POOL_MAX_CONNS")
	_ = v.BindEnv(fmt.Sprintf("%s.pool_max_conn_life_time", prefix), "POSTGRES_POOL_LIFE_TIME")
	_ = v.BindEnv(fmt.Sprintf("%s.pool_max_conn_idle_time", prefix), "POSTGRES_POOL_IDLE_TIME")
	_ = v.BindEnv(fmt.Sprintf("%s.log_level", prefix), "POSTGRES_LOGLEVEL")
	_ = v.BindEnv(fmt.Sprintf("%s.ssl_mode", prefix), "POSTGRES_SSLMODE")
	_ = v.BindEnv(fmt.Sprintf("%s.log_bytes_limit", prefix), "POSTGRES_LOG_BYTES_LIMIT")
}
