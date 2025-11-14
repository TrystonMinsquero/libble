package shared

import (
	"fmt"
	"math/rand"
	"slices"
	"time"
	// "slices"
)

type DBID int64

const NilID = 0

type SaveData struct {
	User   UserData   `json:"user_data"`
	Books  []UserBook `json:"books"`
	Quotes []Quote    `json:"quotes"`
}

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
	// Would have one or the other
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

	quoteCount := len(availableQuotes)
	if quoteCount <= 0 {
		return -1
	}

	now := time.Now().UTC()
	seed := now.Year() + now.YearDay()
	rng := rand.New(rand.NewSource(int64(seed)))

	triedIndexes := make([]bool, quoteCount)
	triedIndexCount := 0
	collisions := 0

	for triedIndexCount < quoteCount && collisions < quoteCount*2 {
		quoteIndex := rng.Intn(quoteCount)
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

	fmt.Printf("Warning: Recycling quote for %s\n", user.UserGRID)
	quoteIndex := rng.Intn(quoteCount)
	return quoteIndex
}
