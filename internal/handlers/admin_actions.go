package handlers

import (
	"errors"
	"net/http"
	"strings"

	"psy/internal/calendar"
)

func (h *Handler) administratorSlotsCreate(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) || !h.administratorRequireAuth(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderAdministratorPage(w, r, PageData{
			AdminTab:       "calendar",
			AdminError:     "Не удалось обработать форму слотов.",
			AdminSlotMode:  "weekly",
			AdminSlotTimes: "",
		}, http.StatusBadRequest)
		return
	}

	weekdays := make([]int, 0, len(r.Form["weekday"]))
	for _, value := range r.Form["weekday"] {
		switch strings.TrimSpace(value) {
		case "1", "2", "3", "4", "5", "6", "7":
			weekdays = append(weekdays, int(value[0]-'0'))
		}
	}

	times := splitTextareaLines(r.FormValue("times"))
	mode := strings.TrimSpace(r.FormValue("mode"))
	date := strings.TrimSpace(r.FormValue("date"))
	if mode == "" {
		mode = calendar.SlotRuleScopeWeekly
	}

	_, err := h.calendar.AddRule(r.Context(), calendar.SlotRuleInput{
		Scope:           mode,
		Date:            date,
		Weekdays:        weekdays,
		StartTimes:      times,
		DurationMinutes: 55,
	})
	if err != nil {
		h.renderAdministratorPage(w, r, PageData{
			AdminTab:          "calendar",
			AdminError:        adminCalendarError(err),
			AdminSlotMode:     mode,
			AdminSlotDate:     date,
			AdminSlotTimes:    strings.Join(times, "\n"),
			AdminSlotWeekdays: weekdays,
		}, http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, "/administrator?tab=calendar&notice=slot-created", http.StatusSeeOther)
}

func (h *Handler) administratorSlotsDelete(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) || !h.administratorRequireAuth(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/administrator?tab=calendar", http.StatusSeeOther)
		return
	}

	if err := h.calendar.DeleteRule(r.Context(), strings.TrimSpace(r.FormValue("rule_id"))); err != nil {
		h.renderAdministratorPage(w, r, PageData{
			AdminTab:   "calendar",
			AdminError: "Не удалось удалить правило слотов.",
		}, http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, "/administrator?tab=calendar&notice=slot-deleted", http.StatusSeeOther)
}

func (h *Handler) administratorBookingCancel(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) || !h.administratorRequireAuth(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/administrator?tab=bookings", http.StatusSeeOther)
		return
	}

	booking, err := h.calendar.Cancel(r.Context(), strings.TrimSpace(r.FormValue("booking_id")))
	if err != nil {
		h.renderAdministratorPage(w, r, PageData{
			AdminTab:   "bookings",
			AdminError: "Не удалось отменить бронь.",
		}, http.StatusBadRequest)
		return
	}
	if h.emailer != nil {
		if err := h.emailer.SendBookingCancellation(r.Context(), booking); err != nil {
			h.logger.Error("send cancellation email", "booking_id", booking.ID, "error", err)
		}
	}

	http.Redirect(w, r, "/administrator?tab=bookings&notice=booking-cancelled", http.StatusSeeOther)
}

func (h *Handler) administratorBookingReschedule(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) || !h.administratorRequireAuth(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/administrator?tab=bookings", http.StatusSeeOther)
		return
	}

	bookingID := strings.TrimSpace(r.FormValue("booking_id"))
	newSlotID := strings.TrimSpace(r.FormValue("slot_id"))

	booking, err := h.calendar.Reschedule(r.Context(), bookingID, newSlotID)
	if err != nil {
		message := "Не удалось перенести бронь."
		switch {
		case errors.Is(err, calendar.ErrSlotNotFound):
			message = "Выберите доступный слот для переноса."
		case errors.Is(err, calendar.ErrSlotAlreadyTaken):
			message = "Этот слот уже занят другой бронью."
		}
		h.renderAdministratorPage(w, r, PageData{
			AdminTab:   "bookings",
			AdminError: message,
		}, http.StatusBadRequest)
		return
	}

	if h.emailer != nil {
		if err := h.emailer.SendBookingRescheduled(r.Context(), booking); err != nil {
			h.logger.Error("send rescheduled email", "booking_id", booking.ID, "error", err)
		}
	}

	http.Redirect(w, r, "/administrator?tab=bookings&notice=booking-rescheduled", http.StatusSeeOther)
}

func adminCalendarError(err error) string {
	switch {
	case err == nil:
		return ""
	case strings.Contains(err.Error(), "09:00"):
		return "Время слота должно начинаться не раньше 09:00."
	case strings.Contains(err.Error(), "22:30"):
		return "Время слота должно заканчиваться не позже 22:30."
	case strings.Contains(err.Error(), "empty weekdays"):
		return "Выберите хотя бы один день недели."
	case strings.Contains(err.Error(), "empty start times"):
		return "Добавьте хотя бы одно время слота."
	case strings.Contains(err.Error(), "invalid date"):
		return "Укажите корректную дату."
	default:
		return "Не удалось сохранить слоты."
	}
}

func splitTextareaLines(value string) []string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	lines := strings.Split(value, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}
