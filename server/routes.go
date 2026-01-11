package main

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

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
		books, err := scrapeBooks(userGRID, options)

		// Resolve book IDs
		for index := range books {
			ub := &books[index]
			bookID, idErr := GetOrCreateBookID(ub.Book.BookGRID, node)
			if idErr != nil {
				logg.Errorf("Failed to get book ID: %v", idErr)
				bookID = BookId(node.Generate().Int64())
			}
			ub.Book.BookId = bookID
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
		quotes, err := scrapeQuotes(bookGRID, options)

		// Resolve book and quote IDs
		for index := range quotes {
			quote := &quotes[index]

			// Get or create book ID
			bookID, idErr := GetOrCreateBookID(quote.BookGRID, node)
			if idErr != nil {
				logg.Errorf("Failed to get book ID: %v", idErr)
				bookID = BookId(node.Generate().Int64())
			}
			quote.BookId = bookID

			// Get or create quote ID
			quoteID, idErr := GetOrCreateQuoteID(quote.QuoteGRID, node)
			if idErr != nil {
				logg.Errorf("Failed to get quote ID: %v", idErr)
				quoteID = QuoteId(node.Generate().Int64())
			}
			quote.QuoteId = quoteID
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

		var player Player
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

		userBook, err := data.GetBook(quote.BookId)
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
		type UserCreateRequest struct {
			GRID          string        `json:"grid"`
			ScrapeOptions ScrapeOptions `json:"scrape_options"`
		}

		var req UserCreateRequest
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		req.GRID = strings.TrimSpace(req.GRID)
		if req.GRID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing 'grid' field"})
			return
		}

		// Create new player
		libbleID := LibbleID(node.Generate().Int64())
		settings := PlayerSettings{
			GameSettings:  DefaultGameSettings(),
			ScrapeOptions: req.ScrapeOptions,
		}

		if err := CreateUser(libbleID, req.GRID, settings); err != nil {
			logg.Errorf("Failed to create user: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}

		player := Player{
			ID:         libbleID,
			UserGRID:   req.GRID,
			Settings:   settings,
			SeenQuotes: []QuoteId{},
			Games:      []Game{},
		}

		logg.Infof("Created new user %d from GRID '%s'", libbleID, req.GRID)
		c.JSON(http.StatusCreated, gin.H{"player": player})
	})
}

func setupScrapeRoutes(r *gin.Engine) {
	dev := r.Group("/dev")
	scrapeGR := dev.Group("/scrape/gr")

	options := DefaultScrapeOptions()

	scrapeGR.GET("/user-books/:GRID", func(ctx *gin.Context) {
		userGRID := ctx.Param("GRID")
		books, err := scrapeBooks(userGRID, options)
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

func parseLibbleID(c *gin.Context, paramName string) (LibbleID, error) {
	libbleIDStr := c.Param(paramName)
	libbleIDUint, err := strconv.ParseUint(libbleIDStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid libbleID format")
	}
	return LibbleID(libbleIDUint), nil
}
