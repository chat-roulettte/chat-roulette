package bot

import (
	"bytes"
	"embed"
	"text/template"

	"github.com/Masterminds/sprig/v3"

	"github.com/chat-roulettte/chat-roulette/internal/templatex"
)

var (
	// templates holds the Slack message templates
	//go:embed templates/*
	tmplFS embed.FS

	// funcMap is a map of custom template functions
	funcMap = template.FuncMap{
		"capitalize": templatex.Capitalize,
		"prettyDate": templatex.PrettierDate,
	}

	templates = template.New("custom").Funcs(funcMap).Funcs(sprig.TxtFuncMap())
)

// renderTemplate renders a template using the supplied filename and data
func renderTemplate(filename string, data interface{}) (string, error) {
	var b bytes.Buffer

	t, err := templates.ParseFS(tmplFS, "templates/*")

	if err != nil {
		return "", err
	}

	if err := t.ExecuteTemplate(&b, filename, data); err != nil {
		return "", err
	}

	return b.String(), nil
}
