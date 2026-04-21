package mailer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"psy/internal/calendar"
)

const resendEndpoint = "https://api.resend.com/emails"

type Service struct {
	apiKey       string
	from         string
	replyTo      string
	baseLocation *time.Location
	client       *http.Client
	endpoint     string
	logger       *slog.Logger
}

type sendEmailRequest struct {
	From    string     `json:"from"`
	To      []string   `json:"to"`
	Subject string     `json:"subject"`
	HTML    string     `json:"html,omitempty"`
	Text    string     `json:"text,omitempty"`
	ReplyTo []string   `json:"reply_to,omitempty"`
	Tags    []emailTag `json:"tags,omitempty"`
}

type emailTag struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type sendEmailResponse struct {
	ID string `json:"id"`
}

type slotPresentation struct {
	Date     string
	Weekday  string
	Time     string
	Timezone string
}

func NewResend(apiKey, from, replyTo, baseTimezone string, logger *slog.Logger) *Service {
	location, err := time.LoadLocation(baseTimezone)
	if err != nil {
		location = time.UTC
	}

	return &Service{
		apiKey:       strings.TrimSpace(apiKey),
		from:         strings.TrimSpace(from),
		replyTo:      strings.TrimSpace(replyTo),
		baseLocation: location,
		client:       &http.Client{Timeout: 15 * time.Second},
		endpoint:     resendEndpoint,
		logger:       logger,
	}
}

func (s *Service) Enabled() bool {
	return s != nil && s.apiKey != "" && s.from != ""
}

