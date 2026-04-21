package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"psy/internal/content"
)

func (h *Handler) administratorContentSave(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) || !h.administratorRequireAuth(w, r) {
		return
	}
	if h.contentManager == nil {
		h.renderAdministratorPage(w, r, PageData{
			AdminTab:   "content",
			AdminError: "Хранилище контента не настроено.",
		}, http.StatusServiceUnavailable)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderAdministratorPage(w, r, PageData{
			AdminTab:   "content",
			AdminError: "Не удалось обработать форму контента.",
		}, http.StatusBadRequest)
		return
	}

	section := adminContentSection(r.FormValue("section"))
	site := h.contentManager.Draft()
	if err := applyContentSection(&site, section, r); err != nil {
		h.renderAdministratorPage(w, r, PageData{
			AdminTab:            "content",
			AdminContentSection: section,
			AdminError:          err.Error(),
			AdminContentForm:    adminContentFormFromSite(site),
		}, http.StatusBadRequest)
		return
	}

	if err := h.contentManager.SaveDraft(context.Background(), site); err != nil {
		h.renderAdministratorPage(w, r, PageData{
			AdminTab:            "content",
			AdminContentSection: section,
			AdminError:          "Не удалось сохранить черновик.",
			AdminContentForm:    adminContentFormFromSite(site),
		}, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/administrator?tab=content&section="+section+"&notice=draft-saved", http.StatusSeeOther)
}

func (h *Handler) administratorContentPublish(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) || !h.administratorRequireAuth(w, r) {
		return
	}
	if h.contentManager == nil {
		h.renderAdministratorPage(w, r, PageData{
			AdminTab:   "content",
			AdminError: "Хранилище контента не настроено.",
		}, http.StatusServiceUnavailable)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/administrator?tab=content", http.StatusSeeOther)
		return
	}

	published, err := h.contentManager.Publish(context.Background())
	if err != nil {
		h.renderAdministratorPage(w, r, PageData{
			AdminTab:            "content",
			AdminContentSection: adminContentSection(r.FormValue("section")),
			AdminError:          "Не удалось опубликовать изменения.",
		}, http.StatusInternalServerError)
		return
	}

	h.site = published
	section := adminContentSection(r.FormValue("section"))
	http.Redirect(w, r, "/administrator?tab=content&section="+section+"&notice=content-published", http.StatusSeeOther)
}

func (h *Handler) adminContentForm() AdminContentForm {
	return adminContentFormFromSite(h.draftSite())
}

func adminContentFormFromSite(site content.Site) AdminContentForm {
	return AdminContentForm{
		Brand:              site.Brand,
		Description:        site.Description,
		FontSans:           site.FontSans,
		ContactEmail:       site.Contact.Email,
		ContactPhone:       site.Contact.Phone,
		ContactLocation:    site.Contact.Location,
		TelegramURL:        site.Contact.TelegramURL,
		MaxURL:             site.Contact.MaxURL,
		CalendarURL:        site.Contact.CalendarURL,
		HomeHeroImageSrc:   site.Home.HeroImage.Src,
		HomeHeroImageAlt:   site.Home.HeroImage.Alt,
		HomeHeadline:       site.Home.Headline,
		HomeSubheadline:    site.Home.Subheadline,
		HomeSupportText:    site.Home.SupportText,
		AboutImageSrc:      site.Home.About.Image.Src,
		AboutImageAlt:      site.Home.About.Image.Alt,
		AboutLead:          joinLines(site.Home.About.Lead),
		AboutButtonText:    site.Home.About.ButtonText,
		Stats:              formatMetrics(site.Home.Stats),
		Values:             formatTitledTexts(site.Home.Values),
		Qualifications:     formatTitledLists(site.Home.Qualifications),
		Standards:          joinLines(site.Home.Standards),
		ReviewImageSrc:     site.Home.ReviewPhilosophy.Image.Src,
		ReviewImageAlt:     site.Home.ReviewPhilosophy.Image.Alt,
		ReviewTitle:        site.Home.ReviewPhilosophy.Title,
		ReviewParagraphs:   joinLines(site.Home.ReviewPhilosophy.Paragraphs),
		Pricing:            formatPricing(site.Home.Pricing),
		FAQ:                formatTitledTexts(site.Home.FAQ),
		BookingTitle:       site.Booking.Title,
		BookingImageSrc:    site.Booking.Image.Src,
		BookingImageAlt:    site.Booking.Image.Alt,
		BookingDescription: joinLines(site.Booking.Description),
		MemoTitle:          site.Memo.Title,
		MemoSubtitle:       site.Memo.Subtitle,
		MemoBlocks:         formatImageTexts(site.Memo.Blocks),
		RulesTitle:         site.Rules.Title,
		RulesSubtitle:      site.Rules.Subtitle,
		RulesLead:          joinLines(site.Rules.Lead),
		RulesBlocks:        formatTextBlocks(site.Rules.Blocks),
		PrivacyTitle:       site.Privacy.Title,
		PrivacySubtitle:    site.Privacy.Subtitle,
		PrivacyLead:        joinLines(site.Privacy.Lead),
		PrivacyBlocks:      formatTextBlocks(site.Privacy.Blocks),
	}
}

