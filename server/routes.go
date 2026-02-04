package main

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	. "libble/shared"

	"github.com/bwmarrin/snowflake"
	"github.com/gin-contrib/cors"
	ginzip "github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func initRouter() *gin.Engine {
	// Run in release mode by default
	if ginMode := os.Getenv(gin.EnvGinMode); ginMode != "" {
		gin.SetMode(ginMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create a Gin router with default middleware (logger and recovery)
	r := gin.Default()

	corsConf := cors.DefaultConfig()
	if !isDebug {
		corsConf.AllowAllOrigins = true
	} else {
		corsConf.AllowOrigins = []string{"https://libble.you"}
	}

	r.Use(
		ginzip.Gzip(ginzip.DefaultCompression),
		cors.New(corsConf),
	)

	r.SetTrustedProxies(nil)
	return r
}

func setupRoutes(r *gin.Engine, node *snowflake.Node) {
	setupUserRoutes(r, node)

	r.GET("/scrape/gr/user-books/:libbleID", func(c *gin.Context) {
		libbleID, err := parseLibbleID(c, "libbleID")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		lock := getUserLock(libbleID)
		lock.Lock()
		defer lock.Unlock()

		// Load player to get userGRID and settings
		player, err := LoadPlayer(libbleID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Player with ID %d not found", libbleID)})
			return
		}

		userGRID := player.UserGRID
		options := player.Settings.ScrapeOptions

		// Scrape books from Goodreads
		books, err := scrapeUserBooks(userGRID, options)

		// Resolve book IDs
		for index := range books {
			ub := &books[index]
			bookID, idErr := GetOrCreateBookID(ub.Book.BookGRID, node)
			if idErr != nil {
				logg.Errorf("Failed to get book ID: %v", idErr)
				bookID = BookID(node.Generate().Int64())
			}
			ub.Book.ID = bookID
		}

		// Save books to database
		if _, saveErr := SaveBooks(books, libbleID); saveErr != nil {
			logg.Errorf("Failed to save books: %v", saveErr)
		}

		res := gin.H{
			"books": books,
		}
		if err != nil {
			res["error"] = fmt.Sprintf("Failed to scrape books from user %s: \n%v", userGRID, err)
		}

		c.JSON(http.StatusOK, res)
	})

	r.GET("/scrape/gr/quotes/:libbleID/:bookGRID", func(c *gin.Context) {
		libbleID, err := parseLibbleID(c, "libbleID")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		bookGRID := c.Param("bookGRID")
		if bookGRID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Must provide bookGRID param"})
			return
		}

		lock := getUserLock(libbleID)
		lock.Lock()
		defer lock.Unlock()

		// Load player to get settings
		player, err := LoadPlayer(libbleID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Player with ID %d not found", libbleID)})
			return
		}

		options := player.Settings.ScrapeOptions

		// Scrape quotes from Goodreads for this specific book
		quotes, err := scrapeBookQuotes(bookGRID, options)

		// Resolve book and quote IDs
		for index := range quotes {
			quote := &quotes[index]

			// Get or create book ID
			bookID, idErr := GetOrCreateBookID(quote.BookGRID, node)
			if idErr != nil {
				logg.Errorf("Failed to get book ID: %v", idErr)
				bookID = BookID(node.Generate().Int64())
			}
			quote.BookID = bookID

			// Get or create quote ID
			quoteID, idErr := GetOrCreateQuoteID(quote.QuoteGRID, node)
			if idErr != nil {
				logg.Errorf("Failed to get quote ID: %v", idErr)
				quoteID = QuoteID(node.Generate().Int64())
			}
			quote.ID = quoteID
		}

		// Save quotes to database
		if _, saveErr := SaveQuotes(quotes); saveErr != nil {
			logg.Errorf("Failed to save quotes: %v", saveErr)
		}

		res := gin.H{
			"quotes": quotes,
		}
		if err != nil {
			res["error"] = fmt.Sprintf("Failed to scrape quotes from book %s: \n%v", bookGRID, err)
		}

		c.JSON(http.StatusOK, res)
	})

	r.GET("/game/player/:id", func(c *gin.Context) {
		libbleID, err := parseLibbleID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		player, err := LoadPlayer(libbleID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Player with ID %d not found", libbleID)})
			return
		}

		c.JSON(http.StatusOK, gin.H{"player": player})
	})

	r.PUT("/game/player/:id", func(c *gin.Context) {
		libbleID, err := parseLibbleID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var player User
		if err := c.BindJSON(&player); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		// Ensure the ID in the body matches the URL
		if player.ID != libbleID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Player ID mismatch"})
			return
		}

		lock := getUserLock(libbleID)
		lock.Lock()
		defer lock.Unlock()

		// Update player in database
		if err := UpdatePlayer(player); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update player: %v", err)})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	r.GET("/game/user-books/:id", func(c *gin.Context) {
		libbleID, err := parseLibbleID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		books, err := LoadUserBooks(libbleID)
		if err != nil {
			errMsg := fmt.Sprintf("Books for player %d not found %v", libbleID, err)
			c.JSON(http.StatusNotFound, gin.H{"error": errMsg})
			return
		}

		c.JSON(http.StatusOK, gin.H{"books": books})
	})

	r.GET("/game/daily/:id", func(c *gin.Context) {
		libbleID, err := parseLibbleID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		lock := getUserLock(libbleID)
		lock.Lock()
		defer lock.Unlock()

		// Load complete SaveData
		data, err := LoadSaveData(libbleID)
		if err != nil {
			errMsg := fmt.Sprintf("Player with ID %d not found: %v", libbleID, err)
			c.JSON(http.StatusNotFound, gin.H{"error": errMsg})
			return
		}

		// Pick today's daily quote
		dailyQuoteId, err := data.PickDailyQuote()
		if dailyQuoteId == NilID {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to pick daily quote: %v", err),
			})
			return
		}

		// Get the quote and book
		quote, err := data.GetQuote(dailyQuoteId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to get quote: %v", err),
			})
			return
		}

		userBook, err := data.GetBook(quote.BookID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to get book: %v", err),
			})
			return
		}

		// Return lightweight payload
		c.JSON(http.StatusOK, gin.H{
			"quote": quote,
			"book":  userBook,
		})
	})
}

