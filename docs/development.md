# Development

### Tech Stack

* Go
* PostgreSQL
* Vanilla JS
* Tailwind CSS
* OpenTelemetry
* Slack

## Design Philosophy

_Chat Roulette for Slack_ was designed to be able to run on the free tier of the various PaaS providers such as [Heroku](https://www.heroku.com/), [fly.io](https://fly.io/), [render](https://render.com/), etc. The compiled binary is completely standalone with all required files baked into it using [embed.FS](https://pkg.go.dev/embed). The only dependency is PostgreSQL. The application is horizontally scalable.

An effort was made to limit storing any personally identifiable information in the database. Only Slack identifiers (eg, `U0123456789`) are saved and not display names (eg, `@bincyber`). For privacy and security reasons, all user data is encrypted at rest using [Column-level encryption](https://en.wikipedia.org/wiki/Column_level_encryption) with 256-bit AES in Galois/Counter Mode (GCM).

### Components

The application consists of two components:

1. a web server which receives and handles [events](https://api.slack.com/apis/connections/events-api) and [interactions](https://api.slack.com/interactivity/handling) from Slack

2. a background worker which processes [jobs](../internal/bot/) in a PostgreSQL queue

<details>
  <summary>Database Schema</summary>

<img src="./diagrams/database-schema.png" alt="Database Schema"/>

</details>

## Local Development

### Install Dependencies

For local development, [docker](https://docs.docker.com/engine/install/) and [ngrok](https://ngrok.com/download) need to be installed. Docker is used to run the PostgreSQL server. ngrok is used to create a HTTPS tunnel enabling Slack to send webhooks to the application.

The following tools are also required:
* [golangci-lint](https://github.com/golangci/golangci-lint)
* [migrate](https://github.com/golang-migrate/migrate)
* [gotestsum](https://github.com/gotestyourself/gotestsum)

To install these tools to `$GOPATH/bin`, run the following command:

```
make go/tools
```

### Start ngrok

Start an https tunnel using ngrok with the following command:

```
make ngrok
```

Copy the generated forwarding URL:

```
ngrok by @inconshreveable                                                                     (Ctrl+C to quit)

Session Status                online
Account                       bincyber (Plan: Free)
Version                       2.3.40
Region                        United States (us)
Web Interface                 http://127.0.0.1:4040
Forwarding                    https://c486-174-114-137-56.ngrok.io -> http://localhost:8080
```

### Install the Slack App

To install the Chat Roulette for Slack app in your Slack workspace, do the following:

1. Visit https://api.slack.com/apps and generate an [App Configuration token](https://api.slack.com/authentication/config-tokens):

<img src="./images/generate-access-token.png" alt="Generate Access Token"/>

Copy the access token:

<img src="./images/copy-access-token.png" alt="Copy Access Token"/>

2. Run the app-manifest-installer CLI to install the Slack app using the provided App Manifest:

```
go run cmd/app-manifest/main.go -u https://YOUR-NGROK-SUBDOMAIN-HERE.ngrok.io -t xoxe.xoxp-1-SlackAppConfigurationAccessTokenHere
```

3. Refresh your browser to view the new Slack app, then click on it. Scroll down to `Display Information` and click on `Add App Icon` to set the image for the bot:

<p align="center">
<img src="./images/add-app-icon.png" alt="Add App Icon" height="500px" width="600px"/>
</p>

Upload the image in [docs/images/bot-icon.png](./images/bot-icon.png).

4. On the left-hand menu under `Features`, click on `OAuth & Permissions`. Click the `Install to Workspace` button to install the _Chat Roulette for Slack_ app to your Slack Workspace.

<img src="./images/oauth-and-permissions.png" alt="Oauth & Permissions"/>

You will need to accept the requested permissions:

<p align="center">
<img src="./images/requesting-permissions.png" alt="Requesting Permissions" height="500px" width="500px"/>
</p>

**Note**: You will need sufficient permissions on your Slack workspace to install the app and complete the above step.

5. Copy the `Bot User OAuth Token`. Open the `config.json` file and set the copied bot token for the value `"auth_token"`.

6. On the left-hand menu under `Features`, click on `App Manifest`. After the app is deployed to fly.io, you will need to [verify URL ownership](https://api.slack.com/events/url_verification) by clicking on the button in the banner.


### Launch the Development Environment

To bring up the development environment:

```
make dev/up
```

This will start the PostgreSQL server in a Docker container, execute database migration, and run the Go app on your machine.

To restart the application, press `Ctrl+C` and then:

```
make go/run
```

To terminate the development environment:

```
make dev/destroy
```

### Enabling Tracing

This application is instrumented with [OpenTelemetry](https://opentelemetry.io/) to emit traces to [Honeycomb](https://www.honeycomb.io/) or [Jaeger](https://www.jaegertracing.io/).

Tracing can be enabled by adding the following settings to the config file:

#### Honeycomb

```
  "tracing": {
    "enabled": true,
    "exporter": "honeycomb",
    "honeycomb": {
      "team": "honeycomb-api-key-here",
      "dataset": "chat-roulette"
    }
  }
```

#### Jaeger

```
  "tracing": {
    "enabled": true,
    "exporter": "jaeger",
    "jaeger": {
      "endpoint": "http://example.com:14268/api/traces"
    }
  }
```

## Testing

To run the test suite:

```
make go/testsum
```

Some tests use [ory/dockertest](https://github.com/ory/dockertest) which requires access to the Docker socket (ie, `/var/run/docker.sock`).
