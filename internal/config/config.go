package config

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	Addr           string
	DataDir        string
	BookingsPath   string
	BaseTimezone   string
	USDRateURL     string
	USDRateTimeout time.Duration

	Email        string
	Phone        string
	Location     string
	TelegramURL  string
	MaxURL       string
	CalendarURL  string
	EmailFrom    string
	EmailReplyTo string

	TelegramBotToken      string
	TelegramNotifyChatIDs []string
	ResendAPIKey          string
}

func FromEnv() Config {
	port := env("PORT", "8080")
	addr := env("ADDR", ":"+port)
	dataDir := env("DATA_DIR", "data")

	return Config{
		Addr:                  addr,
		DataDir:               dataDir,
		BookingsPath:          filepath.Join(dataDir, "bookings.jsonl"),
		BaseTimezone:          env("BASE_TIMEZONE", "Europe/Moscow"),
		USDRateURL:            env("USD_RATE_URL", "https://www.cbr-xml-daily.ru/daily_json.js"),
		USDRateTimeout:        3 * time.Second,
		Email:                 env("CONTACT_EMAIL", "natalia.kudinova.psy@gmail.com"),
		Phone:                 env("CONTACT_PHONE", "+7 (965) 260-50-32"),
		Location:              env("CONTACT_LOCATION", "Онлайн, Россия и другие страны"),
		TelegramURL:           telegramURL(),
		MaxURL:                env("MAX_URL", "#contacts"),
		CalendarURL:           calendarURL(),
		EmailFrom:             env("EMAIL_FROM", "Natalya Kudinova <booking@kudinovanatalya-psy.ru>"),
		EmailReplyTo:          env("EMAIL_REPLY_TO", env("CONTACT_EMAIL", "natalia.kudinova.psy@gmail.com")),
		TelegramBotToken:      env("TG_BOT_TOKEN", ""),
		TelegramNotifyChatIDs: csvEnv("TG_NOTIFY_CHAT_IDS"),
		ResendAPIKey:          env("RESEND_API_KEY", ""),
	}
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func calendarURL() string {
	value := env("CALENDAR_URL", "/booking")
	if value == "/booking#calendar" {
		return "/booking"
	}
	return value
}

func telegramURL() string {
	value := env("TELEGRAM_URL", "https://t.me/NatalyaPoetry")
	return strings.ReplaceAll(value, "NatalyaBKudinova", "NatalyaPoetry")
}

func csvEnv(key string) []string {
	value := os.Getenv(key)
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}

	return result
}
