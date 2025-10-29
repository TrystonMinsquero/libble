package main

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"

	. "libble/shared"
	"github.com/gocolly/colly"
)

const (
	domain        = "www.goodreads.com"
	requestCache  = "./.request_cache"
	minQuoteLikes = 5
)

type ScrapeOptions struct {
	cache bool
}

var scrapeOptions ScrapeOptions

func scrapeGoodreads(userId string, options ScrapeOptions) ([]Book, []Quote, error) {
	scrapeOptions = options

	books, err := scrapeBooks(userId, options)
	if err != nil {
		return books, nil, err
	}

	readCount := 0
	quotes := make([]Quote, 0, 100)

	var wg sync.WaitGroup
	var mutex sync.Mutex
	for _, book := range books {
		if !book.IsRead() {
			continue
		}
		readCount += 1
		wg.Add(1)
		go func() {
			defer wg.Done()

			url := "https://" + domain + "/book/quotes/" + book.BookId
			bookQuotes, err := scrapeQuotes(url, options)
			if err != nil {
				logg.Error(err)
				return
			}

			mutex.Lock()
			defer mutex.Unlock()
			// logg.Infof("Scraped %d Quotes from %s", len(bookQuotes), book.Title)
			quotes = append(quotes, bookQuotes...)
		}()
	}

	wg.Wait()
	logg.Printf("Total Quote Count: %d", len(quotes))
	logg.Printf("Total Book Count: %d", len(books))
	logg.Printf("Read Book Count: %d", readCount)

	return books, quotes, err
}

func scrapeForNextPage(e *colly.HTMLElement) string {
	if href := e.Attr("href"); href != "" {
		nextPageUrl, err := url.Parse(href)
		if err != nil {
			logg.Errorf("Error parsing next page href: %v", err)
			return ""
		}

		reqUrl := e.Request.URL
		nextPageUrl.Host = reqUrl.Host
		nextPageUrl.Scheme = reqUrl.Scheme
		return nextPageUrl.String()
	}
	return ""
}

func tryVisitNextPage(nextPageElem *colly.HTMLElement) {
	if url := scrapeForNextPage(nextPageElem); url != "" {
		nextPageElem.Request.Visit(url)
	}
}

func parseId(href string) string {
	index := strings.LastIndexByte(href, '/')
	if index >= 0 && len(href) > 0 {
		return href[index+1:]
	}
	return ""
}

func scrapeBooks(userId string, options ScrapeOptions) ([]Book, error) {
	bookCollector := colly.NewCollector(
		defaultCollectorOptions(options),
	)

	bookCollector.OnError(func(r *colly.Response, err error) {
		logg.Errorf("Error when collecting book at %+v\n%v", r, err)
	})

	bookCollector.OnHTML("a.next_page", tryVisitNextPage)

	books := make([]Book, 0, 20)

	bookCollector.OnHTML("tr.bookalike", func(bookElem *colly.HTMLElement) {
		book, err := scrapeBook(bookElem)
		if err == nil {
			books = append(books, book)
		} else {
			logg.Errorf("%v", err)
		}
	})

	// lastCount := 0
	// pageCount := 0
	//
	// // triggered once scraping is done (e.g., write the data to a CSV file)
	// bookCollector.OnScraped(func(r *colly.Response) {
	// 	// grew := len(books) - lastCount
	// 	// logg.Infof("scraped %d books from %v", grew, r.Request.URL)
	// 	// for i := lastCount; i < len(books); i += 1 {
	// 	// 	logg.Infof("\t%s: %+v", books[i].Title, books[i])
	// 	// }
	// 	lastCount = len(books)
	// 	pageCount += 1
	// })

	url := "https://" + domain + "/review/list/" + userId
	if err := bookCollector.Visit(url); err != nil {
		logg.Error(err)
		return books, err
	}
	// logg.Printf("scraped %d books on %d pages", len(books), pageCount)
	return books, nil
}

func defaultCollectorOptions(options ScrapeOptions) func(*colly.Collector) {
	return func(c *colly.Collector) {
		if options.cache {
			c.CacheDir = requestCache
		}
		colly.AllowedDomains(domain)
		colly.Async(true)
	}
}

