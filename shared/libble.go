package shared

import (
	"errors"
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"time"
	"unicode"
)

type DBID uint64

type QuoteID DBID
type BookID DBID
type UserID DBID

const NilID = 0

type User struct {
	ID       UserID       `json:"user_id" gorm:"primaryKey;column:user_id"`
	UserGRID string       `json:"user_gr_id,omitempty" gorm:"column:user_gr_id;not null;index"`
	Settings UserSettings `json:"settings" gorm:"type:text;serializer:json"`

	Email      string     `json:"email,omitempty" gorm:"not null"`
	VerifiedAt *time.Time `json:"verified_at" gorm:"column:verified_email_time"`

	SeenQuotes []QuoteID `json:"seen_quote_ids" gorm:"type:text;serializer:json"`
	Games      []Game    `json:"games" gorm:"type:text;serializer:json"`
}

func (User) TableName() string {
	return "users"
}

type App struct {
	User User `json:"user"`

	Books  []UserBook `json:"books"`
	Quotes []Quote    `json:"quotes"`

	NeedsServer bool
	bookMap     map[BookID]int
	quoteMap    map[QuoteID]int
}

func (a *App) PopulateQuoteLookup() {
	a.quoteMap = make(map[QuoteID]int)
	for index, quote := range a.Quotes {
		a.quoteMap[quote.ID] = index
	}
}

func (a *App) PopulateLookups() {
	a.bookMap = make(map[BookID]int)
	for index, book := range a.Books {
		a.bookMap[book.Book.ID] = index
	}
	a.PopulateQuoteLookup()
}

func NewApp(userID UserID, userGRID string, books []UserBook, quotes []Quote) App {
	var app App
	app.User.UserGRID = userGRID
	app.User.ID = userID
	app.User.Settings = DefaultUserSettings()
	app.User.SeenQuotes = []QuoteID{}
	app.User.Games = []Game{}

	// Initialize maps
	app.Books = books
	app.Quotes = quotes

	app.PopulateLookups()

	return app
}

func (a App) GetBook(ID BookID) (UserBook, error) {
	index, ok := a.bookMap[ID]
	if !ok {
		return UserBook{}, fmt.Errorf("Book %v not in map", ID)
	}
	if index < 0 || index >= len(a.Books) {
		return UserBook{}, fmt.Errorf("Book %v had index %d which is out of bounds ", ID, index)
	}
	return a.Books[index], nil
}

func (a App) FindQuote(ID QuoteID) Quote {
	for _, quote := range a.Quotes {
		if quote.ID == ID {
			return quote
		}
	}
	return Quote{}
}

func (a App) GetQuote(ID QuoteID) (Quote, error) {
	index, ok := a.quoteMap[ID]
	if !ok {
		return Quote{}, fmt.Errorf("Quote %v not in map. Quotes Map:\n%v", ID, a.quoteMap)
	}
	if index < 0 || index >= len(a.Quotes) {
		return Quote{}, fmt.Errorf("Quote %v had index %d which is out of bounds ", ID, index)
	}
	return a.Quotes[index], nil
}

func (a *App) AddQuote(quote Quote) {
	index, found := a.quoteMap[quote.ID]
	if found {
		a.Quotes[index] = quote
	} else {
		a.Quotes = append(a.Quotes, quote)
		a.PopulateLookups()
	}
}

func (a App) FindBookID(query string) BookID {
	query = strings.ToLower(strings.TrimSpace(query))
	for _, book := range a.Books {
		target := book.Book.CleanTitle()
		target = strings.ToLower(strings.TrimSpace(target))
		if target == query {
			return book.Book.ID
		}
	}
	return NilID
}

type UserBook struct {
	UserID   UserID       `json:"-" gorm:"primaryKey;column:user_id"`
	BookID   BookID       `json:"-" gorm:"primaryKey;column:book_id"`
	Book     Book         `json:"book" gorm:"foreignKey:BookID;references:BookID"`
	UserData UserBookData `json:"user_book_data" gorm:"embedded"`
}

func (UserBook) TableName() string {
	return "user_books"
}

type UserBookData struct {
	Stars     uint        `json:"stars" gorm:"not null"`
	DatesRead []time.Time `json:"dates_read" gorm:"type:text;serializer:json;column:dates_read"`
	DateAdded time.Time   `json:"date_added" gorm:"type:text;serializer:json;column:date_added"`
}

func (b UserBookData) LastReadDate() (time.Time, error) {
	best := time.Time{}
	checkDate := func(date time.Time) {
		if !date.Equal(time.Time{}) && date.After(best) {
			best = date
		}
	}
	for _, date := range b.DatesRead {
		checkDate(date)
	}
	if best.Equal(time.Time{}) {
		checkDate(b.DateAdded)
	}
	if best.Equal(time.Time{}) {
		return best, errors.New("No date")
	}
	return best, nil
}

