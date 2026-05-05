package server

import (
	"embed"
	"fmt"
	"html/template"
	"io"
)

//go:embed templates/*.html
var templatesFS embed.FS

func renderPage(w io.Writer, page string, data any) error {
	tpl, err := template.ParseFS(templatesFS, "templates/layout.html", "templates/"+page)
	if err != nil {
		return fmt.Errorf("parse templates: %w", err)
	}
	return tpl.ExecuteTemplate(w, "layout", data)
}

func renderPartial(w io.Writer, partial string, data any) error {
	tpl, err := template.ParseFS(templatesFS, "templates/"+partial)
	if err != nil {
		return fmt.Errorf("parse partial: %w", err)
	}
	return tpl.Execute(w, data)
}