func scrapeBook(bookElem *colly.HTMLElement) (Book, error) {
	var book Book
	bookElem.ForEach("td.field", func(_ int, fieldElem *colly.HTMLElement) {
		class := fieldElem.Attr("class")
		class = strings.ReplaceAll(class, "field", "")
		class = strings.TrimSpace(class)
		if class == "" {
			return
		}

		switch class {
		case "title":
			book.Title = fieldElem.ChildAttr("a", "title")
			book.BookId = parseId(fieldElem.ChildAttr("a", "href"))
		case "author":
			book.Author = fieldElem.ChildText("a")
			book.AuthorId = parseId(fieldElem.ChildAttr("a", "href"))
		case "avg_rating":
			avgRating, err := strconv.ParseFloat(fieldElem.ChildText("div.value"), 32)
			if err == nil {
				book.AvgRating = float32(avgRating)
			} else {
				logg.Error("Error getting avg_rating", err)
			}
		case "num_ratings":
			valueText := fieldElem.ChildText("div.value")
			valueText = strings.ReplaceAll(valueText, ",", "")
			numRating, err := strconv.ParseUint(valueText, 10, 32)
			if err == nil {
				book.RatingCount = uint(numRating)
			} else {
				logg.Error("Error getting num_ratings", err)
			}
		case "rating":
			valueText := fieldElem.ChildText("div.value")
			switch valueText {
			case "did not like it":
				book.Stars = 1
			case "it was ok":
				book.Stars = 2
			case "liked it":
				book.Stars = 3
			case "really liked it":
				book.Stars = 4
			case "it was amazing":
				book.Stars = 5
			case "":
				book.Stars = 0 // Not rated
			default:
				logg.Error("Was unable to translate '%s' to star count for %s",
					valueText, book.Title)
			}
		case "date_read":
			fieldElem.ForEach("div.date_row", func(_ int, dateElem *colly.HTMLElement) {
				book.DatesRead = append(book.DatesRead, strings.TrimSpace(dateElem.Text))
			})
		case "date_added":
			book.DateAdded = fieldElem.ChildText("div.value")
		}
	})

	if book.BookId != "" {
		return book, nil
	}
	return book, fmt.Errorf("Failed to scrape the book")
}

func scrapeQuotes(url string, options ScrapeOptions) ([]Quote, error) {
	quoteCollector := colly.NewCollector(
		defaultCollectorOptions(options),
	)

	quotes := make([]Quote, 0, 100)

	quoteCollector.OnError(func(r *colly.Response, err error) {
		logg.Errorf("Error when collecting quote at %v\n%v", r.Request.URL, err)
	})

	var bookId string
	var authorId string
	quoteCollector.OnHTML("a.bookTitle", func(h *colly.HTMLElement) {
		bookId = parseId(h.Attr("href"))
		quoteCollector.OnHTMLDetach("a.bookTitle")
	})
	quoteCollector.OnHTML("a.authorName", func(h *colly.HTMLElement) {
		authorId = parseId(h.Attr("href"))
		quoteCollector.OnHTMLDetach("a.authorName")
	})

	quoteCollector.OnHTML("div.quote", func(quoteElem *colly.HTMLElement) {
		// brokeEarly := false
		// quotesElem.ForEachWithBreak("div.quote", func(_ int, quoteElem *colly.HTMLElement) bool {
		quote, err := scrapeQuote(quoteElem)
		if err == nil {
			if quote.Likes >= minQuoteLikes {
				quote.BookId = bookId
				quote.AuthorId = authorId
				quotes = append(quotes, quote)
				// return false
				// logg.Printf("Quote: %+v", quote)
			}
		} else {
			logg.Error(err)
		}
		// brokeEarly = true
		// return true
		// })
		quoteCollector.OnHTMLDetach("a.next_page")
		quoteCollector.OnHTMLDetach("div.quote")
	})

	quoteCollector.OnHTML("a.next_page", tryVisitNextPage)

	if err := quoteCollector.Visit(url); err != nil {
		return quotes, err
	}
	return quotes, nil
}

func scrapeQuote(quoteElem *colly.HTMLElement) (Quote, error) {
	var quote Quote

	quote.Text = quoteElem.ChildText("div.quoteText")
	endChar := "―"
	lastIndex := strings.LastIndex(quote.Text, endChar)
	if lastIndex < 0 {
		return quote, fmt.Errorf("Unable to find end char in quote")
	}
	quote.Text = strings.TrimSpace(quote.Text[:lastIndex])

	var rightElem *colly.HTMLElement = nil
	quoteElem.ForEachWithBreak("div.right", func(_ int, h *colly.HTMLElement) bool {
		rightElem = h
		return true
	})

	likeText := strings.TrimSpace(rightElem.Text)
	likeText, _ = strings.CutSuffix(likeText, "likes")
	likeText = strings.TrimSpace(likeText)
	likes, err := strconv.ParseInt(likeText, 10, 32) // Yes there are negative likes
	if err != nil {
		return quote, fmt.Errorf("Failed to parse likes: %v", err)
	} else {
		quote.Likes = uint(likes)
	}

	quote.QuoteId = parseId(rightElem.ChildAttr("a", "href"))

	if quote.QuoteId != "" {
		return quote, nil
	}
	return quote, fmt.Errorf("Failed to scrape the quote")
}
