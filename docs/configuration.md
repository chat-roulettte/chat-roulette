# Configuration

_Chat Roulette for Slack_ supports configuration via a JSON config file and/or environment variables. Environment variables take precedence over whatever is in the config file.

See [config.example.json](./examples/config.example.json) for an example of how to modify the configuration of the app using a JSON config file.


#### Bot Config

| Key | Environment Variable | Type | Required | Default Value | Description
| -------- | -------- | -------- | :--------: | -------- | ------
| `auth_token` | `BOT_AUTH_TOKEN` | String | Yes |  | The Slack OAuth2 bot token used by the bot when making API calls to Slack.

###### JSON

```json
{
    "bot": {
        "auth_token": "xoxb-9876543210123-4567778889990-f0A2GclR80dgPZLTUEq5asHm"
    }
}
```

#### Database Config

| Key | Environment Variable | Type | Required | Default Value | Description
| -------- | -------- | -------- | :--------: | -------- | ------
| `url` | `DATABASE_URL` | String | Yes |  | The PostgreSQL connection URL in the form: <br /><br />`postgres://username:password@host:5432/database-name`
| `key` | `DATABASE_ENCRYPTION_KEY` | String | Yes |  | The 32-byte hex-encoded encryption key. This is used to encrypt sensitive data stored in the database. <br /><br />Use `make generate/key` to generate a random key.
| `max_open` | `DATABASE_CONNECTIONS_MAX_OPEN` | Integer | No | `20` | The maximum number of open connections.
| `max_idle` | `DATABASE_CONNECTIONS_MAX_IDLE` | Integer | No | `10` | The maximum number of idle connections.
| `max_lifetime` | `DATABASE_CONNECTIONS_MAX_LIFETIME` | String | No | `60m` | The maximum lifetime of connections. This must be a valid [duration string](https://pkg.go.dev/time#ParseDuration).
| `max_idletime` | `DATABASE_CONNECTIONS_MAX_IDLETIME` | String | No | `15m` | The maximum amount of time a connection can be idle. This must be a valid [duration string](https://pkg.go.dev/time#ParseDuration).

###### JSON
```json
{
    "database": {
        "url": "postgres://postgres:complex-password-here@db.example.com:5432/chat-roulette",
        "encryption": {
            "key": "01234abcde5678901234f62c898cdb592eb3166b56da733e8e798305b0ef6403"
        },
        "connections": {
            "max_open": 50,
            "max_idle": 20,
            "max_lifetime": "2h",
            "max_idletime": "20m"
        }
    }
}
```


#### Server Config

| Key | Environment Variable | Type | Required | Default Value | Description
| -------- | -------- | -------- | -------- | -------- | ------
| `address` | `SERVER_ADDRESS` | String | No | `0.0.0.0` | The address that the HTTP server binds on.
| `port` | `SERVER_ADDRESS` | String | No | `8080` | The TCP port that the HTTP server binds on.
| `client_id` | `SERVER_CLIENT_ID` | String | Yes |  | The Slack OpenID Connect (OIDC) Client ID to support _Sign in with Slack_ for the UI. See [here](https://api.slack.com/authentication/sign-in-with-slack).
| `client_secret` | `SERVER_CLIENT_SECRET` | String | Yes |  | The Slack OIDC Client secret.
| `redirect_url` | `SERVER_REDIRECT_URL` | String | Yes |  | The Slack OIDC redirect URL. Must include the path `/oidc/callback`.
| `secret_key` | `SERVER_SECRET_KEY` | String | Yes |  | The 32-byte hex-encoded secret key used to authenticate cookies.<br /><br />Use `make generate/key` to generate a random key.
| `signing_secret` | `SERVER_SIGNING_SECRET` | String | Yes |  | The Slack-provided secret used to authenticate requests received from Slack.


###### JSON
```json
{
    "server": {
        "client_id": "2518545982190.4321012345789",
        "client_secret": "9f8e7dbc9a5ba2aa4522c7c8eb571f60",
        "redirect_url": "https://www.example.com/oidc/callback",
        "secret_key": "8c4faf836e29d282f2dc7ffdf4ef59c6081e2d8964ba0ac9cd4bc8800021300c",
        "signing_secret": "2773fb7eb76c90f19c0e1504ae1eee4b"
    }
}
```

#### Worker Config

| Key | Environment Variable | Type | Required | Default Value | Description
| -------- | -------- | -------- | -------- | -------- | ------
| `concurrency` | `WORKER_CONCURRENCY` | Integer | No | [runtime.NumCPU()](https://pkg.go.dev/runtime#NumCPU) | The number of concurrent workers to run.<br /><br />This defaults to the number of logical CPUs usable by the current process.

###### JSON
```json
{
    "worker": {
        "concurrency": 2
    }
}
```


#### Tracing Config

| Key | Environment Variable | Type | Required | Default Value | Description
| -------- | -------- | -------- | -------- | -------- | ------
| `enabled` | `TRACING_ENABLED` | Boolean | No | `false`  | Setting this to true enables tracing using [OpenTelemetry](https://opentelemetry.io/).
| `exporter` | `TRACING_EXPORTER` | String | No |  | The tracing exporter to use. This must be set if tracing is enabled. <br /><br />Options: <ul><li>`honeycomb`</li><li>`jaeger`</li></ul>
| `endpoint` | `TRACING_JAEGER_ENDPOINT` | String | No |  | The URL of the Jaeger OTLP HTTP collector. This must be set if the tracing exporter is set to `jaeger`.
| `team` | `TRACING_HONEYCOMB_TEAM` | String | No |  | The [honeycomb.io](https://www.honeycomb.io/) API key. This must be set if the tracing exporter is set to `honeycomb`.
| `dataset` | `TRACING_HONEYCOMB_DATASET` | String | No |  | The dataset to send traces to. This must be set if the tracing exporter is set to `honeycomb`.
| - | `OTEL_TRACES_SAMPLER` | String | No | `always_on` | Configure the sampling strategy to be used.<br /><br />Options: <ul><li>`always_on`</li><li>`traceidratio`</li><li>`parentbased_traceidratio`</li></ul><br />Refer to: [General SDK Configuration](https://opentelemetry.io/docs/languages/sdk-configuration/general/#otel_traces_sampler)
| - | `OTEL_TRACES_SAMPLER_ARG` | String | No |  | Configure additional arguments for the sampler<br />Refer to: [General SDK Configuration](https://opentelemetry.io/docs/languages/sdk-configuration/general/#otel_traces_sampler_arg)

###### JSON
```json
{
    "tracing": {
        "enabled": true,
        "exporter": "honeycomb",
        "honeycomb": {
            "team": "abcdef01234567899876543210a1b3c4",
            "dataset": "chat-roulette"
        }
    }
}
```

```json
{
    "tracing": {
        "enabled": true,
        "exporter": "jaeger",
        "jaeger": {
            "endpoint": "http://localhost:4318/v1/traces"
        }
    }
}
```


#### Misc Config

| Key | Environment Variable | Type | Required | Default Value | Description
| -------- | -------- | -------- | -------- | -------- | ------
| `dev` | `DEV` | Boolean | No | `false` | Set to `true` to enable development mode. <br /><br />Development mode disables verifying requests received from Slack. This should not be enabled for production deployments.

###### JSON
```json
{
    "dev": false
}
```
