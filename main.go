package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/charmbracelet/log"
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
		flag.PrintDefaults()
		logg.Fatal("must provide the user arg")
	}

	_, quotes := scrapeGoodreads(userId, options)


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
