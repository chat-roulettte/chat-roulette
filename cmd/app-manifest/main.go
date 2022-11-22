package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"time"

	"github.com/akamensky/argparse"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/spf13/viper"
	"sigs.k8s.io/yaml"
)

const (
	// configFilePath is the path to the example app config file
	configFilePath = "docs/examples/config.example.json"

	// slackAppManifestTemplatePath is the path to the Slack App Manifest yaml file
	slackAppManifestTemplatePath = "docs/examples/manifest.yaml"

	// slackAppManifestCreateURL is the URL for the apps.manifest.create API
	// See: https://api.slack.com/methods/apps.manifest.create
	slackAppManifestCreateURL = "https://slack.com/api/apps.manifest.create"
)

type appManifestTemplate struct {
	BaseURL string
}

type appsManifestCreateRequest struct {
	Manifest string `json:"manifest"`
}

type appsManifestCreateResponse struct {
	Ok          bool                `json:"ok"`
	Error       string              `json:"error,omitempty"`
	AppID       string              `json:"app_id,omitempty"`
	Credentials slackAppCredentials `json:"credentials,omitempty"`
}

type slackAppCredentials struct {
	ClientID      string `json:"client_id"`
	ClientSecret  string `json:"client_secret"`
	SigningSecret string `json:"signing_secret"`
}

func generateKey() string {
	b := make([]byte, 32)
	io.ReadFull(rand.Reader, b) //nolint:errcheck
	return hex.EncodeToString(b)
}

func main() {
	if err := exec(); err != nil {
		os.Exit(1)
	}
}

func exec() error {
	// Create the logger
	logger := hclog.New(&hclog.LoggerOptions{
		Level:           hclog.Info,
		Output:          os.Stdout,
		IncludeLocation: true,
		JSONFormat:      false,
		Color:           hclog.AutoColor,
	})

	// argparse is used for the CLI
	parser := argparse.NewParser(
		"app-manifest-installer",
		"installs the Chat Roulette for Slack app into your Slack workspace using an App Manifest",
	)

	// Define flags for the CLI
	accessToken := parser.String("t", "token", &argparse.Options{
		Required: true,
		Validate: func(args []string) error {
			s := args[0]

			regex := regexp.MustCompile(`xoxe\.xoxp-\d-.*`)

			if err := validation.Validate(s,
				validation.Required,
				validation.Match(regex),
			); err != nil {
				return fmt.Errorf("must be in the following format: xoxe.xoxp-1-RANDOMSTRINGHERE")
			}

			return nil
		},
		Help: "the Slack App Configuration access token",
	})

	baseURL := parser.String("u", "url", &argparse.Options{
		Required: true,
		Validate: func(args []string) error {
			s := args[0]

			err := validation.Validate(s,
				validation.Required,
				is.URL,
			)

			return err
		},
		Help: "the base URL to receive Slack events and interactions",
	})

	outputPath := parser.File("o", "output", os.O_RDWR|os.O_CREATE, 0644, &argparse.Options{
		Required: false,
		Default:  "config.json",
		Help:     "the path to output the generated starter config",
	})

	if err := parser.Parse(os.Args); err != nil {
		logger.Error("failed to evaluate required command-line flags", "error", err)
		return err
	}

	// Template the App Manifest
	templateParams := appManifestTemplate{
		BaseURL: *baseURL,
	}

	b := new(bytes.Buffer)

	t, err := template.ParseFiles(slackAppManifestTemplatePath)

	if err != nil {
		logger.Error("failed to parse template file", "error", err)
		return err
	}

	if err := t.Execute(b, templateParams); err != nil {
		logger.Error("failed to execute template", "error", err)
		return err
	}

	// Marshal the template to JSON
	content, err := yaml.YAMLToJSON(b.Bytes())
	if err != nil {
		logger.Error("failed to convert yaml to json", "error", err)
		return err
	}

	// Create the new Slack app from the App Manifest
	logger.Info("creating new Slack app from App Manifest")

	httpClient := retryablehttp.NewClient().StandardClient()

	createRequest := &appsManifestCreateRequest{
		Manifest: string(content),
	}

	body := new(bytes.Buffer)

	if err := json.NewEncoder(body).Encode(createRequest); err != nil {
		logger.Error("failed to encode request body as JSON", "error", err)
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, slackAppManifestCreateURL, body)
	if err != nil {
		logger.Error("failed to create http request", "error", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *accessToken))

	resp, err := httpClient.Do(req)
	if err != nil {
		logger.Error("failed to send http request", "error", err)
		return err
	}
	defer resp.Body.Close()

	var createResponse appsManifestCreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResponse); err != nil {
		logger.Error("failed to read HTTP response as JSON", "error", err)
		return err
	}

	if !createResponse.Ok {
		logger.Error("failed to create Slack app from App Manifest", "error", createResponse.Error)
		return err
	}

	logger.Info("created Slack app from App Manifest")

	// Generate the starter config for the Chat Roulette for Slack app
	logger.Info("generating starter config for Chat Roulette for Slack app")

	v := viper.New()
	v.SetConfigFile(configFilePath)

	if err := v.ReadInConfig(); err != nil {
		logger.Error("failed to generate sample config: unable to load sample config", "error", err)
		return err
	}

	redirectURL, _ := url.Parse(*baseURL)
	redirectURL.Path = path.Join(redirectURL.Path, "/oidc/callback")

	secretKey := generateKey()
	encryptionKey := generateKey()

	v.Set("server.client_id", createResponse.Credentials.ClientID)
	v.Set("server.client_secret", createResponse.Credentials.ClientSecret)
	v.Set("server.signing_secret", createResponse.Credentials.SigningSecret)
	v.Set("server.redirect_url", redirectURL.String())
	v.Set("server.secret_key", secretKey)
	v.Set("database.encryption.key", encryptionKey)

	if err := v.WriteConfigAs(outputPath.Name()); err != nil {
		logger.Error("failed to write...", "error", err)
		return err
	}

	outputAbsPath, _ := filepath.Abs(outputPath.Name())

	logger.Info(fmt.Sprintf("generated sample config file for Chat Roulette for Slack app at: %s", outputAbsPath))

	// Print message to help user complete setup
	logger.Info(fmt.Sprintf("browse to https://api.slack.com/apps/%s to complete setup of the Slack app", createResponse.AppID))

	return nil
}
