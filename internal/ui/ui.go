package ui

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"time"
)

//go:embed templates/*.html static/**
var files embed.FS

type Renderer struct {
	pages map[string]*template.Template
}

func NewRenderer() (*Renderer, error) {
	renderer := &Renderer{pages: make(map[string]*template.Template)}
	for _, page := range []string{"home", "rules", "memo", "booking", "thanks"} {
		parsed, err := template.New(page).Funcs(template.FuncMap{
			"year": func() int { return time.Now().Year() },
			"mod":  func(a, b int) int { return a % b },
		}).ParseFS(files,
			"templates/base.html",
			"templates/components.html",
			"templates/"+page+".html",
		)
		if err != nil {
			return nil, err
		}
		renderer.pages[page] = parsed
	}

	return renderer, nil
}

func (r *Renderer) Render(w http.ResponseWriter, page string, data any) error {
	tmpl, ok := r.pages[page]
	if !ok {
		http.Error(w, "template not found", http.StatusInternalServerError)
		return nil
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return tmpl.ExecuteTemplate(w, "base", data)
}

func StaticHandler() http.Handler {
	staticFS, err := fs.Sub(files, "static")
	if err != nil {
		return http.NotFoundHandler()
	}
	return http.FileServer(http.FS(staticFS))
}