func applyContentSection(site *content.Site, section string, r *http.Request) error {
	switch section {
	case "main":
		site.Brand = strings.TrimSpace(r.FormValue("brand"))
		site.Description = strings.TrimSpace(r.FormValue("description"))
		site.FontSans = strings.TrimSpace(r.FormValue("font_sans"))
		site.Contact.Email = strings.TrimSpace(r.FormValue("contact_email"))
		site.Contact.Phone = strings.TrimSpace(r.FormValue("contact_phone"))
		site.Contact.Location = strings.TrimSpace(r.FormValue("contact_location"))
		site.Contact.TelegramURL = strings.TrimSpace(r.FormValue("telegram_url"))
		site.Contact.MaxURL = strings.TrimSpace(r.FormValue("max_url"))
		site.Contact.CalendarURL = strings.TrimSpace(r.FormValue("calendar_url"))
		if site.Brand == "" || site.Contact.Email == "" || site.Contact.Phone == "" {
			return fmt.Errorf("Заполните бренд, e-mail и телефон.")
		}
		return nil
	case "home":
		site.Home.Headline = strings.TrimSpace(r.FormValue("home_headline"))
		site.Home.Subheadline = strings.TrimSpace(r.FormValue("home_subheadline"))
		site.Home.SupportText = strings.TrimSpace(r.FormValue("home_support_text"))
		site.Home.About.Lead = splitTextareaLines(r.FormValue("about_lead"))
		site.Home.About.ButtonText = strings.TrimSpace(r.FormValue("about_button_text"))

		stats, err := parseMetrics(r.FormValue("stats"), site.Home.Stats)
		if err != nil {
			return err
		}
		values, err := parseTitledTexts(r.FormValue("values"))
		if err != nil {
			return err
		}
		qualifications, err := parseTitledLists(r.FormValue("qualifications"))
		if err != nil {
			return err
		}

		site.Home.Stats = stats
		site.Home.Values = values
		site.Home.Qualifications = qualifications
		site.Home.Standards = splitTextareaLines(r.FormValue("standards"))
		site.Home.ReviewPhilosophy = content.ImageText{
			Image:      site.Home.ReviewPhilosophy.Image,
			Title:      strings.TrimSpace(r.FormValue("review_title")),
			Paragraphs: splitTextareaLines(r.FormValue("review_paragraphs")),
		}
		return nil
	case "pricing":
		pricing, err := parsePricing(r.FormValue("pricing"), site.Home.Pricing)
		if err != nil {
			return err
		}
		faq, err := parseTitledTexts(r.FormValue("faq"))
		if err != nil {
			return err
		}
		site.Home.Pricing = pricing
		site.Home.FAQ = faq
		return nil
	case "booking":
		site.Booking.Title = strings.TrimSpace(r.FormValue("booking_title"))
		site.Booking.Description = splitTextareaLines(r.FormValue("booking_description"))
		return nil
	case "memo":
		blocks, err := parseImageTexts(r.FormValue("memo_blocks"), site.Memo.Blocks)
		if err != nil {
			return err
		}
		site.Memo.Title = strings.TrimSpace(r.FormValue("memo_title"))
		site.Memo.Subtitle = strings.TrimSpace(r.FormValue("memo_subtitle"))
		site.Memo.Blocks = blocks
		return nil
	case "rules":
		blocks, err := parseTextBlocks(r.FormValue("rules_blocks"))
		if err != nil {
			return err
		}
		site.Rules.Title = strings.TrimSpace(r.FormValue("rules_title"))
		site.Rules.Subtitle = strings.TrimSpace(r.FormValue("rules_subtitle"))
		site.Rules.Lead = splitTextareaLines(r.FormValue("rules_lead"))
		site.Rules.Blocks = blocks
		return nil
	case "privacy":
		blocks, err := parseTextBlocks(r.FormValue("privacy_blocks"))
		if err != nil {
			return err
		}
		site.Privacy.Title = strings.TrimSpace(r.FormValue("privacy_title"))
		site.Privacy.Subtitle = strings.TrimSpace(r.FormValue("privacy_subtitle"))
		site.Privacy.Lead = splitTextareaLines(r.FormValue("privacy_lead"))
		site.Privacy.Blocks = blocks
		return nil
	default:
		return fmt.Errorf("Неизвестный раздел контента.")
	}
}

func joinLines(values []string) string {
	return strings.Join(values, "\n")
}

func splitBlocks(value string) []string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	parts := strings.Split(value, "\n\n")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func formatTitledTexts(items []content.TitledText) string {
	blocks := make([]string, 0, len(items))
	for _, item := range items {
		blocks = append(blocks, strings.TrimSpace(item.Title+"\n"+item.Text))
	}
	return strings.Join(blocks, "\n\n")
}

