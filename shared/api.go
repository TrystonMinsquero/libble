package shared

type UserCreateRequest struct {
	GRID          string        `json:"grid"`
	ScrapeOptions ScrapeOptions `json:"scrape_options"`
}
type UserCreateResponse struct {
	UserID UserID `json:"user_id"`
	Error  string `json:"error,omitempty"`
}

type UserCreateStatus struct {
	BooksFound      uint    `json:"books_found,omitempty"`
	BooksCollected  uint    `json:"books_collected,omitempty"`
	QuotesCollected uint    `json:"quotes_collected,omitempty"`
	InitialQuote    QuoteID `json:"inital_quote,omitempty"`
	Finished        bool    `json:"finished,omitempty"`
	Error           string  `json:"error,omitempty"`
}
