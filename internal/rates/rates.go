package rates

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const consultationUSD = 65

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

func (s *Service) RubleEquivalent(ctx context.Context) (string, bool) {
	rate, ok := s.usdRate(ctx)
	if !ok {
		return "", false
	}

	rounded := int(math.Round(rate*consultationUSD/100) * 100)
	return fmt.Sprintf("примерно %s ₽ по текущему курсу", formatInt(rounded)), true
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

func formatInt(value int) string {
	raw := strconv.Itoa(value)
	if len(raw) <= 3 {
		return raw
	}

	result := make([]byte, 0, len(raw)+len(raw)/3)
	for i := range raw {
		if i > 0 && (len(raw)-i)%3 == 0 {
			result = append(result, ' ')
		}
		result = append(result, raw[i])
	}
	return string(result)
}
