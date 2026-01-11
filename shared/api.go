package shared

type UserCreateRequest struct {
	GRID          string        `json:"grid"`
	ScrapeOptions ScrapeOptions `json:"scrape_options"`
}
type UserCreateResponse struct {
	Player Player `json:"player"`
	Error  string `json:"error,omitempty"`
}
