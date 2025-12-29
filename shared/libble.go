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

	Books  []UserBook `json:"books"`
	Quotes []Quote    `json:"quotes"`

	NeedsServer bool
	bookMap     map[BookId]int
	quoteMap    map[QuoteId]int
}

func (s *SaveData) PopulateQuoteLookup() {
	s.quoteMap = make(map[QuoteId]int)
	for index, quote := range s.Quotes {
		s.quoteMap[quote.QuoteId] = index
	}
}

func (s *SaveData) PopulateLookups() {
	s.bookMap = make(map[BookId]int)
	for index, book := range s.Books {
		s.bookMap[book.Book.BookId] = index
	}
	s.PopulateQuoteLookup()
}

func NewSaveData(libbleID DBID, userGRID string, books []UserBook, quotes []Quote) SaveData {
	var data SaveData
	data.Player.UserGRID = userGRID
	data.Player.ID = libbleID
	data.Player.Settings = DefaultPlayerSettings()

	// Initialize maps
	data.Books = books
	data.Quotes = quotes

	data.PopulateLookups()

	// Initialize empty slices
	data.Player.SeenQuotes = []QuoteId{}
	data.Player.Games = []Game{}
	return data
}

func (s SaveData) GetBook(ID BookId) (UserBook, error) {
	index, ok := s.bookMap[ID]
	if !ok {
		return UserBook{}, fmt.Errorf("Book %v not in map", ID)
	}
	if index < 0 || index >= len(s.Books) {
		return UserBook{}, fmt.Errorf("Book %v had index %d which is out of bounds ", ID, index)
	}
	return s.Books[index], nil
}

func (s SaveData) FindQuote(ID QuoteId) Quote {
	for _, quote := range s.Quotes {
		if quote.QuoteId == ID {
			return quote
		}
	}
	return Quote{}
}

func (s SaveData) GetQuote(ID QuoteId) (Quote, error) {
	index, ok := s.quoteMap[ID]
	if !ok {
		return Quote{}, fmt.Errorf("Quote %v not in map. Quotes Map:\n%v", ID, s.quoteMap)
	}
	if index < 0 || index >= len(s.Quotes) {
		return Quote{}, fmt.Errorf("Quote %v had index %d which is out of bounds ", ID, index)
	}
	return s.Quotes[index], nil
}

func (s *SaveData) AddQuote(quote Quote) {
	index, found := s.quoteMap[quote.QuoteId]
	if found {
		s.Quotes[index] = quote
	} else {
		s.Quotes = append(s.Quotes, quote)
		s.PopulateLookups()
	}
}

func (s SaveData) FindBookId(query string) BookId {
	query = strings.ToLower(strings.TrimSpace(query))
	for _, book := range s.Books {
		target := book.Book.CleanTitle()
		target = strings.ToLower(strings.TrimSpace(target))
		if target == query {
			return book.Book.BookId
		}
	}
	return NilID
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
	MinQuoteLikes    int
	MaxQuoteForBook  uint
	UseCache         bool
}

func DefaultScrapeOptions() ScrapeOptions {
	return ScrapeOptions{
		MinPersonalStars: 4,
		MinQuoteLikes:    10,
		MaxQuoteForBook:  50,
		UseCache:         true,
	}
}

func (o ScrapeOptions) ShouldScrapeQuotes(userBook UserBook) bool {
	return userBook.UserData.Stars >= o.MinPersonalStars
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

	quote, err := data.GetQuote(g.QuoteID)
	if err != nil {
		return err
	}
	g.Quote = quote
	g.BookId = quote.BookId

	// Get the book from the map
	book, err := data.GetBook(g.BookId)
	if err != nil {
		return fmt.Errorf("Daily Quote's book Id was not found in books map:\n%v", err)
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

func (p *Player) TodaysGame() *Game {
	if len(p.Games) > 0 {
		MaxLookback := 5
		lastValidIndex := max(len(p.Games)-MaxLookback-1, 0)
		for i := len(p.Games) - 1; i >= lastValidIndex; i-- {
			game := &p.Games[i]
			if DateSeed(time.Now()) == DateSeed(game.Date) {
				return game
			}
		}
	}
	return nil
}

func (p *Player) InitTodaysGame(data SaveData, dailyQuote QuoteId) (game *Game, err error) {
	if game = p.TodaysGame(); game != nil {
		game.QuoteID = dailyQuote
		err = game.Init(data)
		return game, err
	}

	p.Games = append(p.Games, Game{
		QuoteID: dailyQuote,
		Date:    time.Now(),
		Guesses: make([]BookId, 0),
	})
	game = &p.Games[len(p.Games)-1]
	err = errors.Join(game.Init(data))
	return game, err
}

type Book struct {
	BookId      BookId  `json:"libble_id"`
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
	QuoteId   QuoteId `json:"libble_id"`
	QuoteGRID string  `json:"quote_gr_id"`
	Likes     int     `json:"likes"`
	Text      string  `json:"text"`

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

func ToCST(t time.Time) time.Time {
	// Intentially not caring about daylight savings, just need this to be the same
	// result called from anywhere
	cst := time.FixedZone("CST", -6*60*60)
	return t.UTC().In(cst)
}

func DateSeed(t time.Time) int {
	t = ToCST(t)
	return t.Year() + t.YearDay()
}

func (s SaveData) PickDailyQuote() (quoteId QuoteId, err error) {
	quoteId = NilID
	quoteCount := len(s.Quotes)
	if quoteCount <= 0 {
		return quoteId, fmt.Errorf("User has no quotes")
	}

	options := s.Player.Settings.ScrapeOptions

	seed := int64(DateSeed(time.Now()))
	rng := rand.New(rand.NewSource(seed))

	// TODO: actually use weights for meta data like how many times the book was played
	type QuoteMetaData struct {
		tries uint8
	}

	quotesMetaData := make(map[QuoteId]QuoteMetaData, quoteCount)

	quotes := make([]QuoteId, quoteCount)
	triedCount := 0
	collisions := 0

	quoteIndex := 0
	for _, quote := range s.Quotes {
		quotes[quoteIndex] = quote.QuoteId
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
		quote, err := s.GetQuote(quoteId)
		if err != nil {
			err = errors.Join(err)
			continue
		}
		book, err := s.GetBook(quote.BookId)
		if err != nil {
			err = errors.Join(err)
			continue
		}
		if !options.ShouldScrapeQuotes(book) {
			continue
		}
		return quoteId, err
	}

	err = errors.Join(fmt.Errorf("Recycling quote for %s\n", s.Player.UserGRID))
	quoteIndex = rng.Intn(quoteCount)
	return quotes[quoteIndex], err
}

// UserSummary contains summary information about a user
type UserSummary struct {
	LibbleID   DBID   `json:"libble_id"`
	GameCount  int    `json:"game_count"`
	LastPlayed string `json:"last_played"` // Empty string if never played
}
