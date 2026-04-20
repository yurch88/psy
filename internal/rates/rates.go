package rates

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sync"
	"time"
)

const (
	consultationUSD    = 65
	consultationMinRUB = 5000
)

type Service struct {
	url     string
	timeout time.Duration
	client  *http.Client
	mu      sync.Mutex
	cache   cachedRate
}

type cachedRate struct {
	value     float64
	expiresAt time.Time
}

func NewService(url string, timeout time.Duration) *Service {
	return &Service{
		url:     url,
		timeout: timeout,
		client:  &http.Client{Timeout: timeout},
	}
}

func (s *Service) ConsultationUSD(ctx context.Context) (string, bool) {
	rate, ok := s.usdRate(ctx)
	if !ok {
		return "", false
	}

	price := int(math.Ceil(consultationMinRUB / rate))
	if price < consultationUSD {
		price = consultationUSD
	}

	return fmt.Sprintf("%d $", price), true
}

func (s *Service) usdRate(ctx context.Context) (float64, bool) {
	now := time.Now()

	s.mu.Lock()
	if s.cache.value > 0 && now.Before(s.cache.expiresAt) {
		value := s.cache.value
		s.mu.Unlock()
		return value, true
	}
	s.mu.Unlock()

	requestCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, s.url, nil)
	if err != nil {
		return 0, false
	}

	response, err := s.client.Do(request)
	if err != nil {
		return 0, false
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return 0, false
	}

	var payload struct {
		Valute struct {
			USD struct {
				Value float64 `json:"Value"`
			} `json:"USD"`
		} `json:"Valute"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return 0, false
	}
	if payload.Valute.USD.Value <= 0 {
		return 0, false
	}

	s.mu.Lock()
	s.cache = cachedRate{value: payload.Valute.USD.Value, expiresAt: now.Add(12 * time.Hour)}
	s.mu.Unlock()

	return payload.Valute.USD.Value, true
}
