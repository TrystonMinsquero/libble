package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/charmbracelet/log"
	"github.com/gin-gonic/gin"
)

var (
	logg = log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: false,
	})
)

func main() {
	var userId string
	var cache bool
	flag.StringVar(&userId, "user", "", "The user id found in the url")
	flag.BoolVar(&cache, "cache", true, "Will cache the requests")

	flag.Parse()

	options := ScrapeOptions{
		cache: cache,
	}

	if userId == "" {
		userId, _ = os.LookupEnv("USER_ID")
	}

	// Create a Gin router with default middleware (logger and recovery)
	r := gin.Default()

	r.SetTrustedProxies(nil)

	r.LoadHTMLGlob("views/templates/*")
	r.Static("/css", "./views/css")
	r.Static("/js", "./views/js")

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})
	r.GET("/game", func(c *gin.Context) {
		c.HTML(http.StatusOK, "game.html", nil)
	})
	r.GET("/start", func(c *gin.Context) {
		c.HTML(http.StatusOK, "start.html", nil)
	})

	r.GET("/scrape/:id", func(c *gin.Context) {
		userId = c.Param("id")
		if userId == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Must provide user param"})
			return
		}
		books, quotes, err := scrapeGoodreads(userId, options)
		res := gin.H{
			"books":  books,
			"quotes": quotes,
		}
		if err != nil {
			res["error"] = err.Error()
		}
		c.JSON(http.StatusOK, res)
	})

	logg.Fatal(r.Run(":8000"))
}

type Book struct {
	BookId      string   `json:"book_id"`
	Title       string   `json:"title"`
	Author      string   `json:"author"`
	AuthorId    string   `json:"author_id"`
	Stars       uint     `json:"stars"`
	AvgRating   float32  `json:"avg_rating"`
	RatingCount uint     `json:"rating_count"`
	DatesRead   []string `json:"dates_read"`
	DateAdded   string   `json:"date_added"`
}

type Quote struct {
	QuoteId string `json:"quote_id"`
	Likes   uint   `json:"likes"`
	Text    string `json:"text"`

	BookId   string `json:"book_id"`
	AuthorId string `json:"author_id"`
}

func (b Book) getQuoteUrl() string {
	return fmt.Sprintf("%s/book/quotes/%s", domain, b.BookId)
}

func (b Book) isRead() bool {
	return b.Stars > 0
}
