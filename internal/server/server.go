package server

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/dgraph-io/ristretto"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/hashicorp/go-hclog"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/oauth2"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/bot"
	"github.com/chat-roulettte/chat-roulette/internal/config"
	"github.com/chat-roulettte/chat-roulette/internal/database"
	"github.com/chat-roulettte/chat-roulette/internal/slackclient"
)

const (
	// SessionKey is the name for the HTTP cookie containing the session
	SessionKey = "GOSESSION"

	// SlackOpenIDConnectURL is the base URL for Slack's OIDC discovery
	SlackOpenIDConnectURL = "https://slack.com"
)

// Server is the API server
type Server struct {
	cache           *ristretto.Cache
	config          *config.Config
	db              *gorm.DB
	devMode         bool
	httpClient      *http.Client
	httpServer      *http.Server
	idTokenVerifier *oidc.IDTokenVerifier
	mux             *mux.Router
	oauthConfig     *oauth2.Config
	sessionStore    *sessions.CookieStore
	slackBotUserID  string
	slackClient     *slack.Client
}

type ServerOptions struct {
	Config         *config.Config
	DB             *gorm.DB
	DevMode        bool
	HTTPClient     *http.Client
	HTTPServer     *http.Server
	SlackBotUserID string
	SlackClient    *slack.Client
	SessionsStore  *sessions.CookieStore
}

// New creates a new API server
func New(ctx context.Context, logger hclog.Logger, c *config.Config) (*Server, error) {
	// Start new span
	tracer := otel.Tracer("")
	ctx, span := tracer.Start(ctx, "server.create")
	defer span.End()

	// Configure gorm.DB
	db, err := database.CreateGormDB(logger, c)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create gorm.DB")
	}

	// Create HTTP server with tracing
	r := mux.NewRouter()
	r.Use(otelmux.Middleware("chat-roulette-server"))

	httpServer := &http.Server{
		Addr:         c.GetAddr(),
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      r,
		BaseContext: func(net.Listener) context.Context {
			return hclog.WithContext(context.Background(), logger)
		},
	}

	// Create Slack client
	slackClient, httpClient := slackclient.New(logger, c.Bot.AuthToken)

	// Retrieve the user_id of the chat-roulette Slack bot
	logger.Debug("retrieving the user ID of the Slack bot")
	slackBotUserID, err := bot.GetBotUserID(ctx, slackClient)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve the user ID of the Slack bot")
	}
	logger.Debug("retrieved the user_id of the Slack bot", "slack_bot_user_id", slackBotUserID)
	span.SetAttributes(
		attribute.String("slack_bot_user_id", slackBotUserID),
	)

	// Discover the OpenID Connect provider configuration for Slack
	logger.Debug("discovering Slack OIDC provider")

	// Set our custom retryable http client in the context
	ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)

	// 2 second timeout should be sufficient for this operation
	rCtx, cancel := context.WithTimeout(ctx, 2000*time.Millisecond)
	defer cancel()

	provider, err := oidc.NewProvider(rCtx, SlackOpenIDConnectURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to discover Slack OIDC provider")
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: c.Server.ClientID})

	oauthConfig := &oauth2.Config{
		RedirectURL:  c.Server.RedirectURL,
		ClientID:     c.Server.ClientID,
		ClientSecret: c.Server.ClientSecret,
		Scopes:       []string{oidc.ScopeOpenID, "profile"},
		Endpoint:     provider.Endpoint(),
	}

	// Cookie-based session store
	secretKey, err := c.Server.GetSecretKey()
	if err != nil {
		return nil, errors.Wrap(err, "failed to configure secret key for cookie-based session store")
	}
	store := sessions.NewCookieStore(secretKey)
	store.Options.MaxAge = 604800 // 7 days

	// In-memory cache
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1000,
		MaxCost:     200,
		BufferItems: 64,
	})

	if err != nil {
		return nil, errors.Wrap(err, "failed to configure in-memory cache")
	}

	// Construct the Server
	s := &Server{
		cache:           cache,
		config:          c,
		db:              db,
		devMode:         c.Dev,
		httpClient:      httpClient,
		httpServer:      httpServer,
		idTokenVerifier: verifier,
		mux:             r,
		oauthConfig:     oauthConfig,
		sessionStore:    store,
		slackBotUserID:  slackBotUserID,
		slackClient:     slackClient,
	}

	return s, nil
}

// NewTestServer returns a Server suitable for use in tests
func NewTestServer(opts *ServerOptions) *Server {
	if opts == nil {
		opts = &ServerOptions{}
	}

	s := &Server{
		config:         opts.Config,
		db:             opts.DB,
		devMode:        opts.DevMode,
		httpClient:     opts.HTTPClient,
		httpServer:     opts.HTTPServer,
		slackBotUserID: opts.SlackBotUserID,
		slackClient:    opts.SlackClient,
		sessionStore:   opts.SessionsStore,
	}

	return s
}

// IsDevMode returns true if the Server is running in Dev Mode
func (s *Server) IsDevMode() bool {
	return s.devMode
}

// GetDB retrieves the gorm DB
func (s *Server) GetDB() *gorm.DB {
	return s.db
}

// GetHTTPClient retrieves the http client
func (s *Server) GetHTTPClient() *http.Client {
	return s.httpClient
}

// GetMux retrieves the router
func (s *Server) GetMux() *mux.Router {
	return s.mux
}

// GetCache retrieves the in-memory cache
func (s *Server) GetCache() *ristretto.Cache {
	return s.cache
}

// GetSlackClient retrieves the Slack client
func (s *Server) GetSlackClient() *slack.Client {
	return s.slackClient
}

// GetSlackSigningSecret retrieves the Slack signing secret from the config
func (s *Server) GetSlackSigningSecret() string {
	return s.config.Server.SigningSecret
}

// GetSlackBotUserID returns the user_id of the chat roulette Slack bot
func (s *Server) GetSlackBotUserID() string {
	return s.slackBotUserID
}

// GetBaseURL returns the base URL of the server
func (s *Server) GetBaseURL() string {
	u, _ := url.Parse(s.config.Server.RedirectURL)
	u.Path = ""
	return u.String()
}

// GenerateAuthCodeURL generates the OIDC URL for Single Sign On with Slack
func (s *Server) GenerateAuthCodeURL(state, nonce string) string {
	return s.oauthConfig.AuthCodeURL(state, oidc.Nonce(nonce))
}

// FetchIDToken exchanges an authorization code for a OIDC token
func (s *Server) FetchIDToken(ctx context.Context, code string) (*oauth2.Token, error) {
	// Set our custom retryable http client in the context
	ctx = context.WithValue(ctx, oauth2.HTTPClient, s.httpClient)

	ctx, cancel := context.WithTimeout(ctx, 2000*time.Millisecond)
	defer cancel()

	return s.oauthConfig.Exchange(ctx, code)
}

// VerifyIDToken verifies that a raw ID token is valid
func (s *Server) VerifyIDToken(ctx context.Context, token string) (*oidc.IDToken, error) {
	// Set our custom retryable http client in the context
	ctx = context.WithValue(ctx, oauth2.HTTPClient, s.httpClient)

	ctx, cancel := context.WithTimeout(ctx, 2000*time.Millisecond)
	defer cancel()

	return s.idTokenVerifier.Verify(ctx, token)
}

// GetSession retrieves an existing or new session
func (s *Server) GetSession(r *http.Request) (*sessions.Session, error) {
	return s.sessionStore.Get(r, SessionKey)
}
