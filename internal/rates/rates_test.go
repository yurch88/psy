package rates

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestConsultationUSDRespectsMinimumRubleAmount(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Valute":{"USD":{"Value":74.4}}}`))
	}))
	defer server.Close()

	service := NewService(server.URL, time.Second)

	price, ok := service.ConsultationUSD(context.Background())
	if !ok {
		t.Fatal("expected price to be calculated")
	}

	if price != "68 $" {
		t.Fatalf("expected 68 $, got %q", price)
	}
}

func TestConsultationUSDDoesNotDropBelowBasePrice(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Valute":{"USD":{"Value":90}}}`))
	}))
	defer server.Close()

	service := NewService(server.URL, time.Second)

	price, ok := service.ConsultationUSD(context.Background())
	if !ok {
		t.Fatal("expected price to be calculated")
	}

	if price != "65 $" {
		t.Fatalf("expected 65 $, got %q", price)
	}
}
