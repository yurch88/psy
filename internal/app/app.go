package app

import (
	"context"
	"log/slog"
	"net/http"
	"path/filepath"

	"psy/internal/calendar"
	"psy/internal/config"
	"psy/internal/content"
	"psy/internal/handlers"
	"psy/internal/mailer"
	"psy/internal/rates"
	"psy/internal/telegram"
	"psy/internal/ui"
)

type App struct {
	handler     http.Handler
	backgrounds []func(context.Context)
}

func New(cfg config.Config, logger *slog.Logger) (*App, error) {
	renderer, err := ui.NewRenderer()
	if err != nil {
		return nil, err
	}

	calendarService, err := calendar.NewService(cfg.BaseTimezone, cfg.BookingsPath, filepath.Join(cfg.DataDir, "slot-rules.json"))
	if err != nil {
		return nil, err
	}

	defaultSite := content.DefaultSite(content.Contact{
		Email:       cfg.Email,
		Phone:       cfg.Phone,
		Location:    cfg.Location,
		TelegramURL: cfg.TelegramURL,
		MaxURL:      cfg.MaxURL,
		CalendarURL: cfg.CalendarURL,
	})

	contentManager, err := content.NewManager(cfg.DataDir, defaultSite)
	if err != nil {
		return nil, err
	}
	site := contentManager.Published()

	var backgrounds []func(context.Context)
	emailService := mailer.NewResend(cfg.ResendAPIKey, cfg.EmailFrom, cfg.EmailReplyTo, cfg.BaseTimezone, logger)
	telegramService := telegram.New(cfg.TelegramBotToken, cfg.TelegramNotifyChatIDs, calendarService, emailService, logger)
	if telegramService.Enabled() {
		backgrounds = append(backgrounds, telegramService.Run)
	}

	rateService := rates.NewService(cfg.USDRateURL, cfg.USDRateTimeout)
	pageHandler := handlers.New(site, contentManager, renderer, calendarService, rateService, telegramService, emailService, logger, cfg.AdminLogin, cfg.AdminPass)

	mux := http.NewServeMux()
	pageHandler.Register(mux)
	mux.Handle("/static/", http.StripPrefix("/static/", ui.StaticHandler()))

	return &App{
		handler:     mux,
		backgrounds: backgrounds,
	}, nil
}

func (a *App) Start(ctx context.Context) {
	for _, background := range a.backgrounds {
		go background(ctx)
	}
}

func (a *App) Handler() http.Handler {
	return a.handler
}
