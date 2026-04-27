package ui

import (
	"html"
	"net/http/httptest"
	"strings"
	"testing"

	"psy/internal/content"
)

func TestRenderBookingUsesSafeFontStackAndPhoneHref(t *testing.T) {
	renderer, err := NewRenderer()
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	site := content.DefaultSite(content.Contact{
		Email:       "test@example.com",
		Phone:       "+7 (965) 260-50-32",
		Location:    "Онлайн",
		TelegramURL: "https://t.me/example",
		MaxURL:      "#",
		CalendarURL: "/booking",
	})

	recorder := httptest.NewRecorder()
	data := map[string]any{
		"Title":          "Запись",
		"Description":    "Тест",
		"Site":           site,
		"HideSiteChrome": true,
		"SlotDays":       []any{},
		"Errors":         []string{},
		"Form": map[string]any{
			"SlotID":         "",
			"Name":           "",
			"Email":          "",
			"Phone":          "",
			"ClientTimezone": "",
			"Comment":        "",
		},
	}

	if err := renderer.Render(recorder, "booking", data); err != nil {
		t.Fatalf("render booking: %v", err)
	}

	body := recorder.Body.String()
	unescapedBody := html.UnescapeString(body)
	if strings.Contains(body, "ZgotmplZ") {
		t.Fatalf("expected font stack to be rendered safely, got %q", body)
	}
	if !strings.Contains(body, `--font-sans: "Aptos", "Segoe UI Variable Text", "Segoe UI", "Helvetica Neue", Arial, sans-serif;`) {
		t.Fatalf("expected custom font stack in output, got %q", body)
	}
	if !strings.Contains(unescapedBody, `href="tel:+79652605032"`) {
		t.Fatalf("expected normalized tel href, got %q", body)
	}
}

func TestRenderHomeKeepsOnlyOneInPageBookingCTA(t *testing.T) {
	renderer, err := NewRenderer()
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	site := content.DefaultSite(content.Contact{
		Email:       "test@example.com",
		Phone:       "+7 (965) 260-50-32",
		Location:    "Онлайн",
		TelegramURL: "https://t.me/example",
		MaxURL:      "#",
		CalendarURL: "/booking",
	})

	recorder := httptest.NewRecorder()
	data := map[string]any{
		"Title":          site.Brand,
		"Description":    site.Description,
		"Site":           site,
		"WorldPrice":     "",
		"HideSiteChrome": true,
	}

	if err := renderer.Render(recorder, "home", data); err != nil {
		t.Fatalf("render home: %v", err)
	}

	body := recorder.Body.String()
	if strings.Contains(body, "ЗАПИСАТЬСЯ НА ПРИЕМ") {
		t.Fatalf("expected hero CTA to be removed, got %q", body)
	}
	if got := strings.Count(body, `href="/booking"`); got != 1 {
		t.Fatalf("expected only pricing booking CTA in page content, got %d in %q", got, body)
	}
}
