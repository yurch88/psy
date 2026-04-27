package calendar

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

type slotWindow struct {
	slot Slot
	end  time.Time
}

func (s *Service) ensureDefaultRules() error {
	if _, err := os.Stat(s.rules.path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	rules, err := s.rules.List()
	if err != nil {
		return err
	}
	if len(rules) > 0 {
		return nil
	}
	return s.rules.Save(context.Background(), defaultSlotRules(s.now()))
}

func (s *Service) Rules() ([]SlotRule, error) {
	rules, err := s.rules.List()
	if err != nil {
		return nil, err
	}
	return append([]SlotRule(nil), rules...), nil
}

func (s *Service) AddRule(ctx context.Context, input SlotRuleInput) (SlotRule, error) {
	normalized, err := normalizeSlotRuleInput(input)
	if err != nil {
		return SlotRule{}, err
	}

	rules, err := s.rules.List()
	if err != nil {
		return SlotRule{}, err
	}

	rule := SlotRule{
		ID:              fmt.Sprintf("rule-%d-%d", s.now().UnixNano(), s.seq.Add(1)),
		Scope:           normalized.Scope,
		Date:            normalized.Date,
		Weekdays:        normalized.Weekdays,
		StartTimes:      normalized.StartTimes,
		DurationMinutes: normalized.DurationMinutes,
		CreatedAt:       s.now().UTC(),
	}

	rules = append(rules, rule)
	if err := s.rules.Save(ctx, rules); err != nil {
		return SlotRule{}, err
	}

	return rule, nil
}

func (s *Service) WeeklySchedule() ([]WeeklyScheduleDay, error) {
	rules, err := s.rules.List()
	if err != nil {
		return nil, err
	}

	byDay := make(map[int]map[string]bool)
	for _, rule := range rules {
		if rule.Scope != SlotRuleScopeWeekly {
			continue
		}
		for _, day := range rule.Weekdays {
			if byDay[day] == nil {
				byDay[day] = make(map[string]bool)
			}
			for _, startTime := range rule.StartTimes {
				byDay[day][startTime] = true
			}
		}
	}

	result := make([]WeeklyScheduleDay, 0, 7)
	for day := 1; day <= 7; day++ {
		startTimes := make([]string, 0, len(byDay[day]))
		for startTime := range byDay[day] {
			startTimes = append(startTimes, startTime)
		}
		sort.Strings(startTimes)
		result = append(result, WeeklyScheduleDay{
			Day:        day,
			StartTimes: startTimes,
		})
	}

	return result, nil
}

func (s *Service) ReplaceWeeklySchedule(ctx context.Context, days []WeeklyScheduleDay) error {
	rules, err := s.rules.List()
	if err != nil {
		return err
	}

	filtered := make([]SlotRule, 0, len(rules))
	for _, rule := range rules {
		if rule.Scope == SlotRuleScopeWeekly {
			continue
		}
		filtered = append(filtered, rule)
	}

	for _, day := range days {
		if len(day.StartTimes) == 0 {
			continue
		}

		normalized, err := normalizeSlotRuleInput(SlotRuleInput{
			Scope:           SlotRuleScopeWeekly,
			Weekdays:        []int{day.Day},
			StartTimes:      day.StartTimes,
			DurationMinutes: 55,
		})
		if err != nil {
			return err
		}

		filtered = append(filtered, SlotRule{
			ID:              fmt.Sprintf("rule-%d-%d", s.now().UnixNano(), s.seq.Add(1)),
			Scope:           SlotRuleScopeWeekly,
			Weekdays:        normalized.Weekdays,
			StartTimes:      normalized.StartTimes,
			DurationMinutes: normalized.DurationMinutes,
			CreatedAt:       s.now().UTC(),
		})
	}

	return s.rules.Save(ctx, filtered)
}

func (s *Service) ReplaceDateSchedule(ctx context.Context, date string, startTimes []string) error {
	normalized, err := normalizeSlotRuleInput(SlotRuleInput{
		Scope:           SlotRuleScopeDate,
		Date:            date,
		StartTimes:      startTimes,
		DurationMinutes: 55,
	})
	if err != nil {
		return err
	}

	rules, err := s.rules.List()
	if err != nil {
		return err
	}

	filtered := make([]SlotRule, 0, len(rules))
	for _, rule := range rules {
		if rule.Scope == SlotRuleScopeDate && rule.Date == normalized.Date {
			continue
		}
		filtered = append(filtered, rule)
	}

	filtered = append(filtered, SlotRule{
		ID:              fmt.Sprintf("rule-%d-%d", s.now().UnixNano(), s.seq.Add(1)),
		Scope:           SlotRuleScopeDate,
		Mode:            SlotRuleModeOverride,
		Date:            normalized.Date,
		StartTimes:      normalized.StartTimes,
		DurationMinutes: normalized.DurationMinutes,
		CreatedAt:       s.now().UTC(),
	})

	return s.rules.Save(ctx, filtered)
}

func (s *Service) DeleteDateSchedule(ctx context.Context, date string) error {
	date = strings.TrimSpace(date)
	if _, err := time.ParseInLocation("2006-01-02", date, s.location); err != nil {
		return ErrSlotNotFound
	}

	rules, err := s.rules.List()
	if err != nil {
		return err
	}

	filtered := make([]SlotRule, 0, len(rules))
	found := false
	for _, rule := range rules {
		if rule.Scope == SlotRuleScopeDate && rule.Date == date {
			found = true
			continue
		}
		filtered = append(filtered, rule)
	}
	if !found {
		return ErrSlotNotFound
	}

	return s.rules.Save(ctx, filtered)
}

func (s *Service) ScheduleForDate(date string) ([]string, error) {
	day, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(date), s.location)
	if err != nil {
		return nil, fmt.Errorf("invalid date")
	}

	rules, err := s.rules.List()
	if err != nil {
		return nil, err
	}

	return slotStartsFromWindows(effectiveSlotWindowsForDay(rulesForDay(rules, day), day, s.location, nil, false, time.Time{})), nil
}

