package calendar

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestConfirmBlocksSlotAndRejectsOtherPendingRequests(t *testing.T) {
	location, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	service, err := NewService("Europe/Moscow", filepath.Join(t.TempDir(), "bookings.jsonl"))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	service.now = func() time.Time {
		return time.Date(2026, time.April, 16, 6, 0, 0, 0, location)
	}

	slot := service.AvailableSlots()[0]

	first, err := service.Book(context.Background(), BookingRequest{
		SlotID: slot.ID,
		Name:   "Иван Иванов",
		Email:  "ivan@example.com",
		Phone:  "+79990000001",
	})
	if err != nil {
		t.Fatalf("book first: %v", err)
	}

	second, err := service.Book(context.Background(), BookingRequest{
		SlotID: slot.ID,
		Name:   "Петр Петров",
		Email:  "petr@example.com",
		Phone:  "+79990000002",
	})
	if err != nil {
		t.Fatalf("book second: %v", err)
	}

	if !containsSlot(service.AvailableSlots(), slot.ID) {
		t.Fatalf("pending booking should not block slot %s", slot.ID)
	}

	result, err := service.Review(context.Background(), first.ID, ReviewActionConfirm)
	if err != nil {
		t.Fatalf("confirm first: %v", err)
	}
	if !result.TransitionedToConfirmed {
		t.Fatalf("expected confirmation transition flag to be set")
	}
	if result.Booking.EffectiveStatus() != BookingStatusConfirmed {
		t.Fatalf("expected confirmed status, got %s", result.Booking.EffectiveStatus())
	}

	if containsSlot(service.AvailableSlots(), slot.ID) {
		t.Fatalf("confirmed booking should block slot %s", slot.ID)
	}
	disabledSlot, ok := findSlot(service.Slots(), slot.ID)
	if !ok {
		t.Fatalf("expected slot %s to stay visible in full slot list", slot.ID)
	}
	if !disabledSlot.Disabled {
		t.Fatalf("expected confirmed slot %s to be disabled in full slot list", slot.ID)
	}

	secondReview, err := service.Review(context.Background(), second.ID, ReviewActionConfirm)
	if err != nil {
		t.Fatalf("review second after confirmation: %v", err)
	}
	if secondReview.Booking.EffectiveStatus() != BookingStatusRejected {
		t.Fatalf("expected second booking to be rejected, got %s", secondReview.Booking.EffectiveStatus())
	}
}

func TestRejectKeepsSlotAvailable(t *testing.T) {
	location, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	service, err := NewService("Europe/Moscow", filepath.Join(t.TempDir(), "bookings.jsonl"))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	service.now = func() time.Time {
		return time.Date(2026, time.April, 16, 6, 0, 0, 0, location)
	}

	slot := service.AvailableSlots()[0]

	booking, err := service.Book(context.Background(), BookingRequest{
		SlotID: slot.ID,
		Name:   "Анна Смирнова",
		Email:  "anna@example.com",
		Phone:  "+79990000003",
	})
	if err != nil {
		t.Fatalf("book: %v", err)
	}

	if _, err := service.Review(context.Background(), booking.ID, ReviewActionReject); err != nil {
		t.Fatalf("reject booking: %v", err)
	}

	if !containsSlot(service.AvailableSlots(), slot.ID) {
		t.Fatalf("rejected booking should keep slot %s available", slot.ID)
	}
	reopenedSlot, ok := findSlot(service.Slots(), slot.ID)
	if !ok {
		t.Fatalf("expected slot %s to stay visible after rejection", slot.ID)
	}
	if reopenedSlot.Disabled {
		t.Fatalf("expected rejected slot %s to remain enabled", slot.ID)
	}
}

func containsSlot(slots []Slot, target string) bool {
	for _, slot := range slots {
		if slot.ID == target {
			return true
		}
	}
	return false
}

func findSlot(slots []Slot, target string) (Slot, bool) {
	for _, slot := range slots {
		if slot.ID == target {
			return slot, true
		}
	}
	return Slot{}, false
}
