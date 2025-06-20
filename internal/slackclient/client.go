package slackclient

import (
	"net/http"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	retryablehttp "github.com/hashicorp/go-retryablehttp"
	"github.com/slack-go/slack"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// New creates a Slack client that uses a retryable HTTP client
func New(logger hclog.Logger, token string) (*slack.Client, *http.Client) {
	retryableHTTPClient := retryablehttp.NewClient()
	retryableHTTPClient.RetryMax = 3
	retryableHTTPClient.RetryWaitMin = 10 * time.Millisecond
	retryableHTTPClient.RetryWaitMax = 200 * time.Millisecond
	retryableHTTPClient.Logger = logger.StandardLogger(&hclog.StandardLoggerOptions{
		InferLevels: true,
	})

	httpClient := retryableHTTPClient.StandardClient()
	httpClient.Transport = otelhttp.NewTransport(httpClient.Transport)

	slackClient := slack.New(token, slack.OptionHTTPClient(httpClient))

	return slackClient, httpClient
}
