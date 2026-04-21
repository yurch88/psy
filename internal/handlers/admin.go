package handlers

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
)

const adminSessionCookieName = "administrator_session"

func (h *Handler) administratorLogin(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	if !h.adminConfigured() {
		h.renderAdministratorPage(w, r, PageData{
			AdminError: "Админ-панель пока не настроена. Добавьте логин и пароль в .env.",
		}, http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.renderAdministratorPage(w, r, PageData{
			AdminError: "Не удалось обработать форму входа.",
		}, http.StatusBadRequest)
		return
	}

	login := strings.TrimSpace(r.FormValue("login"))
	password := r.FormValue("password")

	if !h.validAdminCredentials(login, password) {
		h.renderAdministratorPage(w, r, PageData{
			AdminLogin: login,
			AdminError: "Неверный логин или пароль.",
		}, http.StatusUnauthorized)
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
