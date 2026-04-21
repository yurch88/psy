package calendar

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"
)

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
	endDay := startDay.AddDate(0, 0, 90)

	rules, err := s.rules.List()
	if err != nil {
		return nil
	}

	slots := make([]Slot, 0, 128)
	seen := make(map[string]bool)

	for day := startDay; day.Before(endDay); day = day.AddDate(0, 0, 1) {
		for _, rule := range rules {
			if !ruleAppliesToDate(rule, day) {
				continue
			}
			for _, startTime := range rule.StartTimes {
				startMinutes, err := parseClock(startTime)
				if err != nil {
					continue
				}

				start := time.Date(day.Year(), day.Month(), day.Day(), startMinutes/60, startMinutes%60, 0, 0, s.location)
				end := start.Add(time.Duration(rule.DurationMinutes) * time.Minute)
				if !start.After(now.Add(2 * time.Hour)) {
					continue
				}
				if !end.After(start) {
					continue
				}

				id := start.Format("20060102T1504")
				if seen[id] {
					continue
				}
				seen[id] = true
				slots = append(slots, Slot{
					ID:       id,
					Start:    start,
					End:      end,
					Disabled: reserved[id],
				})
			}
		}
	}

	sort.Slice(slots, func(i, j int) bool {
		return slots[i].Start.Before(slots[j].Start)
	})

	return slots
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
