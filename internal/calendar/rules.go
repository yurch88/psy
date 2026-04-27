package calendar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	SlotRuleScopeWeekly = "weekly"
	SlotRuleScopeDate   = "date"
)

type TimeRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type SlotRule struct {
	ID              string      `json:"id"`
	Scope           string      `json:"scope"`
	Date            string      `json:"date,omitempty"`
	Weekdays        []int       `json:"weekdays,omitempty"`
	StartTimes      []string    `json:"start_times,omitempty"`
	TimeRanges      []TimeRange `json:"time_ranges,omitempty"`
	DurationMinutes int         `json:"duration_minutes"`
	CreatedAt       time.Time   `json:"created_at"`
}

type SlotRuleInput struct {
	Scope           string
	Date            string
	Weekdays        []int
	StartTimes      []string
	TimeRanges      []TimeRange
	DurationMinutes int
}

type WeeklyScheduleDay struct {
	Day        int
	StartTimes []string
}

type DateSchedule struct {
	Date       string
	TimeRanges []string
}

type RuleStore struct {
	path string
	mu   sync.Mutex
}

func NewRuleStore(path string) *RuleStore {
	return &RuleStore{path: path}
}

func (s *RuleStore) List() ([]SlotRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.loadLocked()
}

func (s *RuleStore) Save(ctx context.Context, rules []SlotRule) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.saveLocked(ctx, rules)
}

func (s *RuleStore) loadLocked() ([]SlotRule, error) {
	payload, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var rules []SlotRule
	if len(strings.TrimSpace(string(payload))) == 0 {
		return nil, nil
	}

	if err := json.Unmarshal(payload, &rules); err != nil {
		return nil, err
	}

	sort.Slice(rules, func(i, j int) bool {
		return rules[i].CreatedAt.Before(rules[j].CreatedAt)
	})

	return rules, nil
}

func (s *RuleStore) saveLocked(ctx context.Context, rules []SlotRule) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	payload, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(s.path), "slot-rules-*.json")
	if err != nil {
		return err
	}

	tmpName := tmp.Name()
	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
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

func defaultSlotRules(now time.Time) []SlotRule {
	return []SlotRule{
		{
			ID:              fmt.Sprintf("default-weekdays-%d", now.UnixNano()),
			Scope:           SlotRuleScopeWeekly,
			Weekdays:        []int{1, 2, 3, 4, 5},
			StartTimes:      []string{"09:00", "11:00", "14:00", "16:00", "18:00"},
			DurationMinutes: 55,
			CreatedAt:       now.UTC(),
		},
		{
			ID:              fmt.Sprintf("default-saturday-%d", now.UnixNano()+1),
			Scope:           SlotRuleScopeWeekly,
			Weekdays:        []int{6},
			StartTimes:      []string{"10:00", "12:00", "15:00"},
			DurationMinutes: 55,
			CreatedAt:       now.UTC(),
		},
	}
}

func normalizeSlotRuleInput(input SlotRuleInput) (SlotRuleInput, error) {
	input.Scope = strings.TrimSpace(input.Scope)
	if input.DurationMinutes == 0 {
		input.DurationMinutes = 55
	}
	if input.DurationMinutes <= 0 {
		return SlotRuleInput{}, fmt.Errorf("invalid duration")
	}

	startTimes := make([]string, 0, len(input.StartTimes))
	seenTimes := make(map[string]bool)
	for _, value := range input.StartTimes {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}

		normalizedValue, err := normalizeClockValue(value)
		if err != nil {
			return SlotRuleInput{}, err
		}
		if seenTimes[normalizedValue] {
			continue
		}
		if err := validateTimeWindow(normalizedValue, input.DurationMinutes); err != nil {
			return SlotRuleInput{}, err
		}
		seenTimes[normalizedValue] = true
		startTimes = append(startTimes, normalizedValue)
	}

	timeRanges := make([]TimeRange, 0, len(input.TimeRanges))
	seenRanges := make(map[string]bool)
	for _, current := range input.TimeRanges {
		normalizedRange, err := normalizeTimeRange(current)
		if err != nil {
			return SlotRuleInput{}, err
		}
		key := normalizedRange.Start + "-" + normalizedRange.End
		if seenRanges[key] {
			continue
		}
		seenRanges[key] = true
		timeRanges = append(timeRanges, normalizedRange)
	}

	switch input.Scope {
	case SlotRuleScopeWeekly:
		if len(startTimes) == 0 {
			return SlotRuleInput{}, fmt.Errorf("empty start times")
		}
		sort.Strings(startTimes)
		input.StartTimes = startTimes

		normalizedWeekdays := make([]int, 0, len(input.Weekdays))
		seenDays := make(map[int]bool)
		for _, day := range input.Weekdays {
			if day < 1 || day > 7 || seenDays[day] {
				continue
			}
			seenDays[day] = true
			normalizedWeekdays = append(normalizedWeekdays, day)
		}
		if len(normalizedWeekdays) == 0 {
			return SlotRuleInput{}, fmt.Errorf("empty weekdays")
		}
		sort.Ints(normalizedWeekdays)
		input.Weekdays = normalizedWeekdays
		input.Date = ""
		input.TimeRanges = nil
	case SlotRuleScopeDate:
		if _, err := time.Parse("2006-01-02", strings.TrimSpace(input.Date)); err != nil {
			return SlotRuleInput{}, fmt.Errorf("invalid date")
		}
		input.Date = strings.TrimSpace(input.Date)
		input.Weekdays = nil
		if len(timeRanges) > 0 {
			sort.Slice(timeRanges, func(i, j int) bool {
				if timeRanges[i].Start == timeRanges[j].Start {
					return timeRanges[i].End < timeRanges[j].End
				}
				return timeRanges[i].Start < timeRanges[j].Start
			})
			if err := validateTimeRanges(timeRanges); err != nil {
				return SlotRuleInput{}, err
			}
			input.TimeRanges = timeRanges
			input.StartTimes = nil
			break
		}
		if len(startTimes) == 0 {
			return SlotRuleInput{}, fmt.Errorf("empty date ranges")
		}
		sort.Strings(startTimes)
		input.StartTimes = startTimes
		input.TimeRanges = nil
	default:
		return SlotRuleInput{}, fmt.Errorf("invalid scope")
	}

	return input, nil
}