func getInitials(words []string) string {
	var sb strings.Builder
	for _, word := range words {
		runes := []rune(word)
		if len(runes) <= 0 {
			continue
		}
		sb.WriteRune(unicode.ToUpper(runes[0]))
		sb.WriteRune('.')
	}
	return sb.String()
}

// Arrange words so they come in correct order
// Ex: Green, John        -> John Green        -> J.G.
// Ex: Van Helt, Shelby   -> Shelby Van Helt   -> S.V.H.
// Ex: Last, First Middle -> First Middle Last -> F.M.L.
func (b Book) AuthorInitials() string {
	author := b.Author
	if author == "" {
		return author
	}
	chunks := strings.Split(author, ",")
	var words []string
	if len(chunks) >= 2 {
		for _, chunk := range chunks[1:] {
			words = append(words, strings.Split(chunk, " ")...)
		}
		words = append(words, strings.Split(chunks[0], " ")...)
	} else {
		words = strings.Split(chunks[0], " ")
	}
	return getInitials(words)
}

type UserSettings struct {
	GameSettings  GameSettings  `json:"game_settings"`
	ScrapeOptions ScrapeOptions `json:"scrape_options"`
}

func DefaultUserSettings() UserSettings {
	return UserSettings{
		GameSettings:  DefaultGameSettings(),
		ScrapeOptions: DefaultScrapeOptions(),
	}
}

type Shelf struct {
	Name  string `json:"name"`
	Count uint   `json:"count"` // amount of people assigned this shelf
}

type ScrapeOptions struct {
	MinPersonalStars uint `json:"min_personal_stars"`
	MinQuoteLikes    int  `json:"min_quote_likes"`
	MaxQuoteForBook  uint `json:"max_quotes_per_book"`
	UseCache         bool `json:"use_cache"`
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
	MaxGuesses   int    `json:"max_guesses"`
	HintsEnabled []Hint `json:"allowed_hints"`
}

func DefaultGameSettings() GameSettings {
	return GameSettings{
		MaxGuesses: 5,
		HintsEnabled: []Hint{
			HintTime, HintSelfRating, HintAuthorInitial,
		},
	}
}

type Hint string

const (
	HintTime          Hint = "Time"
	HintSelfRating    Hint = "SelfRating"
	HintAuthorInitial Hint = "AuthorInitial"

	// TODO: Implement these
	HintBookReadBefore Hint = "BookReadBefore"
	HintBookReadAfter  Hint = "BookReadAfter"

	// TODO: Implement these (need to update scraper)
	HintGenre      Hint = "Genre"
	HintCharacters Hint = "Characters"
	HintSelfReview Hint = "SelfReview"
)

type UsedHint struct {
	Kind       Hint `json:"hint"`
	GuessIndex int  `json:"guess_index"` // which guess count the hint was used on
}

type Game struct {
	QuoteID QuoteID    `json:"quote_id"`
	Date    time.Time  `json:"date_started"`
	Guesses []BookID   `json:"guesses"`
	Hints   []UsedHint `json:"used_hints"`

	// Runtime data
	Quote    Quote        `json:"-"`
	BookID   BookID       `json:"-"`
	Book     UserBook     `json:"-"`
	Settings GameSettings `json:"-"`
}

func (g *Game) Init(app App) error {
	g.Settings = app.User.Settings.GameSettings
	if g.Settings.MaxGuesses < 1 {
		// This should never happen, but just in case they can at least play
		g.Settings.MaxGuesses = 1
	}

	// Get the quote from the map

	quote, err := app.GetQuote(g.QuoteID)
	if err != nil {
		return err
	}
	g.Quote = quote
	g.BookID = quote.BookID

	// Get the book from the map
	book, err := app.GetBook(g.BookID)
	if err != nil {
		return fmt.Errorf("Daily Quote's book Id was not found in books map:\n%v", err)
	}
	g.Book = book
	return nil
}

func (g Game) Started() bool {
	return g.Attempts() > 0 || len(g.Hints) > 0
}

func (g Game) UsedHint(kind Hint) bool {
	for _, hint := range g.Hints {
		if hint.Kind == kind {
			return true
		}
	}
	return false
}

func (g *Game) UseHint(kind Hint) {
	g.Hints = append(g.Hints, UsedHint{
		Kind:       kind,
		GuessIndex: g.Attempts(),
	})
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
	return g.Guesses[len(g.Guesses)-1] == g.BookID
}