func (s *Service) SendBookingConfirmation(ctx context.Context, booking calendar.Booking) error {
	if !s.Enabled() {
		return nil
	}

	recipient := strings.TrimSpace(booking.Email)
	if recipient == "" {
		return fmt.Errorf("booking %s has empty email", booking.ID)
	}

	clientSlot, moscowSlot := s.presentations(booking)
	htmlBody, textBody := s.renderConfirmation(booking, clientSlot, moscowSlot)

	payload := sendEmailRequest{
		From:    s.from,
		To:      []string{recipient},
		Subject: fmt.Sprintf("Запись подтверждена — %s %s", clientSlot.Date, clientSlot.Time),
		HTML:    htmlBody,
		Text:    textBody,
		Tags: []emailTag{
			{Name: "type", Value: "booking_confirmation"},
			{Name: "booking_id", Value: sanitizeTagValue(booking.ID)},
		},
	}

	if s.replyTo != "" {
		payload.ReplyTo = []string{s.replyTo}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+s.apiKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "booking-confirmation-"+booking.ID)

	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 32*1024))
	if err != nil {
		return err
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("resend status %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var sent sendEmailResponse
	if len(responseBody) > 0 {
		if err := json.Unmarshal(responseBody, &sent); err != nil {
			return err
		}
	}

	if s.logger != nil {
		s.logger.Info("booking confirmation email sent", "booking_id", booking.ID, "email_id", sent.ID, "to", recipient)
	}

	return nil
}

func (s *Service) SendBookingCancellation(ctx context.Context, booking calendar.Booking) error {
	if !s.Enabled() {
		return nil
	}

	clientSlot, moscowSlot := s.presentations(booking)
	subject := fmt.Sprintf("Ваша запись отменена — %s %s", clientSlot.Date, clientSlot.Time)
	htmlBody, textBody := s.renderCancellation(booking, clientSlot, moscowSlot)
	return s.sendBookingEmail(ctx, booking, subject, "booking_cancellation", htmlBody, textBody)
}

func (s *Service) SendBookingRescheduled(ctx context.Context, booking calendar.Booking) error {
	if !s.Enabled() {
		return nil
	}

	clientSlot, moscowSlot := s.presentations(booking)
	subject := fmt.Sprintf("Ваша запись перенесена — %s %s", clientSlot.Date, clientSlot.Time)
	htmlBody, textBody := s.renderRescheduled(booking, clientSlot, moscowSlot)
	return s.sendBookingEmail(ctx, booking, subject, "booking_rescheduled", htmlBody, textBody)
}

func (s *Service) presentations(booking calendar.Booking) (slotPresentation, *slotPresentation) {
	clientSlot, ok := s.slotForTimezone(booking, booking.ClientTimezone)
	if !ok {
		clientSlot = s.mustSlotForTimezone(booking, s.baseLocation)
		return clientSlot, nil
	}

	baseSlot := s.mustSlotForTimezone(booking, s.baseLocation)
	if clientSlot.Timezone == baseSlot.Timezone {
		return clientSlot, nil
	}

	return clientSlot, &baseSlot
}

func (s *Service) slotForTimezone(booking calendar.Booking, timezone string) (slotPresentation, bool) {
	timezone = strings.TrimSpace(timezone)
	if timezone == "" {
		return slotPresentation{}, false
	}

	location, err := time.LoadLocation(timezone)
	if err != nil {
		return slotPresentation{}, false
	}

	return s.mustSlotForTimezone(booking, location), true
}

func (s *Service) mustSlotForTimezone(booking calendar.Booking, location *time.Location) slotPresentation {
	start := booking.Start.In(location)
	end := booking.End.In(location)

	return slotPresentation{
		Date:     start.Format("02.01.2006"),
		Weekday:  weekdayRU(start.Weekday()),
		Time:     start.Format("15:04") + "-" + end.Format("15:04"),
		Timezone: location.String(),
	}
}

func (s *Service) renderConfirmation(booking calendar.Booking, clientSlot slotPresentation, moscowSlot *slotPresentation) (string, string) {
	name := strings.TrimSpace(booking.Name)
	if name == "" {
		name = "Здравствуйте"
	} else {
		name = "Здравствуйте, " + name
	}

	var htmlBuilder strings.Builder
	htmlBuilder.WriteString("<div style=\"font-family:Helvetica,Arial,sans-serif;font-size:16px;line-height:1.6;color:#111\">")
	htmlBuilder.WriteString("<p>" + html.EscapeString(name) + ".</p>")
	htmlBuilder.WriteString("<p>Ваша запись на онлайн-консультацию подтверждена.</p>")
	htmlBuilder.WriteString("<p><strong>Дата и время:</strong> " + html.EscapeString(clientSlot.Date) + ", " + html.EscapeString(clientSlot.Weekday) + ", " + html.EscapeString(clientSlot.Time) + " (" + html.EscapeString(clientSlot.Timezone) + ")</p>")
	if moscowSlot != nil {
		htmlBuilder.WriteString("<p><strong>По Москве:</strong> " + html.EscapeString(moscowSlot.Date) + ", " + html.EscapeString(moscowSlot.Weekday) + ", " + html.EscapeString(moscowSlot.Time) + " (" + html.EscapeString(moscowSlot.Timezone) + ")</p>")
	}
	htmlBuilder.WriteString("<p><strong>Формат:</strong> онлайн.</p>")
	if s.replyTo != "" {
		htmlBuilder.WriteString("<p>Если понадобится перенести или отменить встречу, просто ответьте на это письмо.</p>")
	}
	htmlBuilder.WriteString("<p>До встречи,<br>Наталья Кудинова</p>")
	htmlBuilder.WriteString("</div>")

	var textBuilder strings.Builder
	textBuilder.WriteString(name + ".\n\n")
	textBuilder.WriteString("Ваша запись на онлайн-консультацию подтверждена.\n\n")
	textBuilder.WriteString("Дата и время: " + clientSlot.Date + ", " + clientSlot.Weekday + ", " + clientSlot.Time + " (" + clientSlot.Timezone + ")\n")
	if moscowSlot != nil {
		textBuilder.WriteString("По Москве: " + moscowSlot.Date + ", " + moscowSlot.Weekday + ", " + moscowSlot.Time + " (" + moscowSlot.Timezone + ")\n")
	}
	textBuilder.WriteString("Формат: онлайн.\n")
	if s.replyTo != "" {
		textBuilder.WriteString("Если понадобится перенести или отменить встречу, просто ответьте на это письмо.\n")
	}
	textBuilder.WriteString("\nДо встречи,\nНаталья Кудинова\n")

	return htmlBuilder.String(), textBuilder.String()
}

func (s *Service) renderCancellation(booking calendar.Booking, clientSlot slotPresentation, moscowSlot *slotPresentation) (string, string) {
	name := strings.TrimSpace(booking.Name)
	if name == "" {
		name = "Здравствуйте"
	} else {
		name = "Здравствуйте, " + name
	}

	var htmlBuilder strings.Builder
	htmlBuilder.WriteString("<div style=\"font-family:Helvetica,Arial,sans-serif;font-size:16px;line-height:1.6;color:#111\">")
	htmlBuilder.WriteString("<p>" + html.EscapeString(name) + ".</p>")
	htmlBuilder.WriteString("<p>Ваша запись отменена.</p>")
	htmlBuilder.WriteString("<p><strong>Изначальный слот:</strong> " + html.EscapeString(clientSlot.Date) + ", " + html.EscapeString(clientSlot.Weekday) + ", " + html.EscapeString(clientSlot.Time) + " (" + html.EscapeString(clientSlot.Timezone) + ")</p>")
	if moscowSlot != nil {
		htmlBuilder.WriteString("<p><strong>По Москве:</strong> " + html.EscapeString(moscowSlot.Date) + ", " + html.EscapeString(moscowSlot.Weekday) + ", " + html.EscapeString(moscowSlot.Time) + " (" + html.EscapeString(moscowSlot.Timezone) + ")</p>")
	}
	if s.replyTo != "" {
		htmlBuilder.WriteString("<p>Если хотите подобрать новое время, просто ответьте на это письмо.</p>")
	}
	htmlBuilder.WriteString("<p>Наталья Кудинова</p>")
	htmlBuilder.WriteString("</div>")

	var textBuilder strings.Builder
	textBuilder.WriteString(name + ".\n\n")
	textBuilder.WriteString("Ваша запись отменена.\n\n")
	textBuilder.WriteString("Изначальный слот: " + clientSlot.Date + ", " + clientSlot.Weekday + ", " + clientSlot.Time + " (" + clientSlot.Timezone + ")\n")
	if moscowSlot != nil {
		textBuilder.WriteString("По Москве: " + moscowSlot.Date + ", " + moscowSlot.Weekday + ", " + moscowSlot.Time + " (" + moscowSlot.Timezone + ")\n")
	}
	if s.replyTo != "" {
		textBuilder.WriteString("Если хотите подобрать новое время, просто ответьте на это письмо.\n")
	}
	textBuilder.WriteString("\nНаталья Кудинова\n")

	return htmlBuilder.String(), textBuilder.String()
}

func (s *Service) renderRescheduled(booking calendar.Booking, clientSlot slotPresentation, moscowSlot *slotPresentation) (string, string) {
	name := strings.TrimSpace(booking.Name)
	if name == "" {
		name = "Здравствуйте"
	} else {
		name = "Здравствуйте, " + name
	}

	var htmlBuilder strings.Builder
	htmlBuilder.WriteString("<div style=\"font-family:Helvetica,Arial,sans-serif;font-size:16px;line-height:1.6;color:#111\">")
	htmlBuilder.WriteString("<p>" + html.EscapeString(name) + ".</p>")
	htmlBuilder.WriteString("<p>Ваша запись перенесена на " + html.EscapeString(clientSlot.Date) + ".</p>")
	htmlBuilder.WriteString("<p><strong>Новое время:</strong> " + html.EscapeString(clientSlot.Date) + ", " + html.EscapeString(clientSlot.Weekday) + ", " + html.EscapeString(clientSlot.Time) + " (" + html.EscapeString(clientSlot.Timezone) + ")</p>")
	if moscowSlot != nil {
		htmlBuilder.WriteString("<p><strong>По Москве:</strong> " + html.EscapeString(moscowSlot.Date) + ", " + html.EscapeString(moscowSlot.Weekday) + ", " + html.EscapeString(moscowSlot.Time) + " (" + html.EscapeString(moscowSlot.Timezone) + ")</p>")
	}
	if s.replyTo != "" {
		htmlBuilder.WriteString("<p>Если новое время не подходит, просто ответьте на это письмо.</p>")
	}
	htmlBuilder.WriteString("<p>Наталья Кудинова</p>")
	htmlBuilder.WriteString("</div>")

	var textBuilder strings.Builder
	textBuilder.WriteString(name + ".\n\n")
	textBuilder.WriteString("Ваша запись перенесена на " + clientSlot.Date + ".\n\n")
	textBuilder.WriteString("Новое время: " + clientSlot.Date + ", " + clientSlot.Weekday + ", " + clientSlot.Time + " (" + clientSlot.Timezone + ")\n")
	if moscowSlot != nil {
		textBuilder.WriteString("По Москве: " + moscowSlot.Date + ", " + moscowSlot.Weekday + ", " + moscowSlot.Time + " (" + moscowSlot.Timezone + ")\n")
	}
	if s.replyTo != "" {
		textBuilder.WriteString("Если новое время не подходит, просто ответьте на это письмо.\n")
	}
	textBuilder.WriteString("\nНаталья Кудинова\n")

	return htmlBuilder.String(), textBuilder.String()
}

func (s *Service) sendBookingEmail(ctx context.Context, booking calendar.Booking, subject, tagType, htmlBody, textBody string) error {
	recipient := strings.TrimSpace(booking.Email)
	if recipient == "" {
		return fmt.Errorf("booking %s has empty email", booking.ID)
	}

	payload := sendEmailRequest{
		From:    s.from,
		To:      []string{recipient},
		Subject: subject,
		HTML:    htmlBody,
		Text:    textBody,
		Tags: []emailTag{
			{Name: "type", Value: tagType},
			{Name: "booking_id", Value: sanitizeTagValue(booking.ID)},
		},
	}
	if s.replyTo != "" {
		payload.ReplyTo = []string{s.replyTo}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+s.apiKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", tagType+"-"+booking.ID+"-"+sanitizeTagValue(subject))

	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 32*1024))
	if err != nil {
		return err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("resend status %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	return nil
}

func sanitizeTagValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}

	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '_' || r == '-':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
		if builder.Len() >= 256 {
			break
		}
	}

	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "unknown"
	}
	return result
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
