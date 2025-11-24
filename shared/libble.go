package shared

import (
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"time"
)

type DBID uint64

type QuoteId DBID
type BookId DBID

const NilID = 0

type SaveData struct {
	User   UserData            `json:"user_data"`
	Books  map[BookId]UserBook `json:"books"`
	Quotes map[QuoteId]Quote   `json:"quotes"`

	SeenQuotes []QuoteId `json:"seen_quote_ids"`
	Games      []Game    `json:"game"`
}

type UserData struct {
	ID       DBID   `json:"libble_id"`
	UserGRID string `json:"user_gr_id"`
}

type UserBook struct {
	Book     Book         `json:"book"`
	UserData UserBookData `json:"user_book_data"`
}

type UserBookData struct {
	Stars     uint     `json:"stars"`
	DatesRead []string `json:"dates_read"`
	DateAdded string   `json:"date_added"`
}

type Game struct {
	QuoteID   QuoteId   `json:"quote_id"`
	Date      time.Time `json:"date_started"`
	Completed bool      `json:"completed"`
	Guesses   []BookId  `json:"guesses"`
}

type Book struct {
	BookGRID    string  `json:"book_gr_id"`
	Title       string  `json:"title"`
	Author      string  `json:"author"`
	AuthorGRID  string  `json:"author_gr_id"`
	AvgRating   float32 `json:"avg_rating"`
	RatingCount uint    `json:"rating_count"`
}

func (b Book) CleanTitle() string {
	return strings.TrimSpace(strings.Join(strings.Fields(b.Title), " "))
}

type Quote struct {
	QuoteGRID string `json:"quote_gr_id"`
	Likes     uint   `json:"likes"`
	Text      string `json:"text"`

	BookId   BookId `json:"book_id"`
	BookGRID string `json:"book_gr_id"`
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

type weightedQuote struct {
	quote QuoteId
	tries uint8
}

func (s SaveData) PickDailyQuote() (QuoteId, error) {
	var quoteId QuoteId
	quoteCount := len(s.Quotes)
	if quoteCount <= 0 {
		return quoteId, fmt.Errorf("User has no quotes")
	}

	now := time.Now().UTC()
	seed := now.Year() + now.YearDay()
	rng := rand.New(rand.NewSource(int64(seed)))

	quotes := make([]weightedQuote, quoteCount)
	triedCount := 0
	collisions := 0

	quoteIndex := 0
	for id, _ := range s.Quotes {
		quotes[quoteIndex].quote = id
		quoteIndex++
	}

	for triedCount < quoteCount && collisions < quoteCount*2 {
		quoteIndex := rng.Intn(quoteCount)

		quoteId := quotes[quoteIndex].quote
		tries := quotes[quoteIndex].tries
		if tries > 0 {
			collisions += 1
			if tries == 100 {
				panic("Too many tries")
			}
			quotes[quoteIndex].tries += 1
			continue
		}

		triedCount += 1
		if slices.Contains(s.SeenQuotes, quoteId) {
			continue
		}

		// Check if book is read
		quote, found := s.Quotes[quoteId]
		if !found {
			panic("Couldn't get quote back")
		}
		book, found := s.Books[quote.BookId]
		if !found {
			fmt.Printf("Couldn't find book with id %d for quote %d\n", quote.BookId, quoteId)
			continue
		}
		if !book.UserData.IsRead() {
			continue
		}
		return quoteId, nil
	}

	fmt.Printf("Warning: Recycling quote for %s\n", s.User.UserGRID)
	quoteIndex = rng.Intn(quoteCount)
	return quotes[quoteIndex].quote, nil
}

// Returns index from `availableQuotes`
// func PickDailyQuote(user UserData, books []UserBook, availableQuotes []Quote) int {
//
// 	quoteCount := len(availableQuotes)
// 	if quoteCount <= 0 {
// 		return -1
// 	}
//
// 	now := time.Now().UTC()
// 	seed := now.Year() + now.YearDay()
// 	rng := rand.New(rand.NewSource(int64(seed)))
//
// 	triedIndexes := make([]bool, quoteCount)
// 	triedIndexCount := 0
// 	collisions := 0
//
// 	for triedIndexCount < quoteCount && collisions < quoteCount*2 {
// 		quoteIndex := rng.Intn(quoteCount)
// 		if triedIndexes[quoteIndex] {
// 			collisions += 1
// 			continue
// 		}
//
// 		triedIndexes[quoteIndex] = true
// 		triedIndexCount += 1
//
// 		quote := availableQuotes[quoteIndex]
// 		if slices.Contains(user.SeenQuotes, quote.ID) {
// 			continue
// 		}
// 		return quoteIndex
// 	}
//
// 	fmt.Printf("Warning: Recycling quote for %s\n", user.UserGRID)
// 	quoteIndex := rng.Intn(quoteCount)
// 	return quoteIndex
// }
