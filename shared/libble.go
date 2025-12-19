package shared

import (
	"errors"
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

type Player struct {
	ID       DBID           `json:"libble_id"`
	UserGRID string         `json:"user_gr_id"`
	Settings PlayerSettings `json:"settings"`

	SeenQuotes []QuoteId `json:"seen_quote_ids"`
	Games      []Game    `json:"games"`
}

type SaveData struct {
	Player Player `json:"player"`

	Books  map[BookId]UserBook `json:"books"`
	Quotes map[QuoteId]Quote   `json:"quotes"`
}

func (s SaveData) FindBookId(query string) BookId {
	query = strings.ToLower(strings.TrimSpace(query))
	for bookId, book := range s.Books {
		target := book.Book.CleanTitle()
		target = strings.ToLower(strings.TrimSpace(target))
		if target == query {
			return bookId
		}
	}
	return NilID
}

func IsStaticSaveDataField(jsonFieldName string) bool {
	switch jsonFieldName {
	case "books":
		return true
	case "quotes":
		return true
	}
	return false
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

type PlayerSettings struct {
	GameSettings  GameSettings  `json:"game_settings"`
	ScrapeOptions ScrapeOptions `json:"scrape_options"`
}

func DefaultPlayerSettings() PlayerSettings {
	return PlayerSettings{
		GameSettings:  DefaultGameSettings(),
		ScrapeOptions: DefaultScrapeOptions(),
	}
}

type ScrapeOptions struct {
	MinPersonalStars uint
	MinQuoteLikes    uint
	MaxQuoteForBook  uint
}

func DefaultScrapeOptions() ScrapeOptions {
	return ScrapeOptions{
		MinPersonalStars: 3,
		MinQuoteLikes:    10,
		MaxQuoteForBook:  150,
	}
}

func (o ScrapeOptions) ShouldScrapeQuotes(userBook UserBook) bool {
	if userBook.UserData.Stars >= o.MinPersonalStars {
		return false
	}
	return true
}

type GameSettings struct {
	MaxGuesses int  `json:"max_guesses"`
	AllowHints bool `json:"allowed_hints"`
}

func DefaultGameSettings() GameSettings {
	return GameSettings{
		MaxGuesses: 5,
		AllowHints: true,
	}
}

type Game struct {
	QuoteID QuoteId   `json:"quote_id"`
	Date    time.Time `json:"date_started"`
	Guesses []BookId  `json:"guesses"`

	// Runtime data
	Quote    Quote        `json:"-"`
	BookId   BookId       `json:"-"`
	Book     UserBook     `json:"-"`
	Settings GameSettings `json:"-"`
}

func (g *Game) Init(data SaveData) error {
	g.Settings = data.Player.Settings.GameSettings
	if g.Settings.MaxGuesses < 1 {
		// This should never happen, but just in case they can at least play
		g.Settings.MaxGuesses = 1
	}

	// Get the quote from the map
	quote, found := data.Quotes[g.QuoteID]
	if !found {
		return fmt.Errorf("Daily Quote not found in quotes map")
	}
	g.Quote = quote
	g.BookId = quote.BookId

	// Get the book from the map
	book, found := data.Books[quote.BookId]
	if !found {
		return fmt.Errorf("Daily Quote's book Id was not found in books map")
	}
	g.Book = book
	return nil
}

func (g Game) Started() bool {
	return g.Attempts() > 0 // NOTE: add hints here later
}

func (g Game) Attempts() int {
	return len(g.Guesses)
}
func (g Game) AttemptsLeft() int {
	return max(g.Settings.MaxGuesses-len(g.Guesses), 0)
}
func (g Game) Completed() bool {
	return g.AttemptsLeft() <= 0 || g.Won()
}
func (g Game) Won() bool {
	if len(g.Guesses) <= 0 {
		return false
	}
	return g.Guesses[len(g.Guesses)-1] == g.BookId
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

func (s SaveData) PickDailyQuote() (quoteId QuoteId, err error) {
	quoteId = NilID
	quoteCount := len(s.Quotes)
	if quoteCount <= 0 {
		return quoteId, fmt.Errorf("User has no quotes")
	}

	now := time.Now().UTC()
	seed := now.Year() + now.YearDay()
	rng := rand.New(rand.NewSource(int64(seed)))

	// TODO: actually use weights for meta data like how many times the book was played
	type QuoteMetaData struct {
		tries uint8
	}

	quotesMetaData := make(map[QuoteId]QuoteMetaData, quoteCount)

	quotes := make([]QuoteId, quoteCount)
	triedCount := 0
	collisions := 0

	quoteIndex := 0
	for id := range s.Quotes {
		quotes[quoteIndex] = id
		quoteIndex++
	}
	slices.Sort(quotes) // needed to make the picking determinstic

	for triedCount < quoteCount && collisions < quoteCount*2 {
		quoteIndex := rng.Intn(quoteCount)

		quoteId := quotes[quoteIndex]
		metaData, found := quotesMetaData[quoteId]
		if found && metaData.tries > 0 {
			collisions += 1
			if metaData.tries >= 100 {
				return quoteId, fmt.Errorf("Too many tries on quote %d", quoteId)
			}
			metaData.tries += 1
			quotesMetaData[quoteId] = metaData
			continue
		}

		triedCount += 1
		if slices.Contains(s.Player.SeenQuotes, quoteId) {
			continue
		}

		// Check if book is read
		quote, found := s.Quotes[quoteId]
		if !found {
			err = errors.Join(fmt.Errorf("Couldn't get quote %d back from map", quoteId))
			continue
		}
		book, found := s.Books[quote.BookId]
		if !found {
			err = errors.Join(fmt.Errorf("Couldn't find book with id %d for quote %d\n", quote.BookId, quoteId))
			continue
		}
		if !book.UserData.IsRead() {
			continue
		}
		return quoteId, err
	}

	err = errors.Join(fmt.Errorf("Recycling quote for %s\n", s.Player.UserGRID))
	quoteIndex = rng.Intn(quoteCount)
	return quotes[quoteIndex], err
}
