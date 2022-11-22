package ui

import (
	"embed"
	"html/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/unrolled/render"

	"github.com/chat-roulettte/chat-roulette/internal/templatex"
)

var (
	//go:embed static/* templates/*
	embeddedFS embed.FS

	rend = render.New(render.Options{
		// Load templates from this directory
		Directory: "templates",
		// Use the files embedded in the binary
		FileSystem: &render.EmbedFileSystem{
			FS: embeddedFS,
		},
		Extensions: []string{
			".html",
			".gohtml",
		},
		Funcs: []template.FuncMap{
			{
				"capitalize":         templatex.Capitalize,
				"capitalizeInterval": templatex.CapitalizeInterval,
				"prettyDate":         templatex.PrettyDate,
				"derefBool":          templatex.DerefBool,
			},
			sprig.HtmlFuncMap(),
		},
	})
)