type UserCreationProgress struct {
	Mutux  sync.RWMutex
	User   User
	Books  []UserBook
	Quotes []Quote

	InitialQuote           QuoteID
	BookQuotesScraped      uint
	FinishedScrapingQuotes bool

	// if data is synced with db
	UserSynced   bool
	BooksSynced  bool
	QuotesSynced bool

	Error error
}

func (p *UserCreationProgress) State() UserCreateState {
	p.Mutux.RLock()
	defer p.Mutux.RUnlock()
	if p.FinishedScrapingQuotes && p.UserSynced && p.BooksSynced && p.QuotesSynced {
		return UserCreateState_Finished
	}
	return p.User.CreateState
}

func (p *UserCreationProgress) Status() UserCreateStatus {
	p.Mutux.RLock()
	defer p.Mutux.RUnlock()
	var errStr string
	if p.Error != nil {
		errStr = p.Error.Error()
	}
	return UserCreateStatus{
		BooksFound:      uint(len(p.Books)),
		BooksCollected:  p.BookQuotesScraped,
		QuotesCollected: uint(len(p.Quotes)),
		InitialQuote:    p.InitialQuote,
		State:           p.State(),
		Error:           errStr,
	}
}

func (p *UserCreationProgress) BookQuotesScrapeCount() uint {
	count := uint(0)
	p.Mutux.RLock()
	defer p.Mutux.RUnlock()
	for _, book := range p.Books {
		if p.User.Settings.ScrapeOptions.ShouldScrapeQuotes(book) {
			count += 1
		}
	}
	return count
}

func (p *UserCreationProgress) AddError(err error, contextFmt string, args ...any) bool {
	if err == nil {
		return false
	}
	context := fmt.Sprintf(contextFmt, args...)
	if context != "" {
		logg.Errorf("User Creation Error for %d (GRID: %s)\n%s\nError: %v", p.User.ID, p.User.UserGRID, context, err)
	} else {
		logg.Errorf("User Creation Error for %d (GRID: %s)\nError: %v", p.User.ID, p.User.UserGRID, err)
	}
	p.JoinError(err)
	return true
}