func parseTitledTexts(value string) ([]content.TitledText, error) {
	blocks := splitBlocks(value)
	result := make([]content.TitledText, 0, len(blocks))
	for _, block := range blocks {
		lines := splitTextareaLines(block)
		if len(lines) < 2 {
			return nil, fmt.Errorf("Каждый блок должен содержать заголовок и текст.")
		}
		result = append(result, content.TitledText{
			Title: lines[0],
			Text:  strings.Join(lines[1:], " "),
		})
	}
	return result, nil
}

func formatTitledLists(items []content.TitledList) string {
	blocks := make([]string, 0, len(items))
	for _, item := range items {
		lines := append([]string{item.Title}, item.Items...)
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	return strings.Join(blocks, "\n\n")
}

func parseTitledLists(value string) ([]content.TitledList, error) {
	blocks := splitBlocks(value)
	result := make([]content.TitledList, 0, len(blocks))
	for _, block := range blocks {
		lines := splitTextareaLines(block)
		if len(lines) < 2 {
			return nil, fmt.Errorf("Каждый блок образования должен содержать заголовок и минимум один пункт.")
		}
		result = append(result, content.TitledList{
			Title: lines[0],
			Items: lines[1:],
		})
	}
	return result, nil
}

func formatMetrics(items []content.Metric) string {
	blocks := make([]string, 0, len(items))
	for _, item := range items {
		lines := []string{item.Prefix, item.Value, item.Label}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	return strings.Join(blocks, "\n\n")
}

func parseMetrics(value string, existing []content.Metric) ([]content.Metric, error) {
	blocks := splitBlocks(value)
	if len(blocks) != len(existing) {
		return nil, fmt.Errorf("Количество метрик должно оставаться таким же, чтобы сохранить текущие иконки.")
	}
	result := make([]content.Metric, 0, len(blocks))
	for index, block := range blocks {
		lines := splitTextareaLines(block)
		if len(lines) < 3 {
			return nil, fmt.Errorf("Каждая метрика должна содержать вступление, значение и подпись.")
		}
		result = append(result, content.Metric{
			Prefix: lines[0],
			Value:  lines[1],
			Label:  strings.Join(lines[2:], " "),
			Icon:   existing[index].Icon,
		})
	}
	return result, nil
}

func formatPricing(items []content.PriceCard) string {
	blocks := make([]string, 0, len(items))
	for _, item := range items {
		lines := []string{item.Title, item.Price, item.Text}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	return strings.Join(blocks, "\n\n")
}

func parsePricing(value string, existing []content.PriceCard) ([]content.PriceCard, error) {
	blocks := splitBlocks(value)
	if len(blocks) != len(existing) {
		return nil, fmt.Errorf("Количество карточек с ценами должно оставаться таким же.")
	}
	result := make([]content.PriceCard, 0, len(blocks))
	for index, block := range blocks {
		lines := splitTextareaLines(block)
		if len(lines) < 3 {
			return nil, fmt.Errorf("Каждая карточка с ценой должна содержать название, сумму и описание.")
		}
		result = append(result, content.PriceCard{
			Title:       lines[0],
			Price:       lines[1],
			DynamicNote: existing[index].DynamicNote,
			Text:        strings.Join(lines[2:], " "),
		})
	}
	return result, nil
}

func formatTextBlocks(items []content.TextBlock) string {
	blocks := make([]string, 0, len(items))
	for _, item := range items {
		lines := append([]string{item.Title}, item.Paragraphs...)
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	return strings.Join(blocks, "\n\n")
}

func parseTextBlocks(value string) ([]content.TextBlock, error) {
	blocks := splitBlocks(value)
	result := make([]content.TextBlock, 0, len(blocks))
	for _, block := range blocks {
		lines := splitTextareaLines(block)
		if len(lines) < 2 {
			return nil, fmt.Errorf("Каждый текстовый блок должен содержать заголовок и минимум один абзац.")
		}
		result = append(result, content.TextBlock{
			Title:      lines[0],
			Paragraphs: lines[1:],
		})
	}
	return result, nil
}

func formatImageTexts(items []content.ImageText) string {
	blocks := make([]string, 0, len(items))
	for _, item := range items {
		lines := []string{item.Title}
		lines = append(lines, item.Paragraphs...)
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	return strings.Join(blocks, "\n\n")
}

func parseImageTexts(value string, existing []content.ImageText) ([]content.ImageText, error) {
	blocks := splitBlocks(value)
	if len(blocks) != len(existing) {
		return nil, fmt.Errorf("Количество текстовых блоков памятки должно оставаться таким же, чтобы сохранить текущие изображения.")
	}
	result := make([]content.ImageText, 0, len(blocks))
	for index, block := range blocks {
		lines := splitTextareaLines(block)
		if len(lines) < 2 {
			return nil, fmt.Errorf("Каждый блок памятки должен содержать заголовок и минимум один абзац.")
		}
		result = append(result, content.ImageText{
			Title:      lines[0],
			Image:      existing[index].Image,
			Paragraphs: lines[1:],
		})
	}
	return result, nil
}