func (s *Service) DateSchedules() ([]DateSchedule, error) {
	rules, err := s.rules.List()
	if err != nil {
		return nil, err
	}

	grouped := make(map[string][]SlotRule)
	order := make([]string, 0)
	for _, rule := range rules {
		if rule.Scope != SlotRuleScopeDate {
			continue
		}
		if _, ok := grouped[rule.Date]; !ok {
			order = append(order, rule.Date)
		}
		grouped[rule.Date] = append(grouped[rule.Date], rule)
	}

	sort.Strings(order)

	schedules := make([]DateSchedule, 0, len(order))
	for _, date := range order {
		day, err := time.ParseInLocation("2006-01-02", date, s.location)
		if err != nil {
			continue
		}
		schedules = append(schedules, DateSchedule{
			Date:       date,
			StartTimes: slotStartsFromWindows(effectiveSlotWindowsForDay(rulesForDay(rules, day), day, s.location, nil, false, time.Time{})),
		})
	}

	return schedules, nil
}

func (s *Service) DeleteRule(ctx context.Context, id string) error {
	rules, err := s.rules.List()
	if err != nil {
		return err
	}

	filtered := make([]SlotRule, 0, len(rules))
	found := false
	for _, rule := range rules {
		if rule.ID == id {
			found = true
			continue
		}
		filtered = append(filtered, rule)
	}
	if !found {
		return ErrSlotNotFound
	}

	return s.rules.Save(ctx, filtered)
}

func (s *Service) generatedSlots(reserved map[string]bool) []Slot {
	now := s.now().In(s.location)
	startDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.location)
	endDay := oneMonthAheadExclusive(startDay)

	rules, err := s.rules.List()
	if err != nil {
		return nil
	}

	slots := make([]Slot, 0, 128)
	seen := make(map[string]bool)

	for day := startDay; day.Before(endDay); day = day.AddDate(0, 0, 1) {
		for _, window := range effectiveSlotWindowsForDay(rulesForDay(rules, day), day, s.location, reserved, true, now) {
			if seen[window.slot.ID] {
				continue
			}
			seen[window.slot.ID] = true
			slots = append(slots, window.slot)
		}
	}

	sort.Slice(slots, func(i, j int) bool {
		return slots[i].Start.Before(slots[j].Start)
	})

	return slots
}

func oneMonthAheadExclusive(startDay time.Time) time.Time {
	year, month, _ := startDay.Date()
	location := startDay.Location()
	lastDayOfNextMonth := time.Date(year, month+2, 0, 0, 0, 0, 0, location).Day()
	targetDay := startDay.Day()
	if targetDay > lastDayOfNextMonth {
		targetDay = lastDayOfNextMonth
	}

	lastVisibleDay := time.Date(year, month+1, targetDay, 0, 0, 0, 0, location)
	return lastVisibleDay.AddDate(0, 0, 1)
}

