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
	"sync/atomic"
	"time"
)

type BookingStatus string

const (
	BookingStatusPending   BookingStatus = "pending"
	BookingStatusConfirmed BookingStatus = "confirmed"
	BookingStatusRejected  BookingStatus = "rejected"
)

const (
	ResolutionConfirmed = "confirmed"
	ResolutionRejected  = "rejected"
	ResolutionSlotTaken = "slot_taken"
)

type ReviewAction string

const (
	ReviewActionConfirm ReviewAction = "confirm"
	ReviewActionReject  ReviewAction = "reject"
)

var ErrBookingNotFound = errors.New("booking not found")

type Service struct {
	location *time.Location
	store    *Store
	now      func() time.Time
	seq      atomic.Uint64
}

type Slot struct {
	ID       string
	Start    time.Time
	End      time.Time
	Disabled bool
}

type BookingRequest struct {
	SlotID         string
	Name           string
	Email          string
	Phone          string
	ClientTimezone string
	Comment        string
}

type NotificationRef struct {
	ChatID    string `json:"chat_id"`
	MessageID int    `json:"message_id"`
}

type Booking struct {
	ID             string            `json:"id"`
	SlotID         string            `json:"slot_id"`
	Start          time.Time         `json:"start"`
	End            time.Time         `json:"end"`
	Name           string            `json:"name"`
	Email          string            `json:"email"`
	Phone          string            `json:"phone"`
	ClientTimezone string            `json:"client_timezone"`
	Comment        string            `json:"comment,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	Status         BookingStatus     `json:"status,omitempty"`
	ReviewedAt     *time.Time        `json:"reviewed_at,omitempty"`
	Resolution     string            `json:"resolution,omitempty"`
	Notifications  []NotificationRef `json:"notifications,omitempty"`
}

type ReviewResult struct {
	Booking                 Booking
	Updated                 []Booking
	CallbackText            string
	TransitionedToConfirmed bool
}

func (b Booking) EffectiveStatus() BookingStatus {
	if b.Status == "" {
		return BookingStatusConfirmed
	}
	return b.Status
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

func (s *Service) Slots() []Slot {
	now := s.now().In(s.location)
	startDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.location)
	bookedSlots := s.store.ConfirmedSlotIDs()
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
			slots = append(slots, Slot{
				ID:       id,
				Start:    start,
				End:      start.Add(55 * time.Minute),
				Disabled: bookedSlots[id],
			})
		}
	}

	return slots
}

func (s *Service) AvailableSlots() []Slot {
	allSlots := s.Slots()
	available := make([]Slot, 0, len(allSlots))
	for _, slot := range allSlots {
		if slot.Disabled {
			continue
		}
		available = append(available, slot)
	}
	return available
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
		ID:             fmt.Sprintf("%s-%d-%d", request.SlotID, s.now().UnixNano(), s.seq.Add(1)),
		SlotID:         request.SlotID,
		Start:          slot.Start,
		End:            slot.End,
		Name:           request.Name,
		Email:          request.Email,
		Phone:          request.Phone,
		ClientTimezone: request.ClientTimezone,
		Comment:        request.Comment,
		CreatedAt:      s.now().UTC(),
		Status:         BookingStatusPending,
	}

	if err := s.store.Append(ctx, booking); err != nil {
		return Booking{}, err
	}

	return booking, nil
}

func (s *Service) AttachNotifications(ctx context.Context, bookingID string, refs []NotificationRef) (Booking, error) {
	return s.store.AddNotifications(ctx, bookingID, refs)
}

func (s *Service) Review(ctx context.Context, bookingID string, action ReviewAction) (ReviewResult, error) {
	return s.store.Review(ctx, bookingID, action, s.now().UTC())
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

func (s *Store) ConfirmedSlotIDs() map[string]bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	booked := make(map[string]bool)
	bookings, err := s.loadLocked()
	if err != nil {
		return booked
	}

	for _, booking := range bookings {
		if booking.SlotID != "" && booking.EffectiveStatus() == BookingStatusConfirmed {
			booked[booking.SlotID] = true
		}
	}

	return booked
}

func (s *Store) AddNotifications(ctx context.Context, bookingID string, refs []NotificationRef) (Booking, error) {
	select {
	case <-ctx.Done():
		return Booking{}, ctx.Err()
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	bookings, err := s.loadLocked()
	if err != nil {
		return Booking{}, err
	}

	for i := range bookings {
		if bookings[i].ID != bookingID {
			continue
		}

		bookings[i].Notifications = mergeNotifications(bookings[i].Notifications, refs)
		if err := s.saveLocked(ctx, bookings); err != nil {
			return Booking{}, err
		}
		return bookings[i], nil
	}

	return Booking{}, ErrBookingNotFound
}

func (s *Store) Review(ctx context.Context, bookingID string, action ReviewAction, now time.Time) (ReviewResult, error) {
	select {
	case <-ctx.Done():
		return ReviewResult{}, ctx.Err()
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	bookings, err := s.loadLocked()
	if err != nil {
		return ReviewResult{}, err
	}

	index := -1
	for i := range bookings {
		if bookings[i].ID == bookingID {
			index = i
			break
		}
	}
	if index == -1 {
		return ReviewResult{}, ErrBookingNotFound
	}

	target := &bookings[index]
	currentStatus := target.EffectiveStatus()

	switch action {
	case ReviewActionConfirm:
		switch currentStatus {
		case BookingStatusConfirmed:
			return ReviewResult{Booking: *target, CallbackText: "Заявка уже подтверждена"}, nil
		case BookingStatusRejected:
			return ReviewResult{Booking: *target, CallbackText: "Заявка уже отклонена"}, nil
		}

		if hasAnotherConfirmed(bookings, target.ID, target.SlotID) {
			target.Status = BookingStatusRejected
			target.ReviewedAt = timePtr(now)
			target.Resolution = ResolutionSlotTaken
			if err := s.saveLocked(ctx, bookings); err != nil {
				return ReviewResult{}, err
			}

			return ReviewResult{
				Booking:      *target,
				Updated:      []Booking{*target},
				CallbackText: "Слот уже подтвержден другой заявкой",
			}, nil
		}

		target.Status = BookingStatusConfirmed
		target.ReviewedAt = timePtr(now)
		target.Resolution = ResolutionConfirmed

		updated := []Booking{*target}
		for i := range bookings {
			if i == index || bookings[i].SlotID != target.SlotID {
				continue
			}
			if bookings[i].EffectiveStatus() != BookingStatusPending {
				continue
			}

			bookings[i].Status = BookingStatusRejected
			bookings[i].ReviewedAt = timePtr(now)
			bookings[i].Resolution = ResolutionSlotTaken
			updated = append(updated, bookings[i])
		}

		if err := s.saveLocked(ctx, bookings); err != nil {
			return ReviewResult{}, err
		}

		return ReviewResult{
			Booking:                 *target,
			Updated:                 updated,
			CallbackText:            "Заявка подтверждена, слот закрыт на сайте",
			TransitionedToConfirmed: true,
		}, nil

	case ReviewActionReject:
		switch currentStatus {
		case BookingStatusConfirmed:
			return ReviewResult{Booking: *target, CallbackText: "Заявка уже подтверждена"}, nil
		case BookingStatusRejected:
			return ReviewResult{Booking: *target, CallbackText: "Заявка уже отклонена"}, nil
		}

		target.Status = BookingStatusRejected
		target.ReviewedAt = timePtr(now)
		target.Resolution = ResolutionRejected

		if err := s.saveLocked(ctx, bookings); err != nil {
			return ReviewResult{}, err
		}

		return ReviewResult{
			Booking:      *target,
			Updated:      []Booking{*target},
			CallbackText: "Заявка отклонена, слот снова доступен",
		}, nil

	default:
		return ReviewResult{}, fmt.Errorf("unknown review action %q", action)
	}
}

func (s *Store) loadLocked() ([]Booking, error) {
	file, err := os.Open(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	bookings := make([]Booking, 0)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := bytesTrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var booking Booking
		if err := json.Unmarshal(line, &booking); err != nil {
			continue
		}
		bookings = append(bookings, booking)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return bookings, nil
}

func (s *Store) saveLocked(ctx context.Context, bookings []Booking) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	file, err := os.CreateTemp(filepath.Dir(s.path), "bookings-*.jsonl")
	if err != nil {
		return err
	}

	tmpName := file.Name()
	writer := bufio.NewWriter(file)

	writeErr := func(err error) error {
		writer.Flush()
		file.Close()
		_ = os.Remove(tmpName)
		return err
	}

	for _, booking := range bookings {
		payload, err := json.Marshal(booking)
		if err != nil {
			return writeErr(err)
		}
		if _, err := writer.Write(payload); err != nil {
			return writeErr(err)
		}
		if err := writer.WriteByte('\n'); err != nil {
			return writeErr(err)
		}
	}

	if err := writer.Flush(); err != nil {
		return writeErr(err)
	}
	if err := file.Chmod(0o600); err != nil {
		return writeErr(err)
	}
	if err := file.Close(); err != nil {
		return writeErr(err)
	}

	if err := os.Rename(tmpName, s.path); err != nil {
		_ = os.Remove(s.path)
		if secondErr := os.Rename(tmpName, s.path); secondErr != nil {
			_ = os.Remove(tmpName)
			return secondErr
		}
	}

	return nil
}

func hasAnotherConfirmed(bookings []Booking, bookingID, slotID string) bool {
	for _, booking := range bookings {
		if booking.ID == bookingID || booking.SlotID != slotID {
			continue
		}
		if booking.EffectiveStatus() == BookingStatusConfirmed {
			return true
		}
	}
	return false
}

func mergeNotifications(existing, refs []NotificationRef) []NotificationRef {
	if len(refs) == 0 {
		return existing
	}

	merged := append([]NotificationRef{}, existing...)
	for _, ref := range refs {
		if ref.ChatID == "" || ref.MessageID == 0 {
			continue
		}
		if containsNotification(merged, ref) {
			continue
		}
		merged = append(merged, ref)
	}
	return merged
}

func containsNotification(refs []NotificationRef, target NotificationRef) bool {
	for _, ref := range refs {
		if ref.ChatID == target.ChatID && ref.MessageID == target.MessageID {
			return true
		}
	}
	return false
}

func timePtr(value time.Time) *time.Time {
	copyValue := value
	return &copyValue
}

func bytesTrimSpace(value []byte) []byte {
	return []byte(strings.TrimSpace(string(value)))
}