func parseClock(value string) (int, error) {
	normalized, err := normalizeClockValue(value)
	if err != nil {
		return 0, fmt.Errorf("invalid time %q", value)
	}

	parts := strings.Split(normalized, ":")
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid time %q", value)
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid time %q", value)
	}

	return hour*60 + minute, nil
}

func normalizeClockValue(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value != "" && !strings.ContainsAny(value, ":.,") {
		hour, err := strconv.Atoi(value)
		if err != nil {
			return "", fmt.Errorf("invalid time %q", value)
		}
		if hour < 0 || hour > 23 {
			return "", fmt.Errorf("invalid time %q", value)
		}
		return fmt.Sprintf("%02d:00", hour), nil
	}
	value = strings.ReplaceAll(value, ".", ":")
	value = strings.ReplaceAll(value, ",", ":")

	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid time %q", value)
	}

	hour, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return "", fmt.Errorf("invalid time %q", value)
	}
	minute, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return "", fmt.Errorf("invalid time %q", value)
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return "", fmt.Errorf("invalid time %q", value)
	}

	return fmt.Sprintf("%02d:%02d", hour, minute), nil
}

func validateTimeWindow(start string, durationMinutes int) error {
	startMinutes, err := parseClock(start)
	if err != nil {
		return err
	}

	dayStart := 9 * 60
	dayEnd := 22*60 + 30
	if startMinutes < dayStart {
		return fmt.Errorf("slot start must be no earlier than 09:00")
	}
	if startMinutes+durationMinutes > dayEnd {
		return fmt.Errorf("slot end must be no later than 22:30")
	}
	return nil
}

func normalizeTimeRange(current TimeRange) (TimeRange, error) {
	start, err := normalizeClockValue(current.Start)
	if err != nil {
		return TimeRange{}, err
	}
	end, err := normalizeClockValue(current.End)
	if err != nil {
		return TimeRange{}, err
	}
	if err := validateTimeRange(start, end); err != nil {
		return TimeRange{}, err
	}
	return TimeRange{Start: start, End: end}, nil
}

func validateTimeRanges(ranges []TimeRange) error {
	for i, current := range ranges {
		if err := validateTimeRange(current.Start, current.End); err != nil {
			return err
		}
		if i == 0 {
			continue
		}
		prevEnd, err := parseClock(ranges[i-1].End)
		if err != nil {
			return err
		}
		currentStart, err := parseClock(current.Start)
		if err != nil {
			return err
		}
		if currentStart < prevEnd {
			return fmt.Errorf("overlapping time ranges")
		}
	}
	return nil
}

func validateTimeRange(start, end string) error {
	startMinutes, err := parseClock(start)
	if err != nil {
		return err
	}
	endMinutes, err := parseClock(end)
	if err != nil {
		return err
	}
	dayStart := 9 * 60
	dayEnd := 22*60 + 30
	if startMinutes < dayStart {
		return fmt.Errorf("slot start must be no earlier than 09:00")
	}
	if endMinutes > dayEnd {
		return fmt.Errorf("slot end must be no later than 22:30")
	}
	if endMinutes <= startMinutes {
		return fmt.Errorf("range end must be later than start")
	}
	return nil
}
