package app

import (
	"log/slog"
	"net/http"

	"psy/internal/calendar"
	"psy/internal/config"
	"psy/internal/content"
	"psy/internal/handlers"
	"psy/internal/rates"
	"psy/internal/ui"
)

type App struct {
	handler http.Handler
}

func New(cfg config.Config, logger *slog.Logger) (*App, error) {
	renderer, err := ui.NewRenderer()
	if err != nil {
		return nil, err
	}

	calendarService, err := calendar.NewService(cfg.BaseTimezone, cfg.BookingsPath)
	if err != nil {
		return nil, err
	}

	site := content.DefaultSite(content.Contact{
		Email:       cfg.Email,
		Phone:       cfg.Phone,
		Location:    cfg.Location,
		TelegramURL: cfg.TelegramURL,
		MaxURL:      cfg.MaxURL,
		CalendarURL: cfg.CalendarURL,
	})

	rateService := rates.NewService(cfg.USDRateURL, cfg.USDRateTimeout)
	pageHandler := handlers.New(site, renderer, calendarService, rateService, logger)

	mux := http.NewServeMux()
	pageHandler.Register(mux)
	mux.Handle("/static/", http.StripPrefix("/static/", ui.StaticHandler()))

	return &App{handler: mux}, nil
}

func (a *App) Handler() http.Handler {
	return a.handler
}
