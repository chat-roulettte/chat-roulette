package isx

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
	"github.com/pkg/errors"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/timex"
	"github.com/chat-roulettte/chat-roulette/internal/tzx"
)

var (
	socialDomains = map[string]string{
		"facebook":  `(?:(www\.)|(m\.))?facebook.com`,
		"github":    `(?:(www\.))?github.com`,
		"instagram": `(?:(www\.)|(m\.))?instagram.com`,
		"linkedin":  `(?:(www\.)|(m\.)|([a-z]{2}\.))?linkedin.com`,
		"pinterest": `(?:(www\.))?pinterest.com`,
		"snapchat":  `(?:(www\.))?snapchat.com`,
		"tiktok":    `(?:(www\.)|(m\.))?tiktok.com`,
		"twitter":   `(?:(www|m)\.)?(x\.com|twitter\.com)`,
		"youtube":   `(?:(www\.)|(m\.))?youtube.com`,
	}
)

// Interval validates that the given value
// is a valid chat-roulette interval.
func Interval(value interface{}) error {
	s, _ := value.(string)

	if _, err := models.ParseInterval(s); err != nil {
		return err
	}

	return nil
}

// Weekday validates that the given value
// is a valid day of the week.
func Weekday(value interface{}) error {
	s, _ := value.(string)

	if _, err := timex.ParseWeekday(s); err != nil {
		return err
	}

	return nil
}

// ConnectionMode validates that the given value
// is a valid chat-roulette connection mode.
func ConnectionMode(value interface{}) error {
	s, _ := value.(string)

	if _, err := models.ParseConnectionMode(s); err != nil {
		return err
	}

	return nil
}

// NextRoundDate validates that the given value
// is a valid date for the next chat-roulette round.
func NextRoundDate(value interface{}) error {
	t, _ := value.(time.Time)

	// The next round date cannot be older than the current date
	now := time.Now().UTC()

	diff := t.Sub(now)
	if diff.Hours() < -36 {
		return fmt.Errorf("invalid next round date: cannot be in the past")
	}

	return nil
}

// Country validates that the given value
// is a valid name of a country.
func Country(value interface{}) error {
	s, _ := value.(string)

	if _, ok := tzx.GetCountryByName(s); !ok {
		return fmt.Errorf("invalid country")
	}

	return nil
}

// ProfileType validates that the given value
// is a valid profile type (eg, Twitter, LinkedIn, etc).
func ProfileType(value interface{}) error {
	s, _ := value.(string)
	s = strings.ToLower(s)

	_, ok := socialDomains[s]
	if !ok {
		return fmt.Errorf("invalid profile type")
	}

	return nil
}

// ValidProfileLink validates that the given
// profile link matches the expected profile type.
func ValidProfileLink(pType string, pLink string) error {
	pType = strings.ToLower(pType)
	pLink = strings.ToLower(pLink)

	domain := socialDomains[pType]

	regex := regexp.MustCompile(fmt.Sprintf(`^(?:https:\/\/)?%s/\S+$`, domain))

	if err := validation.Validate(pLink,
		validation.Required,
		validation.Match(regex).Error("link must match profile type"),
	); err != nil {
		return fmt.Errorf("invalid profile link: %w", err)
	}

	return nil
}

func CalendlyLink(value interface{}) error {
	s, _ := value.(string)

	if s != "" {
		regex := regexp.MustCompile(`(?i)(?:https:\/\/)?calendly.com\/\S+`)

		if err := validation.Validate(s,
			validation.Required,
			validation.Match(regex).Error("URL is malformed"),
		); err != nil {
			return fmt.Errorf("invalid Calendly link")
		}
	}

	return nil
}

// RedirectURL validates that the given value
// is a valid OIDC redirect URL.
func RedirectURL(value interface{}) error {
	s, _ := value.(string)

	regex := regexp.MustCompile(`^https:\/\/\S+\/oidc\/callback$`)

	if err := validation.Validate(s,
		validation.Required,
		is.URL,
		validation.Match(regex).Error("path must contain /oidc/callback"),
	); err != nil {
		return errors.Wrap(err, "invalid redirect URL")
	}

	return nil
}

// SlackBotAuthToken validates that the given value
// is a valid Slack bot auth token.
func SlackBotAuthToken(value interface{}) error {
	s, _ := value.(string)

	regex := regexp.MustCompile(`^xoxb-\d+-\d+-[a-zA-Z0-9]+$`)

	if err := validation.Validate(s,
		validation.Required,
		validation.Match(regex).Error("bot auth token is malformed"),
	); err != nil {
		return errors.Wrap(err, "invalid bot auth token")
	}

	return nil
}

// PostgresConnectionURL validates that the given value
// is a valid PostgreSQL connection URL.
func PostgresConnectionURL(value interface{}) error {
	s, _ := value.(string)

	if err := validation.Validate(s, validation.Required, is.URL); err != nil {
		return errors.Wrap(err, "invalid PostgreSQL connection URL")
	}

	u, err := url.Parse(s)
	if err != nil {
		return err
	}

	var host, port string

	host, port, err = net.SplitHostPort(u.Host)
	if err != nil {
		host = u.Host
	}

	password, _ := u.User.Password()

	type databaseURL struct {
		scheme   string
		user     string
		password string
		host     string
		port     string
		database string
	}

	c := databaseURL{
		scheme:   u.Scheme,
		user:     u.User.Username(),
		password: password,
		host:     host,
		port:     port,
		database: u.Path,
	}

	fieldRules := []*validation.FieldRules{
		validation.Field(&c.scheme, validation.Required, validation.Match(regexp.MustCompile("(?i)^postgres$"))),
		validation.Field(&c.user, validation.Required),
		validation.Field(&c.password, validation.Required),
		validation.Field(&c.host, validation.Required, is.Host),
		validation.Field(&c.port, is.Port),
		validation.Field(&c.database, validation.Required),
	}

	if err := validation.ValidateStruct(&c, fieldRules...); err != nil {
		return errors.Wrap(err, "invalid PostgreSQL connection URL")
	}

	return nil
}
