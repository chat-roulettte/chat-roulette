# Deployment Guide

_Chat Roulette for Slack_ can be run on the free tier of most cloud platforms. This guide will demonstrate how to install the Slack app to your Slack workspace and then deploy the application to [fly.io](https://fly.io).

### Prerequisites

This guide assumes that you have a [fly.io](https://fly.io) account and the `flyctl` command-line utility installed. If you do not have a fly.io account, you can sign up [here](https://fly.io/docs/getting-started/log-in-to-fly/).

You will need sufficient permissions to your Slack workspace to install the _Chat Roulette for Slack_ app. If app approval is enabled on your Slack workspace, you will need to contact a Workspace Owner to [request approval](https://slack.com/help/articles/202035138-Add-apps-to-your-Slack-workspace).


### Install the Slack App

To install the _Chat Roulette for Slack_ app in your Slack workspace, do the following:

1. Choose a unique name for your fly.io app. The app name serves as the subdomain in the URL provided by fly.io and must be passed to the Slack app. For example, choosing `chat-roulette-for-slack` will result in the following URL: `https://chat-roulette-for-slack.fly.dev/`


2. Visit https://api.slack.com/apps and generate an [App Configuration token](https://api.slack.com/authentication/config-tokens):

<img src="./images/generate-access-token.png" alt="Generate Access Token"/>

Copy the access token:

<img src="./images/copy-access-token.png" alt="Copy Access Token"/>


3. Run the [app-manifest-installer](../cmd/app-manifest/main.go) CLI to install the Slack app using the provided [App Manifest](https://api.slack.com/reference/manifests):

```
go run cmd/app-manifest/main.go -u https://YOUR-APP-NAME.fly.dev/ -t xoxe.xoxp-1-SlackAppConfigurationAccessTokenHere
```

or

```
docker run -v $PWD:/mount ghcr.io/chat-roulettte/app-manifest:latest -- -u https://YOUR-APP-NAME.fly.dev/ -t xoxe.xoxp-1-SlackAppConfigurationAccessTokenHere -o mount/config.json
```

4. Refresh your browser to view the new Slack app, then click on it. Scroll down to `Display Information` and click on `Add App Icon` to set the image for the bot:

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

5. Copy the `Bot User OAuth Token`. Open the generated `config.json` file and set the copied bot token for the value `"auth_token"`.

6. On the left-hand menu under `Features`, click on `App Manifest`. After the app is deployed to fly.io, you will need to [verify URL ownership](https://api.slack.com/events/url_verification) by clicking on the button in the banner:

<img src="./images/verify-url-ownership.png" alt="Verify URL ownership"/>


### Prepare Environment Variables

The [app-manifest-installer](../cmd/app-manifest/main.go) CLI generated a sample config file. After deploying the app to fly.io, we will need to set environment variables for the app using values from this config file.

Prepare the environment variables by copying the required values from the config file:

```shell
BOT_AUTH_TOKEN=xoxb-2518545982199-4307457192692-0nhUFFR1lfzWfRKEYRfiv4Z4

DATABASE_ENCRYPTION_KEY=1865fd6c3857574b7865c3822d0a8ecd817637dd2d6e19f2e4248c614dd7b02e

SERVER_CLIENT_ID=2518545982199.4290426097015
SERVER_CLIENT_SECRET=f36d1e7093a9d269bb516b4295ba5cf3
SERVER_REDIRECT_URL=https://YOUR-APP-NAME-HERE.fly.dev/oidc/callback
SERVER_SECRET_KEY=174c9365261bd23832e5c53a850ed2895d824c71cbb8810dd214deaaa46ab44a
SERVER_SIGNING_SECRET=11f574ab154655f81d8601023d7828cb
```

To customize the configuration further, see [configuration.md](./configuration.md).


### Deploy the App

Launch a new fly.io app with attached Postgres database:

```
flyctl launch --name YOUR-APP-NAME-HERE
```

Follow the prompts to select the region and set up a PostgreSQL database. When prompted to deploy now, type `No`.

```
Creating app in /home/username/chat-roulette-for-slack
Scanning source code
Detected a Dockerfile app
Selected App Name: YOUR-APP-NAME-HERE
Automatically selected personal organization: Username
? Select region: ewr (Secaucus, NJ (US))
Created app YOUR-APP-NAME in organization personal
Wrote config file fly.toml
? Would you like to set up a Postgresql database now? Yes
For pricing information visit: https://fly.io/docs/about/pricing/#postgresql-clusters
? Select configuration: Development - Single node, 1x shared CPU, 256MB RAM, 1GB disk
Creating postgres cluster YOUR-APP-NAME-db in organization personal
Postgres cluster YOUR-APP-NAME-db created
  Username:    postgres
  Password:    **************************
  Hostname:    YOUR-APP-NAME-db.internal
  Proxy Port:  5432
  PG Port: 5433
Save your credentials in a secure place -- you won't be able to see them again!

Monitoring Deployment

1 desired, 1 placed, 0 healthy, 0 unhealthy [health checks: 3 total, 1 passing, 2 cri1 desired, 1 placed, 1 healthy, 0 unhealthy [health checks: 3 total, 3 passing]
--> v0 deployed successfully

Connect to postgres
Any app within the personal organization can connect to postgres using the above credentials and the hostname "YOUR-APP-NAME-db.internal."
For example: postgres://postgres:**************************@YOUR-APP-NAME-db.internal:5432

Now that you've set up postgres, here's what you need to understand: https://fly.io/docs/reference/postgres-whats-next/

Postgres cluster YOUR-APP-NAME-db is now attached to YOUR-APP-NAME
The following secret was added to YOUR-APP-NAME:
  DATABASE_URL=postgres://my_chat_roulette:**************************@top2.nearest.of.YOUR-APP-NAME-db.internal:5432/chat_roulette_test4
Postgres cluster YOUR-APP-NAME-db is now attached to YOUR-APP-NAME

? Would you like to set up an Upstash Redis database now? No

? Would you like to deploy now? No
Your app is ready. Deploy with `flyctl deploy`
```

The `DATABASE_URL` environment variable is automatically set by fly.io when the PostgreSQL database is attached to the app.

Set the remaining required environment variables prepared earlier:

```
cat <<EOF | flyctl secrets import
BOT_AUTH_TOKEN=xoxb-2518545982199-4307457192692-0nhUFFR1lfzWfRKEYRfiv4Z4

DATABASE_ENCRYPTION_KEY=1865fd6c3857574b7865c3822d0a8ecd817637dd2d6e19f2e4248c614dd7b02e

SERVER_CLIENT_ID=2518545982199.4290426097015
SERVER_CLIENT_SECRET=f36d1e7093a9d269bb516b4295ba5cf3
SERVER_REDIRECT_URL=https://YOUR-APP-NAME-HERE.fly.dev/oidc/callback
SERVER_SECRET_KEY=174c9365261bd23832e5c53a850ed2895d824c71cbb8810dd214deaaa46ab44a
SERVER_SIGNING_SECRET=11f574ab154655f81d8601023d7828cb
EOF
```

Deploy the app:

```
flyctl deploy -i ghcr.io/chat-roulettte/chat-roulette:latest
```


### Use the App

To enable chat-roulette on a Slack channel, invite the bot to the Slack channel by composing a message with `@chat-roulette-bot`.

To access the UI, visit https://YOUR-APP-NAME-HERE.fly.dev/.
