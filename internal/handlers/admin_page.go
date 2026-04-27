package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"psy/internal/calendar"
	"psy/internal/content"
)

func (h *Handler) administrator(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	h.renderAdministratorPage(w, r, PageData{}, http.StatusOK)
}

func (h *Handler) renderAdministratorPage(w http.ResponseWriter, r *http.Request, overrides PageData, status int) {
	site := h.currentSite()
	data := PageData{
		Title:               "Администратор - " + site.Brand,
		Description:         "Вход в административный раздел сайта.",
		Site:                site,
		AdminEnabled:        h.adminConfigured(),
		AdminTab:            adminTab(r.URL.Query().Get("tab")),
		AdminContentSection: adminContentSection(r.URL.Query().Get("section")),
		HideSiteChrome:      true,
	}

	if data.AdminTab == "" {
		data.AdminTab = "calendar"
	}
	if data.AdminContentSection == "" {
		data.AdminContentSection = "main"
	}
	if overrides.AdminTab != "" {
		data.AdminTab = overrides.AdminTab
	}
	if overrides.AdminContentSection != "" {
		data.AdminContentSection = overrides.AdminContentSection
	}

	if overrides.AdminLogin != "" {
		data.AdminLogin = overrides.AdminLogin
	}
	if overrides.AdminError != "" {
		data.AdminError = overrides.AdminError
	}
	if overrides.AdminNotice != "" {
		data.AdminNotice = overrides.AdminNotice
		data.AdminNoticeClass = overrides.AdminNoticeClass
	}
	if len(overrides.AdminWeeklySchedule) > 0 {
		data.AdminWeeklySchedule = overrides.AdminWeeklySchedule
	}
	if overrides.AdminSlotMode != "" {
		data.AdminSlotMode = overrides.AdminSlotMode
	}
	if overrides.AdminSlotDate != "" {
		data.AdminSlotDate = overrides.AdminSlotDate
	}
	if overrides.AdminSlotTimes != "" {
		data.AdminSlotTimes = overrides.AdminSlotTimes
	}
	if len(overrides.AdminSlotWeekdays) > 0 {
		data.AdminSlotWeekdays = overrides.AdminSlotWeekdays
	}

	if data.AdminNotice == "" {
		data.AdminNotice, data.AdminNoticeClass = adminNotice(r.URL.Query().Get("notice"))
	}

	if data.AdminSlotMode == "" {
		data.AdminSlotMode = "weekly"
	}

	if !data.AdminEnabled {
		if status != http.StatusOK {
			w.WriteHeader(status)
		}
		h.render(w, "administrator", data)
		return
	}

	if !h.isAdminAuthenticated(r) {
		if status != http.StatusOK {
			w.WriteHeader(status)
		}
		h.render(w, "administrator", data)
		return
	}

	data.AdminAuthenticated = true
	data.AdminWeeklySchedule = h.adminWeeklyScheduleViews()
	data.AdminSlotRules = h.adminSlotRuleViews()
	data.AdminDateSlotOptions = adminDateSlotOptions()
	data.AdminBookings = h.adminBookingViews()
	data.AdminAvailableSlots = h.adminAvailableSlotOptions()
	data.AdminContentForm = h.adminContentForm()
	if overrides.AdminContentForm != (AdminContentForm{}) {
		data.AdminContentForm = overrides.AdminContentForm
	}

	if status != http.StatusOK {
		w.WriteHeader(status)
	}
	h.render(w, "administrator", data)
}

func (h *Handler) administratorRequireAuth(w http.ResponseWriter, r *http.Request) bool {
	if !h.adminConfigured() {
		h.renderAdministratorPage(w, r, PageData{AdminError: "Админ-панель пока не настроена. Добавьте логин и пароль в .env."}, http.StatusServiceUnavailable)
		return false
	}
	if !h.isAdminAuthenticated(r) {
		http.Redirect(w, r, "/administrator", http.StatusSeeOther)
		return false
	}
	return true
}

func adminTab(value string) string {
	switch strings.TrimSpace(value) {
	case "calendar", "bookings", "content":
		return value
	default:
		return "calendar"
	}
}

func adminContentSection(value string) string {
	switch strings.TrimSpace(value) {
	case "main", "home", "pricing", "booking", "memo", "rules", "privacy":
		return value
	default:
		return "main"
	}
}

func adminNotice(code string) (string, string) {
	switch code {
	case "slot-created":
		return "Слоты сохранены.", "is-success"
	case "weekly-schedule-saved":
		return "Расписание по дням недели сохранено.", "is-success"
	case "slot-deleted":
		return "Правило слотов удалено.", "is-success"
	case "booking-cancelled":
		return "Бронь отменена, слот снова доступен.", "is-success"
	case "booking-rescheduled":
		return "Бронь перенесена и клиенту отправлено уведомление.", "is-success"
	case "draft-saved":
		return "Черновик сохранён.", "is-success"
	case "content-published":
		return "Изменения опубликованы.", "is-success"
	default:
		return "", ""
	}
}

func (h *Handler) adminSlotRuleViews() []AdminSlotRuleView {
	rules, err := h.calendar.Rules()
	if err != nil {
		h.logger.Error("list slot rules", "error", err)
		return nil
	}

	views := make([]AdminSlotRuleView, 0, len(rules))
	for _, rule := range rules {
		if rule.Scope != calendar.SlotRuleScopeDate {
			continue
		}
		view := AdminSlotRuleView{
			ID:         rule.ID,
			TimesLabel: strings.Join(rule.StartTimes, ", "),
		}
		view.ScopeLabel = "Только на дату"
		view.PatternLabel = rule.Date
		if view.TimesLabel == "" {
			view.TimesLabel = "Слоты скрыты на эту дату"
		}
		views = append(views, view)
	}
	return views
}

