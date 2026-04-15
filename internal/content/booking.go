package content

func bookingPage() BookingPage {
	return BookingPage{
		Title: "Запись на консультацию",
		Image: Image{Src: "/static/img/portrait.jpg", Alt: "Наталья Кудинова"},
		Description: []string{
			"Онлайн-консультации для взрослых и подростков от 16 лет.",
			"Можно прийти с тревогой, сложностями в отношениях, проживанием утраты, адаптацией к новым условиям, вопросами самоценности и другими личными запросами.",
		},
	}
}
