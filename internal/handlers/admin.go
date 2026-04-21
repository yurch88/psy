package handlers

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"

	"psy/internal/calendar"
)

const adminSessionCookieName = "administrator_session"

func (h *Handler) administrator(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	data := PageData{
		Title:          "Администратор - " + h.site.Brand,
		Description:    "Вход в административный раздел сайта.",
		Site:           h.site,
		AdminEnabled:   h.adminConfigured(),
		HideSiteChrome: true,
	}

	if !data.AdminEnabled {
		h.render(w, "administrator", data)
		return
	}

	if !h.isAdminAuthenticated(r) {
		h.render(w, "administrator", data)
		return
	}

	bookings, err := h.calendar.Bookings()
	if err != nil {
		h.logger.Error("load admin bookings", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		data.AdminAuthenticated = true
		data.AdminError = "Не удалось загрузить заявки. Обновите страницу чуть позже."
		h.render(w, "administrator", data)
		return
	}

	data.AdminAuthenticated = true
	data.AdminBookings = h.adminBookingViews(bookings)
	h.render(w, "administrator", data)
}

func (h *Handler) administratorLogin(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	data := PageData{
		Title:          "Администратор - " + h.site.Brand,
		Description:    "Вход в административный раздел сайта.",
		Site:           h.site,
		AdminEnabled:   h.adminConfigured(),
		HideSiteChrome: true,
	}

	if !data.AdminEnabled {
		w.WriteHeader(http.StatusServiceUnavailable)
		data.AdminError = "Админ-панель пока не настроена. Добавьте логин и пароль в .env."
		h.render(w, "administrator", data)
		return
	}

	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		data.AdminError = "Не удалось обработать форму входа."
		h.render(w, "administrator", data)
		return
	}

	login := strings.TrimSpace(r.FormValue("login"))
	password := r.FormValue("password")
	data.AdminLogin = login

	if !h.validAdminCredentials(login, password) {
		w.WriteHeader(http.StatusUnauthorized)
		data.AdminError = "Неверный логин или пароль."
		h.render(w, "administrator", data)
		return
	}

	h.setAdminSession(w, r)
	http.Redirect(w, r, "/administrator", http.StatusSeeOther)
}

func (h *Handler) administratorLogout(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	h.clearAdminSession(w, r)
	http.Redirect(w, r, "/administrator", http.StatusSeeOther)
}

func (h *Handler) adminConfigured() bool {
	return strings.TrimSpace(h.adminLogin) != "" && h.adminPass != ""
}

func (h *Handler) validAdminCredentials(login, password string) bool {
	if !h.adminConfigured() {
		return false
	}

	loginMatch := subtle.ConstantTimeCompare([]byte(login), []byte(h.adminLogin)) == 1
	passwordMatch := subtle.ConstantTimeCompare([]byte(password), []byte(h.adminPass)) == 1
	return loginMatch && passwordMatch
}

func (h *Handler) isAdminAuthenticated(r *http.Request) bool {
	if !h.adminConfigured() {
		return false
	}

	cookie, err := r.Cookie(adminSessionCookieName)
	if err != nil {
		return false
	}

	expected := h.adminSessionValue()
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(expected)) == 1
}

func (h *Handler) setAdminSession(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     adminSessionCookieName,
		Value:    h.adminSessionValue(),
		Path:     "/administrator",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestIsHTTPS(r),
		MaxAge:   60 * 60 * 12,
	})
}

func (h *Handler) clearAdminSession(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     adminSessionCookieName,
		Value:    "",
		Path:     "/administrator",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestIsHTTPS(r),
		MaxAge:   -1,
	})
}

func (h *Handler) adminSessionValue() string {
	sum := sha256.Sum256([]byte(h.adminLogin + "\x00" + h.adminPass))
	return hex.EncodeToString(sum[:])
}

func requestIsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func (h *Handler) adminBookingViews(bookings []calendar.Booking) []AdminBookingView {
	views := make([]AdminBookingView, 0, len(bookings))
	for _, booking := range bookings {
		views = append(views, AdminBookingView{
			ID:             booking.ID,
			Name:           booking.Name,
			Email:          booking.Email,
			Phone:          booking.Phone,
			ClientTimezone: booking.ClientTimezone,
			Comment:        booking.Comment,
			SlotLabel:      booking.Start.Format("02.01.2006 15:04") + " - " + booking.End.Format("15:04"),
			CreatedAtLabel: booking.CreatedAt.In(booking.Start.Location()).Format("02.01.2006 15:04"),
			StatusLabel:    adminStatusLabel(booking),
			StatusClass:    adminStatusClass(booking),
		})
	}
	return views
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
	default:
		return "Ожидает подтверждения"
	}
}

func adminStatusClass(booking calendar.Booking) string {
	switch booking.EffectiveStatus() {
	case calendar.BookingStatusConfirmed:
		return "is-confirmed"
	case calendar.BookingStatusRejected:
		return "is-rejected"
	default:
		return "is-pending"
	}
}