func (p *User) TodaysGame() *Game {
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

func (p *User) InitTodaysGame(app App, dailyQuote QuoteID) (game *Game, err error) {
	if game = p.TodaysGame(); game != nil {
		game.QuoteID = dailyQuote
		err = game.Init(app)
		return game, err
	}

	p.Games = append(p.Games, Game{
		QuoteID: dailyQuote,
		Date:    time.Now(),
		Guesses: make([]BookID, 0),
	})
	game = &p.Games[len(p.Games)-1]
	err = errors.Join(game.Init(app))
	return game, err
}

type Book struct {
	ID          BookID    `json:"book_id" gorm:"primaryKey;column:book_id"`
	BookGRID    string    `json:"book_gr_id" gorm:"uniqueIndex;column:book_gr_id;not null"`
	Title       string    `json:"title" gorm:"not null"`
	Author      string    `json:"author" gorm:"not null"`
	AuthorGRID  string    `json:"author_gr_id" gorm:"column:author_gr_id;not null"`
	AvgRating   float32   `json:"avg_rating" gorm:"not null"`
	RatingCount uint      `json:"rating_count" gorm:"not null"`
	LastUpdated time.Time `json:"last_updated" gorm:"column:last_updated"`
}

func (Book) TableName() string {
	return "books"
}

func (b Book) CleanTitle() string {
	text := strings.Join(strings.Fields(b.Title), " ")
	end := strings.LastIndex(text, "(")
	if end > 0 {
		return strings.TrimSpace(text[:end-1])
	}
	return strings.TrimSpace(text)
}

type Quote struct {
	ID          QuoteID   `json:"libble_id" gorm:"primaryKey;column:quote_id"`
	QuoteGRID   string    `json:"quote_gr_id" gorm:"uniqueIndex;column:quote_gr_id;not null"`
	LastUpdated time.Time `json:"last_updated" gorm:"column:last_updated"`
	BookID      BookID    `json:"book_id" gorm:"index;column:book_id;not null"`
	BookGRID    string    `json:"book_gr_id" gorm:"-"` // Not stored in DB, computed field
	Text        string    `json:"text" gorm:"type:text;not null"`
	Likes       int       `json:"likes" gorm:"not null"`
}

func (Quote) TableName() string {
	return "quotes"
}

func (b UserBookData) IsRead() bool {
	if b.Stars > 0 {
		return true
	}
	for _, date := range b.DatesRead {
		if !date.Equal(time.Time{}) {
			return true
		}
	}
	return false
}

var cst = time.FixedZone("CST", -6*60*60)

func ToCST(t time.Time) time.Time {
	// Intentionally not caring about daylight savings,
	// just need this to be the same result called from anywhere
	return t.UTC().In(cst)
}

func DateSeed(t time.Time) int {
	t = ToCST(t)
	return t.Year() + t.YearDay()
}

func (a App) PickDailyQuote() (quoteID QuoteID, err error) {
	// NOTE: This needs to be deterministic because we don't save quotes the user might see.
	// We only change the status of a quote when they take an action (like skipping or guessing).
	// So when they refresh the page, we need to return the same quote.

	quoteID = NilID
	quoteCount := len(a.Quotes)
	if quoteCount <= 0 {
		return quoteID, fmt.Errorf("User has no quotes")
	}

	options := a.User.Settings.ScrapeOptions

	seed := int64(DateSeed(time.Now()))
	rng := rand.New(rand.NewSource(seed))

	// TODO: actually use weights for meta data. Probably use some sort of heuristic
	// Increase chance to pick quote when book:
	// - not used in previous game
	// - recently read
	// - player gave more stars
	// - has more likes on good reads
	// can probably think of more things...

	type QuoteMetaData struct {
		tries uint8
	}

	quotesMetaData := make(map[QuoteID]QuoteMetaData, quoteCount)

	quotes := make([]QuoteID, quoteCount)
	triedCount := 0
	collisions := 0

	quoteIndex := 0
	for _, quote := range a.Quotes {
		quotes[quoteIndex] = quote.ID
		quoteIndex++
	}
	slices.Sort(quotes) // required to make picking deterministic

	for triedCount < quoteCount && collisions < quoteCount*2 {
		quoteIndex := rng.Intn(quoteCount)

		quoteID := quotes[quoteIndex]
		metaData, found := quotesMetaData[quoteID]
		if found && metaData.tries > 0 {
			collisions += 1
			if metaData.tries >= 100 {
				return quoteID, fmt.Errorf("Too many tries on quote %d", quoteID)
			}
			metaData.tries += 1
			quotesMetaData[quoteID] = metaData
			continue
		}

		triedCount += 1
		if slices.Contains(a.User.SeenQuotes, quoteID) {
			continue
		}

		// Check if book is read
		quote, err := a.GetQuote(quoteID)
		if err != nil {
			err = errors.Join(err)
			continue
		}
		book, err := a.GetBook(quote.BookID)
		if err != nil {
			err = errors.Join(err)
			continue
		}
		if !options.ShouldScrapeQuotes(book) {
			continue
		}
		return quoteID, err
	}

	err = errors.Join(fmt.Errorf("Recycling quote for %s\n", a.User.UserGRID))
	quoteIndex = rng.Intn(quoteCount)
	return quotes[quoteIndex], err
}

// UserSummary contains summary information about a user
type UserSummary struct {
	LibbleID   UserID `json:"libble_id"`
	EmailHint  string `json:"email_hint"`
	GameCount  int    `json:"game_count"`
	LastPlayed string `json:"last_played"` // Empty string if never played
}
