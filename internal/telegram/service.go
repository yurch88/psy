package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"psy/internal/calendar"
)

const apiBase = "https://api.telegram.org"

type Service struct {
	token         string
	chatIDs       []string
	calendar      *calendar.Service
	confirmations ConfirmationSender
	logger        *slog.Logger
	client        *http.Client
}

type ConfirmationSender interface {
	SendBookingConfirmation(context.Context, calendar.Booking) error
}

type Update struct {
	UpdateID      int            `json:"update_id"`
	CallbackQuery *CallbackQuery `json:"callback_query,omitempty"`
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	Data    string   `json:"data"`
	Message *Message `json:"message,omitempty"`
}

type Message struct {
	MessageID int `json:"message_id"`
}

type sendMessageRequest struct {
	ChatID      string          `json:"chat_id"`
	Text        string          `json:"text"`
	ParseMode   string          `json:"parse_mode,omitempty"`
	ReplyMarkup *inlineKeyboard `json:"reply_markup,omitempty"`
}

type editMessageTextRequest struct {
	ChatID      string          `json:"chat_id"`
	MessageID   int             `json:"message_id"`
	Text        string          `json:"text"`
	ParseMode   string          `json:"parse_mode,omitempty"`
	ReplyMarkup *inlineKeyboard `json:"reply_markup,omitempty"`
}

type callbackAnswerRequest struct {
	CallbackQueryID string `json:"callback_query_id"`
	Text            string `json:"text,omitempty"`
	ShowAlert       bool   `json:"show_alert,omitempty"`
}

type getUpdatesRequest struct {
	Offset         int      `json:"offset,omitempty"`
	Timeout        int      `json:"timeout,omitempty"`
	AllowedUpdates []string `json:"allowed_updates,omitempty"`
	Limit          int      `json:"limit,omitempty"`
}

type deleteWebhookRequest struct {
	DropPendingUpdates bool `json:"drop_pending_updates"`
}

type apiResponse[T any] struct {
	OK          bool   `json:"ok"`
	Result      T      `json:"result"`
	Description string `json:"description,omitempty"`
}

type inlineKeyboard struct {
	InlineKeyboard [][]inlineButton `json:"inline_keyboard"`
}

type inlineButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}

type sentMessage struct {
	MessageID int `json:"message_id"`
}

func New(token string, chatIDs []string, calendarService *calendar.Service, confirmations ConfirmationSender, logger *slog.Logger) *Service {
	normalizedIDs := make([]string, 0, len(chatIDs))
	seen := make(map[string]bool)

	for _, chatID := range chatIDs {
		chatID = strings.TrimSpace(chatID)
		if chatID == "" || seen[chatID] {
			continue
		}
		seen[chatID] = true
		normalizedIDs = append(normalizedIDs, chatID)
	}

	return &Service{
		token:         strings.TrimSpace(token),
		chatIDs:       normalizedIDs,
		calendar:      calendarService,
		confirmations: confirmations,
		logger:        logger,
		client:        &http.Client{Timeout: 70 * time.Second},
	}
}

func (s *Service) Enabled() bool {
	return s != nil && s.token != ""
}

func (s *Service) Run(ctx context.Context) {
	if !s.Enabled() {
		return
	}

	if err := s.deleteWebhook(ctx); err != nil {
		s.logger.Warn("telegram deleteWebhook failed", "error", err)
	}

	offset := 0
	for {
		if ctx.Err() != nil {
			return
		}

		updates, err := s.getUpdates(ctx, offset)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			s.logger.Error("telegram getUpdates failed", "error", err)
			if !sleepWithContext(ctx, 3*time.Second) {
				return
			}
			continue
		}

		for _, update := range updates {
			if update.UpdateID >= offset {
				offset = update.UpdateID + 1
			}
			if err := s.handleUpdate(ctx, update); err != nil {
				s.logger.Error("telegram update failed", "update_id", update.UpdateID, "error", err)
			}
		}
	}
}

