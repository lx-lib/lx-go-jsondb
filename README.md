# go-jsondb

`go-jsondb` module provides `Store` interface for data stores and `postgres` store implementation for `PostgreSQL` database with common procedure interface having in and out json parameters.

## Usage

### Environment Variables

| Variable | Description | Default | Is Required |
| --- | --- | --- | --- |
| `POSTGRES_HOST` | Database host name or IP address | - | Yes  |
| `POSTGRES_PORT` | Database port number | `5432` | Yes |
| `POSTGRES_USER` | Database user name | - | Yes |
| `POSTGRES_PASSWORD` | Database user password | - | Yes |
| `POSTGRES_DB` | Database name | - | Yes |
| `POSTGRES_POOL_MIN_CONNS` | Minimum number of connections in the pool | `0` | No |
| `POSTGRES_POOL_MAX_CONNS` | Maximum number of connections in the pool | `10` | Yes |
| `POSTGRES_POOL_IDLE_TIME` | Maximum time a connection can be idle in the pool | `30m` | No |
| `POSTGRES_POOL_LIFE_TIME` | Maximum time a connection can be alive in the pool | `1h` | No |
| `POSTGRES_LOGLEVEL` | Log level for database operations. Possible values: `dpanic`, `panic`, `fatal`, `error`, `warn`, `info`, `debug` | `warn` | No |
| `POSTGRES_SSLMODE` | SSL mode for database connection. Possible values: `disable`, `allow`, `prefer`, `require`, `verify-ca`, `verify-full` | `prefer` | Yes |
| `POSTGRES_LOG_BYTES_LIMIT` | Maximum number of bytes to log for a single execution argument on error (active when `POSTGRES_LOGLEVEL` is set to `debug`, maximum value is `1000000`) | `512` | No |
