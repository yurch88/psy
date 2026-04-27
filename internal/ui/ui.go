package ui

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

//go:embed templates/*.html static/**
var files embed.FS

type Renderer struct {
	pages map[string]*template.Template
}

func NewRenderer() (*Renderer, error) {
	renderer := &Renderer{pages: make(map[string]*template.Template)}
	for _, page := range []string{"home", "rules", "memo", "privacy", "booking", "thanks", "administrator"} {
		parsed, err := template.New(page).Funcs(template.FuncMap{
			"year":      func() int { return time.Now().Year() },
			"mod":       func(a, b int) int { return a % b },
			"fontStack": safeFontStack,
			"phoneHref": phoneHref,
			"containsInt": func(values []int, target int) bool {
				for _, value := range values {
					if value == target {
						return true
					}
				}
				return false
			},
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

func safeFontStack(value string) template.CSS {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case strings.ContainsRune(" ,\"'-_", r):
		default:
			return ""
		}
	}

	return template.CSS(value)
}

func phoneHref(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	var builder strings.Builder
	for i, r := range value {
		switch {
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '+' && i == 0:
			builder.WriteRune(r)
		}
	}

	normalized := builder.String()
	if normalized == "" || normalized == "+" {
		return value
	}

	return normalized
}