func (h *Handler) adminWeeklyScheduleViews() []AdminWeekdayScheduleView {
	schedule, err := h.calendar.WeeklySchedule()
	if err != nil {
		h.logger.Error("list weekly schedule", "error", err)
		return defaultAdminWeeklyScheduleViews()
	}

	views := make([]AdminWeekdayScheduleView, 0, len(schedule))
	for _, day := range schedule {
		views = append(views, AdminWeekdayScheduleView{
			Day:     day.Day,
			Label:   adminWeekdayName(day.Day),
			Enabled: len(day.StartTimes) > 0,
			Times:   strings.Join(day.StartTimes, "\n"),
		})
	}
	if len(views) == 0 {
		return defaultAdminWeeklyScheduleViews()
	}
	return views
}

func defaultAdminWeeklyScheduleViews() []AdminWeekdayScheduleView {
	views := make([]AdminWeekdayScheduleView, 0, 7)
	for day := 1; day <= 7; day++ {
		views = append(views, AdminWeekdayScheduleView{
			Day:   day,
			Label: adminWeekdayName(day),
		})
	}
	return views
}

func adminWeekdaysLabel(days []int) string {
	labels := make([]string, 0, len(days))
	for _, day := range days {
		switch day {
		case 1:
			labels = append(labels, "понедельники")
		case 2:
			labels = append(labels, "вторники")
		case 3:
			labels = append(labels, "среды")
		case 4:
			labels = append(labels, "четверги")
		case 5:
			labels = append(labels, "пятницы")
		case 6:
			labels = append(labels, "субботы")
		case 7:
			labels = append(labels, "воскресенья")
		}
	}
	return strings.Join(labels, ", ")
}

func adminWeekdayName(day int) string {
	switch day {
	case 1:
		return "Понедельник"
	case 2:
		return "Вторник"
	case 3:
		return "Среда"
	case 4:
		return "Четверг"
	case 5:
		return "Пятница"
	case 6:
		return "Суббота"
	default:
		return "Воскресенье"
	}
}

func adminDateSlotOptions() []AdminDateSlotOption {
	options := make([]AdminDateSlotOption, 0, 160)
	startMinutes := 9 * 60
	lastStartMinutes := 21*60 + 35
	durationMinutes := 55

	for value := startMinutes; value <= lastStartMinutes; value += 25 {
		startHour := value / 60
		startMinute := value % 60
		endValue := value + durationMinutes
		endHour := endValue / 60
		endMinute := endValue % 60

		options = append(options, AdminDateSlotOption{
			Value: fmt.Sprintf("%02d:%02d", startHour, startMinute),
			End:   fmt.Sprintf("%02d:%02d", endHour, endMinute),
			Label: fmt.Sprintf("%02d:%02d-%02d:%02d", startHour, startMinute, endHour, endMinute),
		})
	}

	return options
}

func (h *Handler) adminBookingViews() []AdminBookingView {
	bookings, err := h.calendar.Bookings()
	if err != nil {
		h.logger.Error("list admin bookings", "error", err)
		return nil
	}

	views := make([]AdminBookingView, 0, len(bookings))
	for _, booking := range bookings {
		views = append(views, AdminBookingView{
			ID:             booking.ID,
			Name:           booking.Name,
			Email:          booking.Email,
			Phone:          booking.Phone,
			ClientTimezone: booking.ClientTimezone,
			Comment:        booking.Comment,
			CurrentSlotID:  booking.SlotID,
			SlotLabel:      booking.Start.Format("02.01.2006 15:04") + " - " + booking.End.Format("15:04"),
			CreatedAtLabel: booking.CreatedAt.In(booking.Start.Location()).Format("02.01.2006 15:04"),
			StatusLabel:    adminStatusLabel(booking),
			StatusClass:    adminStatusClass(booking),
		})
	}
	return views
}

func (h *Handler) adminAvailableSlotOptions() []AdminSlotOption {
	slots := h.calendar.AvailableSlots()
	options := make([]AdminSlotOption, 0, len(slots))
	for _, slot := range slots {
		options = append(options, AdminSlotOption{
			ID:    slot.ID,
			Label: slot.Start.Format("02.01.2006 15:04") + " - " + slot.End.Format("15:04"),
		})
	}
	return options
}

func adminStatusLabel(booking calendar.Booking) string {
	switch booking.EffectiveStatus() {
	case calendar.BookingStatusConfirmed:
		return "Подтверждена"
	case calendar.BookingStatusRejected:
		if booking.Resolution == calendar.ResolutionSlotTaken {
			return "Отклонена: слот занят"
		}
		return "Отклонена"
	case calendar.BookingStatusCancelled:
		return "Отменена"
	default:
		return "Ожидает подтверждения"
	}
}

func adminStatusClass(booking calendar.Booking) string {
	switch booking.EffectiveStatus() {
	case calendar.BookingStatusConfirmed:
		return "is-confirmed"
	case calendar.BookingStatusRejected, calendar.BookingStatusCancelled:
		return "is-rejected"
	default:
		return "is-pending"
	}
}

func (h *Handler) draftSite() content.Site {
	if h.contentManager != nil {
		return h.contentManager.Draft()
	}
	return h.currentSite()
}

func adminFormError(err error, fallback string) string {
	if err == nil {
		return fallback
	}
	var validation calendar.ValidationError
	if errors.As(err, &validation) && len(validation) > 0 {
		return strings.Join(validation, ". ")
	}
	return fallback
}
