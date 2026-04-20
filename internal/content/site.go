package content

func DefaultSite(contact Contact) Site {
	return Site{
		Brand:       "Психолог Наталья Кудинова",
		Description: "Индивидуальные онлайн-консультации психолога Натальи Кудиновой.",
		Contact:     contact,
		Nav: []NavItem{
			{Label: "Обо мне", Href: "/#about"},
			{Label: "Стоимость", Href: "/#pricing"},
			{Label: "Контакты", Href: "/#contacts"},
			{Label: "Правила", Href: "/rules"},
			{Label: "Памятка", Href: "/memo"},
			{Label: "Вопросы и ответы", Href: "/#faq"},
		},
		Home:    homePage(),
		Rules:   rulesPage(),
		Memo:    memoPage(),
		Privacy: privacyPage(),
		Booking: bookingPage(),
	}
}
