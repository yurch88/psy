package calendar

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Service struct {
	location *time.Location
	store    *Store
	now      func() time.Time
}

type Slot struct {
	ID    string
	Start time.Time
	End   time.Time
}

type BookingRequest struct {
	SlotID         string
	Name           string
	Email          string
	Phone          string
	ClientTimezone string
	Comment        string
}

type Booking struct {
	ID             string    `json:"id"`
	SlotID         string    `json:"slot_id"`
	Start          time.Time `json:"start"`
	End            time.Time `json:"end"`
	Name           string    `json:"name"`
	Email          string    `json:"email"`
	Phone          string    `json:"phone"`
	ClientTimezone string    `json:"client_timezone"`
	Comment        string    `json:"comment,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type ValidationError []string

func (e ValidationError) Error() string {
	return strings.Join(e, "; ")
}

func NewService(timezone, bookingsPath string) (*Service, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("load timezone %q: %w", timezone, err)
	}

	return &Service{
		location: location,
		store:    NewStore(bookingsPath),
		now:      time.Now,
	}, nil
}

func (s *Service) AvailableSlots() []Slot {
	now := s.now().In(s.location)
	startDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.location)
	bookedSlots := s.store.BookedSlotIDs()
	slots := make([]Slot, 0, 54)

	for dayOffset := 0; dayOffset < 21; dayOffset++ {
		day := startDay.AddDate(0, 0, dayOffset)
		if day.Weekday() == time.Sunday {
			continue
		}

		hours := []int{9, 11, 14, 16, 18}
		if day.Weekday() == time.Saturday {
			hours = []int{10, 12, 15}
		}

		for _, hour := range hours {
			start := time.Date(day.Year(), day.Month(), day.Day(), hour, 0, 0, 0, s.location)
			if !start.After(now.Add(2 * time.Hour)) {
				continue
			}
			id := start.Format("20060102T1504")
			if bookedSlots[id] {
				continue
			}
			end := start.Add(55 * time.Minute)
			slots = append(slots, Slot{
				ID:    id,
				Start: start,
				End:   end,
			})
		}
	}

	return slots
}

func (s *Service) Book(ctx context.Context, request BookingRequest) (Booking, error) {
	request.Name = strings.TrimSpace(request.Name)
	request.Email = strings.TrimSpace(request.Email)
	request.Phone = strings.TrimSpace(request.Phone)
	request.ClientTimezone = strings.TrimSpace(request.ClientTimezone)
	request.Comment = strings.TrimSpace(request.Comment)

	var validation ValidationError
	if request.Name == "" {
		validation = append(validation, "Укажите ФИО")
	}
	if !looksLikeEmail(request.Email) {
		validation = append(validation, "Укажите корректный e-mail")
	}
	if len(normalizePhone(request.Phone)) < 7 {
		validation = append(validation, "Укажите телефон")
	}
	if request.SlotID == "" {
		validation = append(validation, "Выберите дату и время")
	}

	slot, ok := s.findSlot(request.SlotID)
	if request.SlotID != "" && !ok {
		validation = append(validation, "Выбранное время уже недоступно, выберите другой слот")
	}

	if len(validation) > 0 {
		return Booking{}, validation
	}

	booking := Booking{
		ID:             fmt.Sprintf("%s-%d", request.SlotID, s.now().UnixNano()),
		SlotID:         request.SlotID,
		Start:          slot.Start,
		End:            slot.End,
		Name:           request.Name,
		Email:          request.Email,
		Phone:          request.Phone,
		ClientTimezone: request.ClientTimezone,
		Comment:        request.Comment,
		CreatedAt:      s.now().UTC(),
	}

	if err := s.store.Append(ctx, booking); err != nil {
		return Booking{}, err
	}

	return booking, nil
}

func (s *Service) findSlot(id string) (Slot, bool) {
	for _, slot := range s.AvailableSlots() {
		if slot.ID == id {
			return slot, true
		}
	}
	return Slot{}, false
}

func looksLikeEmail(value string) bool {
	at := strings.Index(value, "@")
	dot := strings.LastIndex(value, ".")
	return at > 0 && dot > at+1 && dot < len(value)-1
}

func normalizePhone(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

type Store struct {
	path string
	mu   sync.Mutex
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Append(ctx context.Context, booking Booking) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()

	payload, err := json.Marshal(booking)
	if err != nil {
		return err
	}

	_, err = file.Write(append(payload, '\n'))
	return err
}

func (s *Store) BookedSlotIDs() map[string]bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	booked := make(map[string]bool)
	file, err := os.Open(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return booked
		}
		return booked
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var booking Booking
		if err := json.Unmarshal(scanner.Bytes(), &booking); err != nil {
			continue
		}
		if booking.SlotID != "" {
			booked[booking.SlotID] = true
		}
	}

	return booked
}
