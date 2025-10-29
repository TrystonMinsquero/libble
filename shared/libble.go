package shared

type UserData struct {
	LID        int64   `json:"libble_id"`
	UserId     string  `json:"user_gr_id"`
	SeenQuotes []int64 `json:"seen_quote_ids"`
	Games      []int64 `json:"game_ids"`
}

type UserBook struct {
	LID int64 `json:"libble_id"`
}

type Book struct {
	LID         int64    `json:"libble_id"`
	BookId      string   `json:"book_gr_id"`
	Title       string   `json:"title"`
	Author      string   `json:"author"`
	AuthorId    string   `json:"author_gr_id"`
	Stars       uint     `json:"stars"`
	AvgRating   float32  `json:"avg_rating"`
	RatingCount uint     `json:"rating_count"`
	DatesRead   []string `json:"dates_read"`
	DateAdded   string   `json:"date_added"`
}

type Quote struct {
	LID     int64  `json:"libble_id"`
	QuoteId string `json:"quote_gr_id"`
	Likes   uint   `json:"likes"`
	Text    string `json:"text"`

	BookId   string `json:"book_gr_id"`
	AuthorId string `json:"author_gr_id"`
}

func (b Book) ShouldScrape() bool {
	return b.Stars >= 4
}
func (b Book) IsRead() bool {
	return b.Stars > 0
}
