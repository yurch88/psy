package calendar

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestPendingBookingBlocksSlotAndCancelReopensIt(t *testing.T) {
	location, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	tempDir := t.TempDir()
	service, err := NewService("Europe/Moscow", filepath.Join(tempDir, "bookings.jsonl"), filepath.Join(tempDir, "slot-rules.json"))
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

	if containsSlot(service.AvailableSlots(), slot.ID) {
		t.Fatalf("pending booking should block slot %s", slot.ID)
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

	if _, err := service.Cancel(context.Background(), first.ID); err != nil {
		t.Fatalf("cancel booking: %v", err)
	}

	if !containsSlot(service.AvailableSlots(), slot.ID) {
		t.Fatalf("cancelled booking should reopen slot %s", slot.ID)
	}
}

func TestRejectKeepsSlotAvailable(t *testing.T) {
	location, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	tempDir := t.TempDir()
	service, err := NewService("Europe/Moscow", filepath.Join(tempDir, "bookings.jsonl"), filepath.Join(tempDir, "slot-rules.json"))
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

	if containsSlot(service.AvailableSlots(), slot.ID) {
		t.Fatalf("pending booking should block slot %s", slot.ID)
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

func TestCancelPreservesMalformedLinesInBookingsFile(t *testing.T) {
	location, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	tempDir := t.TempDir()
	bookingsPath := filepath.Join(tempDir, "bookings.jsonl")
	service, err := NewService("Europe/Moscow", bookingsPath, filepath.Join(tempDir, "slot-rules.json"))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	service.now = func() time.Time {
		return time.Date(2026, time.April, 16, 6, 0, 0, 0, location)
	}

	slot := service.AvailableSlots()[0]
	booking, err := service.Book(context.Background(), BookingRequest{
		SlotID: slot.ID,
		Name:   "Мария Иванова",
		Email:  "maria@example.com",
		Phone:  "+79990000005",
	})
	if err != nil {
		t.Fatalf("book: %v", err)
	}

	file, err := os.OpenFile(bookingsPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open bookings file: %v", err)
	}
	if _, err := file.WriteString("{not-json}\n"); err != nil {
		file.Close()
		t.Fatalf("append malformed line: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close bookings file: %v", err)
	}

	if _, err := service.Cancel(context.Background(), booking.ID); err != nil {
		t.Fatalf("cancel booking: %v", err)
	}

	payload, err := os.ReadFile(bookingsPath)
	if err != nil {
		t.Fatalf("read bookings file: %v", err)
	}

	if !bytes.Contains(payload, []byte("{not-json}")) {
		t.Fatalf("expected malformed line to be preserved, got %q", string(payload))
	}
	if !bytes.Contains(payload, []byte(`"status":"cancelled"`)) {
		t.Fatalf("expected cancelled booking to be saved, got %q", string(payload))
	}
}

func TestDateRuleKeepsNonConflictingWeeklySlotsAndReplacesOverlappingOnes(t *testing.T) {
	location, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	tempDir := t.TempDir()
	service, err := NewService("Europe/Moscow", filepath.Join(tempDir, "bookings.jsonl"), filepath.Join(tempDir, "slot-rules.json"))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	service.now = func() time.Time {
		return time.Date(2026, time.April, 27, 8, 0, 0, 0, location)
	}

	err = service.ReplaceWeeklySchedule(context.Background(), []WeeklyScheduleDay{
		{Day: 2, StartTimes: []string{"09:00", "10:00", "11:00"}},
	})
	if err != nil {
		t.Fatalf("replace weekly schedule: %v", err)
	}

	targetDate := "2026-04-28"
	if _, err := service.AddRule(context.Background(), SlotRuleInput{
		Scope:           SlotRuleScopeDate,
		Date:            targetDate,
		StartTimes:      []string{"10:15"},
		DurationMinutes: 55,
	}); err != nil {
		t.Fatalf("add date rule: %v", err)
	}

	got := slotStartsForDate(service.AvailableSlots(), targetDate)
	want := []string{"09:00", "10:15"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected slots %v on %s, got %v", want, targetDate, got)
	}

	err = service.ReplaceWeeklySchedule(context.Background(), []WeeklyScheduleDay{
		{Day: 2, StartTimes: []string{"09:00", "10:00", "11:00", "12:00"}},
	})
	if err != nil {
		t.Fatalf("replace weekly schedule with extra slot: %v", err)
	}

	got = slotStartsForDate(service.AvailableSlots(), targetDate)
	want = []string{"09:00", "10:15", "12:00"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected updated slots %v on %s, got %v", want, targetDate, got)
	}
}

func TestDateRuleRequiresAtLeastOneStartTime(t *testing.T) {
	tempDir := t.TempDir()
	service, err := NewService("Europe/Moscow", filepath.Join(tempDir, "bookings.jsonl"), filepath.Join(tempDir, "slot-rules.json"))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := service.AddRule(context.Background(), SlotRuleInput{
		Scope:           SlotRuleScopeDate,
		Date:            "2026-04-28",
		StartTimes:      nil,
		DurationMinutes: 55,
	}); err == nil {
		t.Fatal("expected date rule without start times to fail")
	}
}

func TestSlotBecomesUnavailableOneHourBeforeStart(t *testing.T) {
	location, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	tempDir := t.TempDir()
	service, err := NewService("Europe/Moscow", filepath.Join(tempDir, "bookings.jsonl"), filepath.Join(tempDir, "slot-rules.json"))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	service.now = func() time.Time {
		return time.Date(2026, time.April, 16, 17, 0, 0, 0, location)
	}
	if !containsSlot(service.AvailableSlots(), "20260416T1800") {
		t.Fatal("expected 18:00 slot to remain available exactly one hour before start")
	}

	service.now = func() time.Time {
		return time.Date(2026, time.April, 16, 17, 1, 0, 0, location)
	}
	if containsSlot(service.AvailableSlots(), "20260416T1800") {
		t.Fatal("expected 18:00 slot to disappear once less than one hour remains")
	}
}

func TestSlotsVisibleForAtMostOneMonthAhead(t *testing.T) {
	location, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	tempDir := t.TempDir()
	service, err := NewService("Europe/Moscow", filepath.Join(tempDir, "bookings.jsonl"), filepath.Join(tempDir, "slot-rules.json"))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	service.now = func() time.Time {
		return time.Date(2026, time.April, 16, 6, 0, 0, 0, location)
	}

	limitDay := time.Date(2026, time.May, 16, 23, 59, 59, 0, location)
	hasLimitDaySlot := false

	for _, slot := range service.AvailableSlots() {
		if slot.Start.After(limitDay) {
			t.Fatalf("expected no slots after %s, got %s", limitDay.Format(time.RFC3339), slot.Start.Format(time.RFC3339))
		}
		if slot.Start.Format("2006-01-02") == "2026-05-16" {
			hasLimitDaySlot = true
		}
	}

	if !hasLimitDaySlot {
		t.Fatal("expected slots to remain visible through the same date next month")
	}
}

func TestOneMonthAheadExclusiveClampsToEndOfNextMonth(t *testing.T) {
	location, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	startDay := time.Date(2026, time.January, 31, 0, 0, 0, 0, location)
	got := oneMonthAheadExclusive(startDay)
	want := time.Date(2026, time.March, 1, 0, 0, 0, 0, location)

	if !got.Equal(want) {
		t.Fatalf("expected exclusive end %s, got %s", want.Format(time.RFC3339), got.Format(time.RFC3339))
	}
}

func TestReplaceWeeklyScheduleAcceptsDotSeparatedTimes(t *testing.T) {
	location, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	tempDir := t.TempDir()
	service, err := NewService("Europe/Moscow", filepath.Join(tempDir, "bookings.jsonl"), filepath.Join(tempDir, "slot-rules.json"))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	service.now = func() time.Time {
		return time.Date(2026, time.April, 27, 8, 0, 0, 0, location)
	}

	err = service.ReplaceWeeklySchedule(context.Background(), []WeeklyScheduleDay{
		{Day: 1, StartTimes: []string{"9.00", "18.00"}},
	})
	if err != nil {
		t.Fatalf("replace weekly schedule: %v", err)
	}

	schedule, err := service.WeeklySchedule()
	if err != nil {
		t.Fatalf("weekly schedule: %v", err)
	}

	var monday WeeklyScheduleDay
	for _, day := range schedule {
		if day.Day == 1 {
			monday = day
			break
		}
	}

	if len(monday.StartTimes) != 2 {
		t.Fatalf("expected 2 monday times, got %+v", monday.StartTimes)
	}
	if monday.StartTimes[0] != "09:00" || monday.StartTimes[1] != "18:00" {
		t.Fatalf("expected normalized monday times, got %+v", monday.StartTimes)
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

func slotStartsForDate(slots []Slot, targetDate string) []string {
	starts := make([]string, 0)
	for _, slot := range slots {
		if slot.Start.Format("2006-01-02") == targetDate {
			starts = append(starts, slot.Start.Format("15:04"))
		}
	}
	return starts
}

func findSlot(slots []Slot, target string) (Slot, bool) {
	for _, slot := range slots {
		if slot.ID == target {
			return slot, true
		}
	}
	return Slot{}, false
}
