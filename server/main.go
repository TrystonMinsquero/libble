package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"path"
	"strconv"

	"compress/gzip"
	. "libble/shared"

	"github.com/charmbracelet/log"
	ginzip "github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

var (
	logg = log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: false,
	})
)

const saveDir = "saves/"

func main() {

	// Run in release mode by default
	if ginMode := os.Getenv(gin.EnvGinMode); ginMode != "" {
		gin.SetMode(ginMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	isDebug := gin.Mode() == gin.DebugMode

	// Create a Gin router with default middleware (logger and recovery)
	r := gin.Default()

	r.Use(ginzip.Gzip(ginzip.DefaultCompression))

	r.SetTrustedProxies(nil)

	// Host the site as well when debugging
	if isDebug {
		const siteDir = "./site"
		if entries, err := os.ReadDir(siteDir); err == nil {
			for _, entry := range entries {
				name := entry.Name()
				if entry.IsDir() {
					r.Static("/"+name, path.Join(siteDir, name))
				} else {
					r.StaticFile(name, path.Join(siteDir, name))
				}
			}
		} else {
			logg.Errorf("Failed reading from %s:\n%v", siteDir, err)
		}
	}

	if err := os.MkdirAll(saveDir, os.ModePerm); err != nil {
		logg.Errorf("Failed making save dir: %v", err)
	}

	options := ScrapeOptions{
		cache: isDebug,
	}

	r.POST("/user/:GRID", func(c *gin.Context) {
		userGRID := c.Param("GRID")
		if userGRID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Must provide user param"})
			return
		}

		// TODO: Maybe limit to 3 per user?

		books, quotes, err := scrapeGoodreads(userGRID, options)
		if err != nil {
			errorMsg := fmt.Sprintf("Error scraping goodreads with id %s: %v", userGRID, err)
			c.JSON(http.StatusFailedDependency, gin.H{"error": errorMsg})
			return
		}

		saveData := createUserData(userGRID, books, quotes)
		c.JSON(http.StatusOK, saveData)
	})

	// TODO: update save data? I could just use the scrape request
	// r.POST("/update/:id", func(c *gin.Context) {}

	r.GET("/update/:id", func(c *gin.Context) {
		userIDParam := c.Param("id")
		if userIDParam == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Must provide user id param"})
			return
		}

		userID, err := strconv.ParseUint(userIDParam, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Must provide valid user id param"})
			return
		}

		saveData, err := loadUserData(DBID(userID))
		if err != nil {
			errMsg := fmt.Sprintf("Failed loading user data: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}

		userGRID := saveData.Player.UserGRID
		_, _, err = scrapeGoodreads(userGRID, options)
		if err != nil {
			errorMsg := fmt.Sprintf("Error scraping goodreads with id %s: %v", userGRID, err)
			c.JSON(http.StatusFailedDependency, gin.H{"error": errorMsg})
			return
		}
	})

	r.GET("/scrape/:id", func(c *gin.Context) {
		userGRID := c.Param("id")
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

	logg.Fatal(r.Run())
}

func saveFileName(userID DBID) string {
	return strconv.FormatUint(uint64(userID), 10)
}

func saveUserData(save SaveData) error {
	fileName := saveFileName(save.Player.ID)
	file, err := os.OpenFile(path.Join(saveDir, fileName), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("Failed opening save file: %v", err)
	}

	saveBytes, err := json.Marshal(save)
	if err != nil {
		return fmt.Errorf("Failed marshelling save data: %v", err)
	}

	// Compress saveBytes to compressedBuffer
	var compressedBuffer bytes.Buffer
	compresser := gzip.NewWriter(&compressedBuffer)
	defer compresser.Close()
	if _, err := compresser.Write(saveBytes); err != nil {
		return err
	}
	if err := compresser.Close(); err != nil {
		return err
	}

	compressedBytes := compressedBuffer.Bytes()
	written, err := file.Write(compressedBytes)
	if err != nil {
		return fmt.Errorf("Failed writing save data: %v", err)
	}

	compressPercent := float32(len(compressedBytes)) / float32(len(saveBytes))
	logg.Infof("Saved %d bytes (%.2f%% of original) of data for %s", written, compressPercent, fileName)
	return nil
}

func loadUserData(userID DBID) (SaveData, error) {
	var data SaveData
	fileName := saveFileName(userID)
	file, err := os.Open(path.Join(saveDir, fileName))
	if err != nil {
		return data, fmt.Errorf("Failed opening save file: %v", err)
	}
	defer file.Close()

	// Decompress the file
	decompresser, err := gzip.NewReader(file)
	if err != nil {
		return data, fmt.Errorf("Failed creating gzip reader: %v", err)
	}
	defer decompresser.Close()

	// Decode JSON from decompressed data
	decoder := json.NewDecoder(decompresser)
	if err := decoder.Decode(&data); err != nil {
		return data, fmt.Errorf("Failed decoding save data: %v", err)
	}

	return data, nil
}

func createUserData(userGRID string, books []UserBook, quotes []Quote) SaveData {
	var data SaveData
	data.Player.UserGRID = userGRID
	data.Player.ID = DBID(rand.Uint64())

	// Initialize maps
	data.Books = make(map[BookId]UserBook)
	data.Quotes = make(map[QuoteId]Quote)

	bookGRIDtoID := make(map[string]BookId)

	// Populate books map
	for _, book := range books {
		bookID := BookId(rand.Uint64())
		if _, exists := data.Books[bookID]; exists {
			logg.Errorf("Generated unique book id that wasn't unique")
		}
		exists := func() bool {
			_, found := data.Books[bookID]
			return found
		}
		for exists() {
			logg.Errorf("Generated unique quote id that wasn't unique")
			bookID = BookId(rand.Uint64())
		}

		data.Books[bookID] = book
		bookGRIDtoID[book.Book.BookGRID] = bookID
	}

	// Populate quotes map
	for _, quote := range quotes {
		quoteID := QuoteId(rand.Uint64())

		exists := func() bool {
			_, found := data.Quotes[quoteID]
			return found
		}
		for exists() {
			logg.Errorf("Generated unique quote id that wasn't unique")
			quoteID = QuoteId(rand.Uint64())
		}

		if quote.BookGRID != "" {
			if bookID, found := bookGRIDtoID[quote.BookGRID]; found {
				quote.BookId = bookID
			} else {
				logg.Errorf("BookGRID %s exists on quote %d but wasn't found", quote.BookGRID, quoteID)
			}
		}

		data.Quotes[quoteID] = quote
	}

	// Initialize empty slices
	data.Player.SeenQuotes = []QuoteId{}
	data.Player.Games = []Game{}

	if err := saveUserData(data); err != nil {
		logg.Errorf("Unabled to save new user data: %v", err)
	}

	return data
}
