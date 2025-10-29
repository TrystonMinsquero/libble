package shared

import (
	"fmt"
	"math/rand"
	"slices"
	"time"
	// "slices"
)

type DBID int64

type UserData struct {
	ID         DBID   `json:"libble_id"`
	UserGRID   string `json:"user_gr_id"`
	SeenQuotes []DBID `json:"seen_quote_ids"`
	Games      []DBID `json:"game_ids"`
}

type UserBook struct {
	Book     Book         `json:"book"`
	UserData UserBookData `json:"user_book_data"`
}

type UserBookData struct {
	ID     DBID `json:"libble_id"`
	UserID DBID `json:"user_id"`
	BookID DBID `json:"book_id"`

	UserGRID string `json:"user_gr_id"`
	BookGRID string `json:"book_gr_id"`

	Stars     uint     `json:"stars"`
	DatesRead []string `json:"dates_read"`
	DateAdded string   `json:"date_added"`
}

type Game struct {
	ID      DBID   `json:"libble_id"`
	QuoteID DBID   `json:"quote_id"`
	Guesses []DBID `json:"guesses"` // book IDs
}

type Book struct {
	ID          DBID    `json:"libble_id"`
	BookGRID    string  `json:"book_gr_id"`
	Title       string  `json:"title"`
	Author      string  `json:"author"`
	AuthorGRID  string  `json:"author_gr_id"`
	AvgRating   float32 `json:"avg_rating"`
	RatingCount uint    `json:"rating_count"`
}

type Quote struct {
	ID        DBID   `json:"libble_id"`
	QuoteGRID string `json:"quote_gr_id"`
	Likes     uint   `json:"likes"`
	Text      string `json:"text"`

	BookGRID   string `json:"book_gr_id"`
	AuthorGRID string `json:"author_gr_id"`
}

func (b UserBookData) ShouldScrape() bool {
	return b.IsRead()
}

func (b UserBookData) IsRead() bool {
	if b.Stars > 0 {
		return true
	}
	for _, date := range b.DatesRead {
		if date != "not set" {
			return true
		}
	}
	return false
}

type DailyData struct {
	User       UserData   `json:"user"`
	Books      []UserBook `json:"books"`
	Quotes     []Quote    `json:"quotes"`
	DailyQuote int        `json:"daily_quote_index"`
}

// Returns index from `availableQuotes`
func PickDailyQuote(user UserData, books []UserBook, availableQuotes []Quote) int {
	now := time.Now()
	seed := now.Year() + now.YearDay()
	rng := rand.New(rand.NewSource(int64(seed)))

	triedIndexes := make([]bool, len(availableQuotes))
	triedIndexCount := 0
	collisions := 0

	for triedIndexCount < len(availableQuotes) && collisions < len(availableQuotes)*2 {
		quoteIndex := rng.Intn(len(availableQuotes))
		if triedIndexes[quoteIndex] {
			collisions += 1
			continue
		}

		triedIndexes[quoteIndex] = true
		triedIndexCount += 1

		quote := availableQuotes[quoteIndex]
		if slices.Contains(user.SeenQuotes, quote.ID) {
			continue
		}
		return quoteIndex
	}

	fmt.Println("Warning: Recycling quote")
	quoteIndex := rng.Intn(len(availableQuotes))
	return quoteIndex
}