func (p *UserCreationProgress) JoinError(err error) {
	p.Mutux.Lock()
	p.Error = errors.Join(p.Error, err)
	p.Mutux.Unlock()
}

var userCreationMutex sync.RWMutex
var userCreationProgress = make(map[UserID]*UserCreationProgress, 0)

func setupUserRoutes(r *gin.Engine, node *snowflake.Node) {
	user := r.Group("/user")
	user.GET("/lookup/gr/:GRID", func(c *gin.Context) {
		userGRID := c.Param("GRID")
		if userGRID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Must provide GRID param"})
			return
		}

		summaries, err := GetUsersByGRID(userGRID)
		if err != nil {
			// No users found - return empty list
			c.JSON(http.StatusOK, gin.H{"users": []UserSummary{}})
			return
		}

		c.JSON(http.StatusOK, gin.H{"users": summaries})
	})

	user.POST("/create", func(c *gin.Context) {
		var req UserCreateRequest
		var res UserCreateResponse
		if err := c.BindJSON(&req); err != nil {
			res.Error = fmt.Sprintf("Invalid request body. %v", err)
			c.JSON(http.StatusBadRequest, res)
			return
		}

		req.GRID = strings.TrimSpace(req.GRID)
		if req.GRID == "" {
			res.Error = "Missing 'grid' field"
			c.JSON(http.StatusBadRequest, res)
			return
		}

		// Create new player
		userID := UserID(node.Generate().Int64())
		settings := UserSettings{
			GameSettings:  DefaultGameSettings(),
			ScrapeOptions: req.ScrapeOptions,
		}

		user := User{
			ID:          userID,
			UserGRID:    req.GRID,
			Settings:    settings,
			SeenQuotes:  []QuoteID{},
			Games:       []Game{},
			CreateState: UserCreateState_InProgress,
		}

		progress := NewUserCreationProgress(user)
		go progress.StartCreation(user)
		res.UserID = user.ID
		c.JSON(http.StatusCreated, res)
	})

	user.GET("/create/status/:ID", func(ctx *gin.Context) {
		var status UserCreateStatus
		userID, err := parseLibbleID(ctx, "ID")
		if err != nil {
			status.Error = err.Error()
			ctx.JSON(http.StatusBadRequest, status)
			return
		}
		userCreationMutex.RLock()
		progress, found := userCreationProgress[userID]
		userCreationMutex.RUnlock()

		if found {
			status = progress.Status()
			ctx.JSON(http.StatusOK, status)
			return
		}

		user, err := GetUser(userID)
		if err != nil {
			status.Error = err.Error()
			ctx.JSON(http.StatusNotFound, status)
			return
		}
		status.State = user.CreateState
		switch user.CreateState {
		case UserCreateState_Finished:
			ctx.JSON(http.StatusAccepted, status)
		case UserCreateState_InProgress:
			// We know this was in progress on another server,
			// otherwise would have caught from our userCreationProgress search.
			user.CreateState = UserCreateState_NeedsReboot
			status.State = UserCreateState_NeedsReboot
			fallthrough
		case UserCreateState_NeedsReboot:
			// Caches the state found so we don't have to ask the database again
			NewUserCreationProgress(user)
			ctx.JSON(http.StatusAccepted, status)
		}

		ctx.JSON(http.StatusBadRequest, status)
	})
}

func NewUserCreationProgress(user User) *UserCreationProgress {
	p := new(UserCreationProgress)
	p.User = user
	userCreationMutex.Lock()
	userCreationProgress[user.ID] = p
	userCreationMutex.Unlock()
	return p
}

