package shared

type UserCreateRequest struct {
	GRID          string        `json:"grid"`
	ScrapeOptions ScrapeOptions `json:"scrape_options"`
}
type UserCreateResponse struct {
	UserID UserID `json:"user_id,omitempty"`
	Error  string `json:"error,omitempty"`
}

type UserCreateState string

const (
	UserCreateState_InProgress  UserCreateState = "InProgress"
	UserCreateState_NeedsReboot UserCreateState = "NeedsReboot"
	UserCreateState_Finished    UserCreateState = "Finished"
)

type UserCreateStatus struct {
	BooksFound      uint            `json:"books_found,omitempty"`
	BooksCollected  uint            `json:"books_collected,omitempty"` // collected in terms of scraped quotes
	QuotesCollected uint            `json:"quotes_collected,omitempty"`
	InitialQuote    QuoteID         `json:"inital_quote,omitempty"`
	State           UserCreateState `json:"state,omitempty"`
	Error           string          `json:"error,omitempty"`
}
