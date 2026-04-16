package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"psy/internal/calendar"
	"psy/internal/content"
	"psy/internal/rates"
	"psy/internal/ui"
)

type BookingNotifier interface {
	NotifyBooking(context.Context, calendar.Booking) error
}

type Handler struct {
	site     content.Site
	renderer *ui.Renderer
	calendar *calendar.Service
	rates    *rates.Service
	notifier BookingNotifier
	logger   *slog.Logger
}

type PageData struct {
	Title       string
	Description string
	Site        content.Site
	USDNote     string
	Slots       []SlotView
	Form        BookingForm
	Errors      []string
	Booking     *calendar.Booking
}

type SlotView struct {
	ID        string
	Date      string
	Weekday   string
	TimeRange string
	StartISO  string
	EndISO    string
	Disabled  bool
}

type BookingForm struct {
	SlotID         string
	Name           string
	Email          string
	Phone          string
	ClientTimezone string
	Comment        string
}

func New(site content.Site, renderer *ui.Renderer, calendarService *calendar.Service, rateService *rates.Service, notifier BookingNotifier, logger *slog.Logger) *Handler {
	return &Handler{
		site:     site,
		renderer: renderer,
		calendar: calendarService,
		rates:    rateService,
		notifier: notifier,
		logger:   logger,
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/", h.home)
	mux.HandleFunc("/rules", h.rules)
	mux.HandleFunc("/memo", h.memo)
	mux.HandleFunc("/booking", h.booking)
	mux.HandleFunc("/booking/submit", h.submitBooking)
	mux.HandleFunc("/healthz", h.healthz)
}

func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	usdNote, _ := h.rates.RubleEquivalent(r.Context())
	h.render(w, "home", PageData{
		Title:       h.site.Brand,
		Description: h.site.Description,
		Site:        h.site,
		USDNote:     usdNote,
	})
}

func (h *Handler) rules(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	h.render(w, "rules", PageData{
		Title:       h.site.Rules.Title + " - " + h.site.Brand,
		Description: h.site.Rules.Subtitle,
		Site:        h.site,
	})
}

func (h *Handler) memo(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	h.render(w, "memo", PageData{
		Title:       h.site.Memo.Title + " - " + h.site.Brand,
		Description: h.site.Memo.Subtitle,
		Site:        h.site,
	})
}

func (h *Handler) booking(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	h.renderBooking(w, PageData{
		Title:       h.site.Booking.Title + " - " + h.site.Brand,
		Description: h.site.Booking.Description[0],
		Site:        h.site,
	})
}

func (h *Handler) submitBooking(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Некорректная форма", http.StatusBadRequest)
		return
	}

	form := BookingForm{
		SlotID:         r.FormValue("slot_id"),
		Name:           r.FormValue("name"),
		Email:          r.FormValue("email"),
		Phone:          r.FormValue("phone"),
		ClientTimezone: r.FormValue("timezone"),
		Comment:        r.FormValue("comment"),
	}

	booking, err := h.calendar.Book(r.Context(), calendar.BookingRequest{
		SlotID:         form.SlotID,
		Name:           form.Name,
		Email:          form.Email,
		Phone:          form.Phone,
		ClientTimezone: form.ClientTimezone,
		Comment:        form.Comment,
	})
	if err != nil {
		var validation calendar.ValidationError
		if errors.As(err, &validation) {
			w.WriteHeader(http.StatusBadRequest)
			h.renderBooking(w, PageData{
				Title:       h.site.Booking.Title + " - " + h.site.Brand,
				Description: h.site.Booking.Description[0],
				Site:        h.site,
				Form:        form,
				Errors:      []string(validation),
			})
			return
		}
		h.logger.Error("save booking", "error", err)
		http.Error(w, "Не удалось сохранить заявку", http.StatusInternalServerError)
		return
	}

	if h.notifier != nil {
		if err := h.notifier.NotifyBooking(r.Context(), booking); err != nil {
			h.logger.Error("notify booking", "booking_id", booking.ID, "error", err)
		}
	}

	h.render(w, "thanks", PageData{
		Title:       "Заявка принята - " + h.site.Brand,
		Description: "Заявка на консультацию сохранена и отправлена на подтверждение.",
		Site:        h.site,
		Booking:     &booking,
	})
}

func (h *Handler) healthz(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) renderBooking(w http.ResponseWriter, data PageData) {
	data.Slots = h.slotViews(h.calendar.Slots())
	h.render(w, "booking", data)
}

func (h *Handler) render(w http.ResponseWriter, page string, data PageData) {
	if data.Site.Brand == "" {
		data.Site = h.site
	}
	if err := h.renderer.Render(w, page, data); err != nil {
		h.logger.Error("render page", "page", page, "error", err)
	}
}

func (h *Handler) slotViews(slots []calendar.Slot) []SlotView {
	views := make([]SlotView, 0, len(slots))
	for _, slot := range slots {
		views = append(views, SlotView{
			ID:        slot.ID,
			Date:      slot.Start.Format("02.01"),
			Weekday:   weekdayRU(slot.Start.Weekday()),
			TimeRange: slot.Start.Format("15:04") + "-" + slot.End.Format("15:04"),
			StartISO:  slot.Start.Format(time.RFC3339),
			EndISO:    slot.End.Format(time.RFC3339),
			Disabled:  slot.Disabled,
		})
	}
	return views
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	w.Header().Set("Allow", method)
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	return false
}

func weekdayRU(day time.Weekday) string {
	switch day {
	case time.Monday:
		return "понедельник"
	case time.Tuesday:
		return "вторник"
	case time.Wednesday:
		return "среда"
	case time.Thursday:
		return "четверг"
	case time.Friday:
		return "пятница"
	case time.Saturday:
		return "суббота"
	default:
		return "воскресенье"
	}
}
