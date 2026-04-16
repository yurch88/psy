package mailer

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"psy/internal/calendar"
)

func TestSendBookingConfirmationUsesResendAPI(t *testing.T) {
	t.Parallel()

	type requestBody struct {
		From    string     `json:"from"`
		To      []string   `json:"to"`
		Subject string     `json:"subject"`
		HTML    string     `json:"html"`
		Text    string     `json:"text"`
		ReplyTo []string   `json:"reply_to"`
		Tags    []emailTag `json:"tags"`
	}

	var captured requestBody
	var authHeader string
	var idempotencyKey string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		idempotencyKey = r.Header.Get("Idempotency-Key")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"email_123"}`))
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := NewResend(
		"re_test_123",
		"Natalya Kudinova <booking@kudinovanatalya-psy.ru>",
		"natalia.kudinova.psy@gmail.com",
		"Europe/Moscow",
		logger,
	)
	service.endpoint = server.URL

	location, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	booking := calendar.Booking{
		ID:             "20260420T0900-1",
		SlotID:         "20260420T0900",
		Start:          time.Date(2026, time.April, 20, 9, 0, 0, 0, location),
		End:            time.Date(2026, time.April, 20, 9, 55, 0, 0, location),
		Name:           "Иван Иванов",
		Email:          "ivan@example.com",
		ClientTimezone: "Europe/Berlin",
	}

	if err := service.SendBookingConfirmation(context.Background(), booking); err != nil {
		t.Fatalf("send confirmation: %v", err)
	}

	if authHeader != "Bearer re_test_123" {
		t.Fatalf("unexpected Authorization header: %q", authHeader)
	}
	if idempotencyKey != "booking-confirmation-"+booking.ID {
		t.Fatalf("unexpected idempotency key: %q", idempotencyKey)
	}
	if captured.From != "Natalya Kudinova <booking@kudinovanatalya-psy.ru>" {
		t.Fatalf("unexpected from: %q", captured.From)
	}
	if len(captured.To) != 1 || captured.To[0] != "ivan@example.com" {
		t.Fatalf("unexpected recipients: %#v", captured.To)
	}
	if len(captured.ReplyTo) != 1 || captured.ReplyTo[0] != "natalia.kudinova.psy@gmail.com" {
		t.Fatalf("unexpected reply_to: %#v", captured.ReplyTo)
	}
	if !strings.Contains(captured.Subject, "Запись подтверждена") {
		t.Fatalf("unexpected subject: %q", captured.Subject)
	}
	if !strings.Contains(captured.Text, "Europe/Berlin") {
		t.Fatalf("expected client timezone in text body, got %q", captured.Text)
	}
	if !strings.Contains(captured.Text, "По Москве") {
		t.Fatalf("expected Moscow fallback in text body, got %q", captured.Text)
	}
	if !strings.Contains(captured.HTML, "Ваша запись на онлайн-консультацию подтверждена") {
		t.Fatalf("unexpected html body: %q", captured.HTML)
	}
	if len(captured.Tags) < 2 {
		t.Fatalf("expected tags to be sent, got %#v", captured.Tags)
	}
}
