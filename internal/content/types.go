package content

type Contact struct {
	Email       string
	Phone       string
	Location    string
	TelegramURL string
	MaxURL      string
	CalendarURL string
}

type Site struct {
	Brand       string
	Description string
	Nav         []NavItem
	Contact     Contact
	Home        HomePage
	Rules       TextPage
	Memo        MemoPage
	Privacy     TextPage
	Booking     BookingPage
}

type NavItem struct {
	Label string
	Href  string
}

type Image struct {
	Src string
	Alt string
}

type HomePage struct {
	HeroImage        Image
	Headline         string
	Subheadline      string
	SupportText      string
	About            AboutBlock
	Stats            []Metric
	Values           []TitledText
	Qualifications   []TitledList
	Standards        []string
	ReviewPhilosophy ImageText
	Pricing          []PriceCard
	FAQ              []TitledText
}

type AboutBlock struct {
	Image      Image
	Lead       []string
	ButtonText string
}

type Metric struct {
	Prefix string
	Value  string
	Label  string
	Icon   string
}

type TitledText struct {
	Title string
	Text  string
}

type TitledList struct {
	Title string
	Items []string
}

type ImageText struct {
	Image      Image
	Title      string
	Paragraphs []string
}

type PriceCard struct {
	Title       string
	Price       string
	DynamicNote bool
	Text        string
}

type TextPage struct {
	Title    string
	Subtitle string
	Lead     []string
	Blocks   []TextBlock
}

type TextBlock struct {
	Title      string
	Paragraphs []string
}

type MemoPage struct {
	Title    string
	Subtitle string
	Blocks   []ImageText
}

type BookingPage struct {
	Title       string
	Description []string
	Image       Image
}
