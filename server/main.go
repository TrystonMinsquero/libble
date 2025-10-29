package main

import (
	"flag"
	"net/http"
	"os"

	"github.com/charmbracelet/log"
	"github.com/gin-gonic/gin"
	. "libble/shared"
)

var (
	logg = log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: false,
	})
)

func main() {
	var userGRID string
	var cache bool
	flag.StringVar(&userGRID, "user", "", "The user id found in the url")
	flag.BoolVar(&cache, "cache", true, "Will cache the requests")

	flag.Parse()

	options := ScrapeOptions{
		cache: cache,
	}

	if userGRID == "" {
		userGRID, _ = os.LookupEnv("USER_GRID")
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

	r.GET("/daily/:id", func(c *gin.Context) {
		userGRID = c.Param("id")

		// TODO: replace with database id and retrieve data
		if userGRID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Must provide user param"})
			return
		}

		books, quotes, err := scrapeGoodreads(userGRID, options)
		dailyQuoteIndex := PickDailyQuote(UserData{}, books, quotes)
		dailyData := DailyData{
			User:       UserData{UserGRID: userGRID},
			Books:      books,
			Quotes:     quotes,
			DailyQuote: dailyQuoteIndex,
		}

		if err != nil {
			c.Header("error", err.Error())
		}
		c.JSON(http.StatusOK, dailyData)
	})

	r.GET("/scrape/:id", func(c *gin.Context) {
		userGRID = c.Param("id")
		if userGRID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Must provide user param"})
			return
		}
		books, quotes, err := scrapeGoodreads(userGRID, options)

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