func rulesForDay(rules []SlotRule, day time.Time) []SlotRule {
	applicable := make([]SlotRule, 0, len(rules))
	for _, rule := range rules {
		if !ruleAppliesToDate(rule, day) {
			continue
		}
		applicable = append(applicable, rule)
	}

	return applicable
}

func effectiveSlotWindowsForDay(rules []SlotRule, day time.Time, location *time.Location, reserved map[string]bool, enforceLeadTime bool, now time.Time) []slotWindow {
	dateRules := make([]SlotRule, 0, len(rules))
	weeklyRules := make([]SlotRule, 0, len(rules))
	for _, rule := range rules {
		switch rule.Scope {
		case SlotRuleScopeDate:
			dateRules = append(dateRules, rule)
		case SlotRuleScopeWeekly:
			weeklyRules = append(weeklyRules, rule)
		}
	}

	windows := weeklySlotWindows(weeklyRules, day, location, reserved, enforceLeadTime, now)
	if len(dateRules) == 0 {
		return windows
	}
	return applyDateRulesToWindows(windows, dateRules, day, location, reserved, enforceLeadTime, now)
}

func weeklySlotWindows(rules []SlotRule, day time.Time, location *time.Location, reserved map[string]bool, enforceLeadTime bool, now time.Time) []slotWindow {
	windows := make([]slotWindow, 0, len(rules))
	seen := make(map[string]bool)

	for _, rule := range rules {
		for _, startTime := range rule.StartTimes {
			window, ok := buildSlotWindow(day, startTime, rule.DurationMinutes, location, reserved, enforceLeadTime, now)
			if !ok || seen[window.slot.ID] {
				continue
			}
			seen[window.slot.ID] = true
			windows = append(windows, window)
		}
	}

	sortSlotWindows(windows)
	return windows
}

func applyDateRulesToWindows(base []slotWindow, rules []SlotRule, day time.Time, location *time.Location, reserved map[string]bool, enforceLeadTime bool, now time.Time) []slotWindow {
	windows := append([]slotWindow(nil), base...)
	for _, rule := range rules {
		if rule.Mode == SlotRuleModeOverride {
			windows = windows[:0]
		}
		for _, startTime := range rule.StartTimes {
			window, ok := buildSlotWindow(day, startTime, rule.DurationMinutes, location, reserved, enforceLeadTime, now)
			if !ok {
				continue
			}
			filtered := windows[:0]
			for _, existing := range windows {
				if timesOverlap(existing.slot.Start, existing.end, window.slot.Start, window.end) {
					continue
				}
				filtered = append(filtered, existing)
			}
			windows = append(filtered, window)
		}
	}

	sortSlotWindows(windows)
	return windows
}

func buildSlotWindow(day time.Time, startTime string, durationMinutes int, location *time.Location, reserved map[string]bool, enforceLeadTime bool, now time.Time) (slotWindow, bool) {
	startMinutes, err := parseClock(startTime)
	if err != nil {
		return slotWindow{}, false
	}

	start := time.Date(day.Year(), day.Month(), day.Day(), startMinutes/60, startMinutes%60, 0, 0, location)
	end := start.Add(time.Duration(durationMinutes) * time.Minute)
	if enforceLeadTime && start.Before(now.Add(time.Hour)) {
		return slotWindow{}, false
	}
	if !end.After(start) {
		return slotWindow{}, false
	}

	id := start.Format("20060102T1504")
	disabled := false
	if reserved != nil {
		disabled = reserved[id]
	}

	return slotWindow{
		slot: Slot{
			ID:       id,
			Start:    start,
			End:      end,
			Disabled: disabled,
		},
		end: end,
	}, true
}

func slotStartsFromWindows(windows []slotWindow) []string {
	startTimes := make([]string, 0, len(windows))
	for _, window := range windows {
		startTimes = append(startTimes, window.slot.Start.Format("15:04"))
	}
	return startTimes
}

func sortSlotWindows(windows []slotWindow) {
	sort.Slice(windows, func(i, j int) bool {
		return windows[i].slot.Start.Before(windows[j].slot.Start)
	})
}

func timesOverlap(firstStart, firstEnd, secondStart, secondEnd time.Time) bool {
	return firstStart.Before(secondEnd) && secondStart.Before(firstEnd)
}

func ruleAppliesToDate(rule SlotRule, day time.Time) bool {
	switch rule.Scope {
	case SlotRuleScopeWeekly:
		weekday := int(day.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		for _, value := range rule.Weekdays {
			if value == weekday {
				return true
			}
		}
		return false
	case SlotRuleScopeDate:
		return day.Format("2006-01-02") == rule.Date
	default:
		return false
	}
}