// Will start all the jobs to create the user. See the status from /user/create/status
func (p *UserCreationProgress) StartCreation(user User) {
	user.CreateState = UserCreateState_InProgress
	// Sync user to database
	go func() {
		tx := db.Create(&user)
		if tx.Error != nil {
			p.AddError(tx.Error, "Failed to sync user to database")
		} else {
			p.UserSynced = true
		}
	}()

	options := user.Settings.ScrapeOptions

	// Scrape all the books
	go func() {
		books, scrapeErr := scrapeUserBooks(user.UserGRID, options)
		p.Books = books

		p.AddError(scrapeErr, "Issue(s) when scraping user books")
		if len(books) == 0 {
			p.JoinError(fmt.Errorf("No books found for %s", user.UserGRID))
			return
		}

		go func() {
			_, err := SaveBooks(p.Books, p.User.ID)
			if err != nil {
				p.AddError(err, "Failed to sync books to database")
			} else {
				p.BooksSynced = true
			}
		}()

		go func() {
			p.Mutux.Lock()
			if len(p.Quotes) > 0 {
				panic("Trying to add quotes to progress twice!")
			}
			p.Quotes = make([]Quote, 0, 100)
			p.Mutux.Unlock()

			var wg sync.WaitGroup

			for _, userBook := range books {
				if options.ShouldScrapeQuotes(userBook) {
					continue
				}

				book := userBook.Book

				wg.Add(1)
				go func() {
					defer wg.Done()

					bookQuotes, err := scrapeBookQuotes(book.BookGRID, options)
					p.AddError(err, "Issue when scraping quotes for book GRID %s", book.BookGRID)
					if len(bookQuotes) == 0 {
						return
					}

					// TODO: delete
					fakeSeconds := rand.Intn(5)
					time.Sleep(time.Second * time.Duration(fakeSeconds))

					p.Mutux.Lock()
					defer p.Mutux.Unlock()

					p.BookQuotesScraped += 1
					logg.Debugf("Scraped %d Quotes from %s", len(bookQuotes), book.Title)
					p.Quotes = append(p.Quotes, bookQuotes...)
				}()
			}
			wg.Wait()

			p.Mutux.Lock()
			p.FinishedScrapingQuotes = true
			p.Mutux.Unlock()
			go func() {
				_, err := SaveQuotes(p.Quotes)
				if err != nil {
					p.AddError(err, "Failed to sync quotes to database")
				} else {
					p.Mutux.Lock()
					p.QuotesSynced = true
					p.Mutux.Unlock()
				}
			}()
		}()
	}()
}

func setupScrapeRoutes(r *gin.Engine) {
	dev := r.Group("/dev")
	scrapeGR := dev.Group("/scrape/gr")

	options := DefaultScrapeOptions()

	scrapeGR.GET("/user-books/:GRID", func(ctx *gin.Context) {
		userGRID := ctx.Param("GRID")
		books, err := scrapeUserBooks(userGRID, options)
		res := gin.H{
			"books": books,
		}
		status := http.StatusOK
		if err != nil {
			res["error"] = fmt.Sprintf("Failed to scrape books from user %s: \n%v", userGRID, err)
			status = http.StatusFailedDependency
		}

		ctx.JSON(status, res)
	})

	scrapeGR.GET("/book/:GRID", func(ctx *gin.Context) {
		bookGRID := ctx.Param("GRID")
		book, err := scrapeBook(bookGRID, options)
		res := gin.H{
			"book": book,
		}
		status := http.StatusOK
		if err != nil {
			res["error"] = fmt.Sprintf("Failed to scrape book from %s: \n%v", bookGRID, err)
			status = http.StatusFailedDependency
		}

		ctx.JSON(status, res)
	})
}

func hostSite(r *gin.Engine) {
	const siteDir = "./site"
	if entries, err := os.ReadDir(siteDir); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			filePath := path.Join(siteDir, name)
			if entry.IsDir() {
				r.Static("/"+name, filePath)
			} else {
				r.StaticFile(name, filePath)
			}
			if name == "index.html" {
				r.StaticFile("/", filePath)
			}
		}
	} else {
		logg.Errorf("Failed reading from %s:\n%v", siteDir, err)
	}
}

func parseLibbleID(c *gin.Context, paramName string) (UserID, error) {
	libbleIDStr := c.Param(paramName)
	libbleIDUint, err := strconv.ParseUint(libbleIDStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid libbleID format")
	}
	return UserID(libbleIDUint), nil
}