func (s *Service) NotifyBooking(ctx context.Context, booking calendar.Booking) error {
	if !s.Enabled() || len(s.chatIDs) == 0 {
		return nil
	}

	payload := sendMessageRequest{
		Text:        s.renderBookingText(booking),
		ParseMode:   "HTML",
		ReplyMarkup: s.replyMarkup(booking),
	}

	refs := make([]calendar.NotificationRef, 0, len(s.chatIDs))
	var sendErrors []error

	for _, chatID := range s.chatIDs {
		payload.ChatID = chatID

		var sent sentMessage
		if err := s.apiCall(ctx, "sendMessage", payload, &sent); err != nil {
			sendErrors = append(sendErrors, fmt.Errorf("send to %s: %w", chatID, err))
			continue
		}

		refs = append(refs, calendar.NotificationRef{
			ChatID:    chatID,
			MessageID: sent.MessageID,
		})
	}

	if len(refs) > 0 {
		if _, err := s.calendar.AttachNotifications(ctx, booking.ID, refs); err != nil {
			sendErrors = append(sendErrors, fmt.Errorf("store notification refs: %w", err))
		}
	}

	return errors.Join(sendErrors...)
}

func (s *Service) handleUpdate(ctx context.Context, update Update) error {
	if update.CallbackQuery == nil {
		return nil
	}

	action, bookingID, err := parseCallbackData(update.CallbackQuery.Data)
	if err != nil {
		_ = s.answerCallback(ctx, update.CallbackQuery.ID, "Не удалось обработать действие")
		return err
	}

	result, err := s.calendar.Review(ctx, bookingID, action)
	if err != nil {
		if errors.Is(err, calendar.ErrBookingNotFound) {
			_ = s.answerCallback(ctx, update.CallbackQuery.ID, "Заявка не найдена")
			return nil
		}
		_ = s.answerCallback(ctx, update.CallbackQuery.ID, "Не удалось обработать заявку")
		return err
	}

	if err := s.answerCallback(ctx, update.CallbackQuery.ID, result.CallbackText); err != nil {
		s.logger.Warn("telegram answerCallbackQuery failed", "error", err)
	}

	updated := result.Updated
	if len(updated) == 0 {
		updated = []calendar.Booking{result.Booking}
	}

	for _, booking := range updated {
		if err := s.syncBookingNotifications(ctx, booking); err != nil {
			s.logger.Warn("telegram sync booking notifications failed", "booking_id", booking.ID, "error", err)
		}
	}

	if action == calendar.ReviewActionConfirm && result.TransitionedToConfirmed && s.confirmations != nil {
		if err := s.confirmations.SendBookingConfirmation(ctx, result.Booking); err != nil {
			s.logger.Error("send booking confirmation email", "booking_id", result.Booking.ID, "error", err)
		}
	}

	return nil
}

func (s *Service) syncBookingNotifications(ctx context.Context, booking calendar.Booking) error {
	if len(booking.Notifications) == 0 {
		return nil
	}

	request := editMessageTextRequest{
		Text:        s.renderBookingText(booking),
		ParseMode:   "HTML",
		ReplyMarkup: s.replyMarkup(booking),
	}

	var syncErrors []error
	for _, ref := range booking.Notifications {
		request.ChatID = ref.ChatID
		request.MessageID = ref.MessageID

		if err := s.apiCall(ctx, "editMessageText", request, nil); err != nil {
			if strings.Contains(err.Error(), "message is not modified") {
				continue
			}
			syncErrors = append(syncErrors, fmt.Errorf("edit %s/%d: %w", ref.ChatID, ref.MessageID, err))
		}
	}

	return errors.Join(syncErrors...)
}

