package config

import (
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	Addr           string
	DataDir        string
	BookingsPath   string
	BaseTimezone   string
	USDRateURL     string
	USDRateTimeout time.Duration

	Email       string
	Phone       string
	Location    string
	TelegramURL string
	MaxURL      string
	CalendarURL string
}

func FromEnv() Config {
	port := env("PORT", "8080")
	addr := env("ADDR", ":"+port)
	dataDir := env("DATA_DIR", "data")

	return Config{
		Addr:           addr,
		DataDir:        dataDir,
		BookingsPath:   filepath.Join(dataDir, "bookings.jsonl"),
		BaseTimezone:   env("BASE_TIMEZONE", "Europe/Moscow"),
		USDRateURL:     env("USD_RATE_URL", "https://www.cbr-xml-daily.ru/daily_json.js"),
		USDRateTimeout: 3 * time.Second,
		Email:          env("CONTACT_EMAIL", "natalia.kudinova.psy@gmail.com"),
		Phone:          env("CONTACT_PHONE", "+7 (965) 260-50-32"),
		Location:       env("CONTACT_LOCATION", "Онлайн, Россия и другие страны"),
		TelegramURL:    env("TELEGRAM_URL", "https://t.me/NatalyaBKudinova"),
		MaxURL:         env("MAX_URL", "#contacts"),
		CalendarURL:    env("CALENDAR_URL", "/booking#calendar"),
	}
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
