# go-jsondb

<!-- TOC -->

- [go-jsondb](#go-jsondb)
  - [Configuration](#configuration)
  - [Procedure output format](#procedure-output-format)
    - [Helper functions lx/database-util](#helper-functions-lxdatabase-util)
  - [Error handling & rollback](#error-handling--rollback)
    - [Automatic transaction no Begin](#automatic-transaction-no-begin)
    - [Manual transaction Begin](#manual-transaction-begin)
  - [Returning errors from a procedure](#returning-errors-from-a-procedure)
    - [Return error - no rollback](#return-error---no-rollback)
    - [Raise exception - with rollback](#raise-exception---with-rollback)
  - [HTTP status codes](#http-status-codes)
  - [Per-call slow query threshold](#per-call-slow-query-threshold)
  - [Running tests locally](#running-tests-locally)

<!-- /TOC -->

PostgreSQL store that communicates with the database exclusively via stored procedures using JSON parameters.

Each procedure receives a JSON input (`pi_data`) and returns a JSON output (`po_data`):

```sql
create or replace procedure public.my_proc(in pi_data json, inout po_data json)
language plpgsql as $$
begin
  po_data := result_success(json_build_object('id', 1, 'name', 'example'));
end;
$$;
```

`Exec` calls the procedure as `CALL "schema"."proc"($1, $2)` where `$1` is the marshaled input struct and `$2` is `{}`.

---

## Configuration

| Variable | Description | Default | Required |
|---|---|---|---|
| `POSTGRES_HOST` | Host name or IP address | - | Yes |
| `POSTGRES_PORT` | Port number | `5432` | Yes |
| `POSTGRES_USER` | User name | - | Yes |
| `POSTGRES_PASSWORD` | User password | - | Yes |
| `POSTGRES_DB` | Database name | - | Yes |
| `POSTGRES_SSLMODE` | `disable` / `allow` / `prefer` / `require` / `verify-ca` / `verify-full` | `prefer` | Yes |
| `POSTGRES_SSLROOTCERT_FILE` | Path to the Root CA certificate file used to verify the server certificate (e.g. with `verify-ca` / `verify-full`). Must exist when set | - | No |
| `POSTGRES_POOL_MIN_CONNS` | Min pool connections | `0` | No |
| `POSTGRES_POOL_MAX_CONNS` | Max pool connections | `10` | Yes |
| `POSTGRES_POOL_IDLE_TIME` | Max idle time per connection | `30m` | No |
| `POSTGRES_POOL_LIFE_TIME` | Max lifetime per connection | `1h` | No |
| `POSTGRES_LOGLEVEL` | `debug` / `info` / `warn` / `error` / `fatal` / `panic` / `dpanic` | `warn` | No |
| `POSTGRES_SLOW_QUERY_THRESHOLD` | Log queries slower than this (e.g. `500ms`). `0` = disabled | `3s` | No |
| `POSTGRES_DEBUG_QUERY_ARGS` | Log truncated query args at debug/error level. **May log sensitive data.** | `false` | No |
| `POSTGRES_DEBUG_SLOW_QUERY_ARGS` | Log truncated query args for slow queries at warn level. **May log sensitive data.** | `false` | No |
| `POSTGRES_LOG_BYTES_LIMIT` | Max bytes logged per arg (max `1000000`, requires `POSTGRES_DEBUG_QUERY_ARGS` or `POSTGRES_DEBUG_SLOW_QUERY_ARGS` enabled) | `512` | No |

---

## Procedure output format

The procedure **must** return JSON with the following shape:

```json
{ "success": true, "data": { ... } }
```

The `data` field is unmarshaled into the result struct passed to `Exec`.

### Helper functions (`lx/database-util`)

LX projects include the `database-util` submodule which provides two helpers so you never build these objects by hand:

| Function | Returns |
|---|---|
| `result_success(pi_data json)` | `{"success": true, "data": ...}` |
| `result_error(pi_code text, pi_error text)` | `{"code": ..., "error": ...}` - no `data` field |

```sql
-- success
po_data := result_success(json_build_object('id', 1, 'name', 'example'));

-- error without rollback (e.g. validation)
po_data := result_error('user:invalid', 'Invalid input');

-- error with rollback (e.g. after DB changes)
raise exception '%', result_error('user:not_found', 'User not found') using errcode = 'P0001';
```

---

## Error handling & rollback

### Automatic transaction (no `Begin`)

When `Exec` is called without an explicit transaction, the library wraps the call in its own transaction automatically:

- **success** -> commit
- **any error** (query error, unmarshal failure, `success: false`) -> automatic rollback

```go
result := &MyResult{}
err := store.Exec(ctx, "public.my_proc", params, result)
// no transaction management needed
```

### Manual transaction (`Begin`)

Call `Begin` to group multiple operations in one transaction. The caller is fully responsible for commit/rollback. `Exec` detects the existing transaction and does **not** issue a rollback on its own.

```go
tx, err := store.Begin(ctx)
if err != nil { ... }
defer tx.Rollback() // no-op if Commit was already called

result, err := svc.CreateUser(tx, req)
if err != nil { ... } // deferred rollback will fire

if err = tx.Commit(); err != nil { ... }
```

---

## Returning errors from a procedure

There are two distinct patterns depending on whether a rollback is needed.

### Return error - no rollback

Assign `result_error()` directly to `po_data`. The transaction is **not** rolled back. Use this for input validation **before** any data modifications:

```sql
create or replace procedure public.my_proc(in pi_data json, inout po_data json)
language plpgsql as $$
begin
  -- validate first, before touching any data
  if (pi_data->>'id') is null then
    po_data := result_error('user:invalid', 'Missing id');
    return;
  end if;

  -- safe to modify data here
  insert into ...;

  po_data := result_success(json_build_object('id', 1));
end;
$$;
```

### Raise exception - with rollback

Use `raise exception ... using errcode = 'P0001'` when data modifications have already happened and must be rolled back. The exception message is parsed as the full `execResult` JSON, so you can optionally include `data` alongside the error - it will be unmarshaled into the result struct passed to `Exec`:

```sql
  insert into orders ...;

  -- structured error - rolls back, returns ExecError to caller
  raise exception '%', result_error('order:conflict', 'Duplicate order') using errcode = 'P0001';

  -- structured error with data - rolls back and populates the result struct passed to Exec
  raise exception '%', json_build_object(
    'code', 'order:conflict',
    'error', 'Duplicate order',
    'data', json_build_object('conflictingId', 42)
  ) using errcode = 'P0001';
```

Any other `raise exception` (plain text, different errcode) is **not** parsed as `ExecError` - it is wrapped as a generic Go error and will also trigger a rollback:

```sql
  -- plain raise - rolls back, caller gets a generic error (not ExecError)
  raise exception 'Duplicate order';
```

When `P0001` with a JSON message is used, the library returns an `ExecError`. The `data` field (if present) is unmarshaled into the result struct that was passed to `Exec`, **not** into the error:

```go
result := &OrderResult{}
err := store.Exec(ctx, "public.create_order", params, result)

var execErr jsondb.ExecError
if errors.As(err, &execErr) {
  fmt.Println(execErr.Code)          // "order:conflict"
  fmt.Println(execErr.Message)       // "Duplicate order"
  fmt.Println(result.ConflictingID)  // 42 — populated from "data" in the exception
}
```

---

## HTTP status codes

`ExecError` implements `StatusCode() int` - compatible with [azugo](https://azugo.io) error handling. The suffix of `code` determines the HTTP status returned to the client. This applies to **both** error patterns (return and raise):

| `code` suffix | HTTP status |
|---|---|
| ends with `:not_found` | `404 Not Found` |
| anything else | `422 Unprocessable Entity` |

```sql
-- 404 - via return (no rollback)
po_data := result_error('order:not_found', 'Order not found');
return;

-- 404 - via raise (with rollback)
raise exception '%', result_error('order:not_found', 'Order not found') using errcode = 'P0001';

-- 422 - via raise (with rollback)
raise exception '%', result_error('order:invalid_state', 'Order already shipped') using errcode = 'P0001';
```

---

## Per-call slow query threshold

The global `POSTGRES_SLOW_QUERY_THRESHOLD` applies to every query. For individual `Exec` calls where you expect a longer runtime (e.g. heavy reports), wrap the context with `WithSlowQueryThreshold` to override the threshold just for that call:

```go
ctx = jsondb.WithSlowQueryThreshold(ctx, 10*time.Second)
err := store.Exec(ctx, "reporting.generate_report", params, &result)
```

The per-call value takes precedence over the global setting. Other calls are unaffected.

---

## Running tests locally

```sh
docker-compose -f ./test/docker-compose.yaml up --build -d
```

Then run tests via VS Code or `go test ./...`.
