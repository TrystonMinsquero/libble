package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"strconv"
	"sync"

	"compress/gzip"
	. "libble/shared"

	"github.com/bwmarrin/snowflake"
	"github.com/charmbracelet/log"
	"github.com/gin-contrib/cors"
	ginzip "github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

const saveDir = "saves/"

var isDebug = os.Getenv(gin.EnvGinMode) == gin.DebugMode
var logg = logger()

func main() {

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

	// Host the site as well when debugging
	if true {
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

	if err := os.MkdirAll(saveDir, os.ModePerm); err != nil {
		logg.Errorf("Failed making save dir: %v", err)
	}

	// I really need to setup some kind of database

	var GRIDtoID sync.Map
	node, err := snowflake.NewNode(1)
	if err != nil {
		logg.Fatal(err)
	}

	r.GET("/scrape/gr/user-books/:GRID", func(c *gin.Context) {
		userGRID := c.Param("GRID")
		if userGRID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Must provide GRID param (goodreads user ID)"})
			return
		}

		// TODO: try to reads options from req body
		options := DefaultScrapeOptions()
		books, err := scrapeBooks(userGRID, options)

		// resolve ids
		for index := range books {
			ub := &books[index]
			id, _ := GRIDtoID.LoadOrStore(ub.Book.BookGRID, node.Generate().Int64())
			ub.Book.BookId = BookId(id.(int64))
		}

		res := gin.H{
			"books": books,
		}
		if err != nil {
			res["error"] = fmt.Sprintf("Failed to scrape books from user %s: \n%v", userGRID, err)
		}

		c.JSON(http.StatusOK, res)
	})

	r.GET("/scrape/gr/quotes/:GRID", func(c *gin.Context) {
		userGRID := c.Param("GRID")
		if userGRID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Must provide GRID param (goodreads book ID)"})
			return
		}

		// TODO: try to reads options from req body
		options := DefaultScrapeOptions()
		quotes, err := scrapeQuotes(userGRID, options)

		for index := range quotes {
			quote := &quotes[index]
			bookId, loaded := GRIDtoID.LoadOrStore(quote.BookGRID, node.Generate().Int64())
			if !loaded {
				logg.Warnf("Quote %s has book grid %s but it's not in the id map", quote.QuoteGRID, quote.BookGRID)
			}
			quote.BookId = BookId(bookId.(int64))
			quoteId, _ := GRIDtoID.LoadOrStore(quote.QuoteGRID, node.Generate().Int64())
			quote.QuoteId = QuoteId(quoteId.(int64))
		}

		res := gin.H{
			"quotes": quotes,
		}
		if err != nil {
			res["error"] = fmt.Sprintf("Failed to scrape books from user %s: \n%v", userGRID, err)
		}

		c.JSON(http.StatusOK, res)
	})

	logg.Fatal(r.Run())
}

func logger() *log.Logger {
	level := log.InfoLevel
	if isDebug {
		level = log.DebugLevel
	}

	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: false,
		Level:           level,
	})
	return logger
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
	data := NewSaveData(userGRID, books, quotes)
	logg.Debugf("Created new user %d from GRID '%s'", data.Player.ID, userGRID)

	if err := saveUserData(data); err != nil {
		logg.Errorf("Unabled to save new user data: %v", err)
	}

	return data
}