func (s *Service) renderBookingText(booking calendar.Booking) string {
	title := "Новая заявка на консультацию"
	switch booking.EffectiveStatus() {
	case calendar.BookingStatusConfirmed:
		title = "✅ Заявка подтверждена"
	case calendar.BookingStatusRejected:
		if booking.Resolution == calendar.ResolutionSlotTaken {
			title = "⏹ Слот уже подтвержден другой заявкой"
		} else {
			title = "❌ Заявка отклонена"
		}
	}

	comment := "—"
	if strings.TrimSpace(booking.Comment) != "" {
		comment = html.EscapeString(booking.Comment)
	}

	timezone := "не указан"
	if strings.TrimSpace(booking.ClientTimezone) != "" {
		timezone = html.EscapeString(booking.ClientTimezone)
	}

	statusLine := "Ожидает решения"
	switch booking.EffectiveStatus() {
	case calendar.BookingStatusConfirmed:
		statusLine = "Подтверждена"
	case calendar.BookingStatusRejected:
		if booking.Resolution == calendar.ResolutionSlotTaken {
			statusLine = "Отклонена автоматически: слот уже занят"
		} else {
			statusLine = "Отклонена"
		}
	}

	return fmt.Sprintf(
		"<b>%s</b>\n\n<b>Статус:</b> %s\n<b>Слот (Мск):</b> %s, %s\n<b>ФИО:</b> %s\n<b>E-mail:</b> %s\n<b>Телефон:</b> %s\n<b>Часовой пояс клиента:</b> %s\n<b>Комментарий:</b> %s\n<b>ID заявки:</b> <code>%s</code>",
		title,
		statusLine,
		booking.Start.Format("02.01.2006"),
		html.EscapeString(weekdayRU(booking.Start.Weekday())+", "+booking.Start.Format("15:04")+"-"+booking.End.Format("15:04")),
		html.EscapeString(booking.Name),
		html.EscapeString(booking.Email),
		html.EscapeString(booking.Phone),
		timezone,
		comment,
		html.EscapeString(booking.ID),
	)
}

func (s *Service) replyMarkup(booking calendar.Booking) *inlineKeyboard {
	if booking.EffectiveStatus() != calendar.BookingStatusPending {
		return &inlineKeyboard{InlineKeyboard: [][]inlineButton{}}
	}

	return &inlineKeyboard{
		InlineKeyboard: [][]inlineButton{
			{
				{Text: "✅ Подтвердить", CallbackData: string(calendar.ReviewActionConfirm) + ":" + booking.ID},
				{Text: "❌ Отклонить", CallbackData: string(calendar.ReviewActionReject) + ":" + booking.ID},
			},
		},
	}
}

func (s *Service) getUpdates(ctx context.Context, offset int) ([]Update, error) {
	request := getUpdatesRequest{
		Offset:         offset,
		Timeout:        50,
		Limit:          20,
		AllowedUpdates: []string{"callback_query"},
	}

	var updates []Update
	if err := s.apiCall(ctx, "getUpdates", request, &updates); err != nil {
		return nil, err
	}

	return updates, nil
}

func (s *Service) deleteWebhook(ctx context.Context) error {
	request := deleteWebhookRequest{DropPendingUpdates: false}
	return s.apiCall(ctx, "deleteWebhook", request, nil)
}

func (s *Service) answerCallback(ctx context.Context, queryID, text string) error {
	return s.apiCall(ctx, "answerCallbackQuery", callbackAnswerRequest{
		CallbackQueryID: queryID,
		Text:            text,
	}, nil)
}

func (s *Service) apiCall(ctx context.Context, method string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint(method), bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram status %d", response.StatusCode)
	}

	if out == nil {
		var apiPayload apiResponse[json.RawMessage]
		if err := json.NewDecoder(response.Body).Decode(&apiPayload); err != nil {
			return err
		}
		if !apiPayload.OK {
			return errors.New(apiPayload.Description)
		}
		return nil
	}

	var apiPayload apiResponse[json.RawMessage]
	if err := json.NewDecoder(response.Body).Decode(&apiPayload); err != nil {
		return err
	}
	if !apiPayload.OK {
		return errors.New(apiPayload.Description)
	}

	if len(apiPayload.Result) == 0 || string(apiPayload.Result) == "null" {
		return nil
	}

	return json.Unmarshal(apiPayload.Result, out)
}

func (s *Service) endpoint(method string) string {
	return fmt.Sprintf("%s/bot%s/%s", apiBase, s.token, method)
}

func parseCallbackData(value string) (calendar.ReviewAction, string, error) {
	parts := strings.SplitN(strings.TrimSpace(value), ":", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "", "", fmt.Errorf("invalid callback data %q", value)
	}

	action := calendar.ReviewAction(parts[0])
	switch action {
	case calendar.ReviewActionConfirm, calendar.ReviewActionReject:
		return action, parts[1], nil
	default:
		return "", "", fmt.Errorf("unknown callback action %q", parts[0])
	}
}

func sleepWithContext(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
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
