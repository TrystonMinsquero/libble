package main

import (
	"errors"
	"fmt"
	"os"
	"sync"

	. "libble/shared"

	"github.com/bwmarrin/snowflake"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormLog "gorm.io/gorm/logger"
)

const saveDir = "saves/"
const dbPath = "saves/libble.db"

var (
	db        *gorm.DB
	userLocks sync.Map // LibbleID -> *sync.Mutex
)

// InitDB initializes the SQLite database with GORM
func InitDB() error {
	if err := os.MkdirAll(saveDir, os.ModePerm); err != nil {
		return err
	}

	var err error
	db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: gormLog.Default.LogMode(gormLog.Info),
	})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Get underlying SQL DB for configuration
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying SQL DB: %w", err)
	}

	// SQLite-specific configuration
	db.Exec("PRAGMA foreign_keys = ON")
	db.Exec("PRAGMA journal_mode = WAL")
	db.Exec("PRAGMA busy_timeout = 5000")

	// Set connection pool settings
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	// Auto-migrate tables
	err = db.AutoMigrate(
		&User{},
		&Book{},
		&Quote{},
		&UserBook{},
	)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	logg.Info("Database initialized successfully with GORM")
	return nil
}

// getUserLock returns a mutex for the given user ID to ensure concurrent safety
func getUserLock(userID UserID) *sync.Mutex {
	lock, _ := userLocks.LoadOrStore(userID, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

// CreateUser creates a new user in the database
func CreateUser(userID UserID, userGRID string, settings UserSettings) error {
	player := User{
		ID:         userID,
		UserGRID:   userGRID,
		Settings:   settings,
		SeenQuotes: []QuoteID{},
		Games:      []Game{},
	}

	result := db.Create(&player)
	if result.Error != nil {
		return fmt.Errorf("failed to insert user: %w", result.Error)
	}

	return nil
}

func UpdateUserCreateState(user User, state UserCreateState) error {
	result := db.Model(&user).Where("user_id = ?", user.ID).UpdateColumn("state", state)
	return result.Error
}

func GetUser(userID UserID) (User, error) {
	var user User
	result := db.Where("user_id = ?", userID).Find(&user)
	if result.Error != nil {
		return user, result.Error
	}
	return user, nil
}

// GetUsersByGRID returns summary info for all users with a given Goodreads user ID
func GetUsersByGRID(userGRID string) ([]UserSummary, error) {
	var players []User
	result := db.Where("user_gr_id = ?", userGRID).Find(&players)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to query users: %w", result.Error)
	}

	if len(players) == 0 {
		return nil, fmt.Errorf("no users found")
	}

	if len(players) > 20 {
		logg.Warn("User GRID '%s' has a lot (%d) of users", userGRID, len(players))
	}

	summaries := make([]UserSummary, len(players))
	for i, player := range players {
		summaries[i] = UserSummary{
			LibbleID:  player.ID,
			GameCount: len(player.Games),
		}

		// Find most recent game date
		if len(player.Games) > 0 {
			mostRecent := player.Games[0].Date
			for _, game := range player.Games {
				if game.Date.After(mostRecent) {
					mostRecent = game.Date
				}
			}
			summaries[i].LastPlayed = mostRecent.Format("2006-01-02")
		}
	}

	return summaries, nil
}

// LoadPlayer loads a player from the database
func LoadPlayer(userID UserID) (User, error) {
	var player User

	result := db.Where("user_id = ?", userID).First(&player)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return player, fmt.Errorf("player not found")
		}
		return player, fmt.Errorf("failed to query player: %w", result.Error)
	}

	return player, nil
}

// UpdatePlayer updates a player in the database
func UpdatePlayer(player User) error {
	result := db.Save(&player)
	if result.Error != nil {
		return fmt.Errorf("failed to update player: %w", result.Error)
	}

	return nil
}

// LoadUserBooks loads just the books for a user (without quotes)
func LoadUserBooks(userID UserID) ([]UserBook, error) {
	var userBooks []UserBook

	result := db.Preload("Book").Where("user_id = ?", userID).Find(&userBooks)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to query books: %w", result.Error)
	}

	return userBooks, nil
}

// LoadSaveData loads a complete SaveData structure for a user
func LoadSaveData(userID UserID) (App, error) {
	var data App

	// Load player
	player, err := LoadPlayer(userID)
	if err != nil {
		return data, err
	}
	data.User = player

	// Load user's books with preloading
	data.Books, err = LoadUserBooks(userID)
	if err != nil {
		return data, err
	}

	// Load quotes for user's books
	if len(data.Books) > 0 {
		bookIDs := make([]BookID, len(data.Books))
		for i, ub := range data.Books {
			bookIDs[i] = ub.Book.ID
		}

		result := db.Where("book_id IN ?", bookIDs).Find(&data.Quotes)
		if result.Error != nil {
			return data, fmt.Errorf("failed to query quotes: %w", result.Error)
		}

		// Populate BookGRID for each quote (it's a computed field, not stored in DB)
		bookGRIDMap := make(map[BookID]string)
		for _, ub := range data.Books {
			bookGRIDMap[ub.Book.ID] = ub.Book.BookGRID
		}
		for i := range data.Quotes {
			data.Quotes[i].BookGRID = bookGRIDMap[data.Quotes[i].BookID]
		}
	}

	data.PopulateLookups()
	return data, nil
}

// SaveBooks inserts or updates books in the database and returns a map of GRID -> BookId
func SaveBooks(books []UserBook, userID UserID) (map[string]BookID, error) {
	gridToID := make(map[string]BookID)

	err := db.Transaction(func(tx *gorm.DB) error {
		for _, ub := range books {
			// Insert or get existing book
			result := tx.Where("book_gr_id = ?", ub.Book.BookGRID).FirstOrCreate(&ub.Book)
			if result.Error != nil {
				return fmt.Errorf("failed to upsert book: %w", result.Error)
			}

			gridToID[ub.Book.BookGRID] = ub.Book.ID

			// Save user_book relationship
			ub.UserID = userID
			ub.BookID = ub.Book.ID
			result = tx.Save(&ub)
			if result.Error != nil {
				return fmt.Errorf("failed to insert user_book: %w", result.Error)
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return gridToID, nil
}

// SaveQuotes inserts or updates quotes in the database and returns a map of GRID -> QuoteId
func SaveQuotes(quotes []Quote) (map[string]QuoteID, error) {
	gridToID := make(map[string]QuoteID)

	err := db.Transaction(func(tx *gorm.DB) error {
		for _, q := range quotes {
			// Insert or get existing quote
			result := tx.Where("quote_gr_id = ?", q.QuoteGRID).FirstOrCreate(&q)
			if result.Error != nil {
				return fmt.Errorf("failed to insert quote: %w", result.Error)
			}

			gridToID[q.QuoteGRID] = q.ID
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return gridToID, nil
}

// GetOrCreateBookID returns the book ID for a given GRID, creating it if needed
func GetOrCreateBookID(bookGRID string, node *snowflake.Node) (BookID, error) {
	var book Book
	result := db.Where("book_gr_id = ?", bookGRID).First(&book)

	if result.Error == nil {
		return book.ID, nil
	}

	if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return 0, fmt.Errorf("failed to query book: %w", result.Error)
	}

	// Book doesn't exist, return a new ID (will be inserted later during SaveBooks)
	return BookID(node.Generate().Int64()), nil
}

// GetOrCreateQuoteID returns the quote ID for a given GRID, creating it if needed
func GetOrCreateQuoteID(quoteGRID string, node *snowflake.Node) (QuoteID, error) {
	var quote Quote
	result := db.Where("quote_gr_id = ?", quoteGRID).First(&quote)

	if result.Error == nil {
		return quote.ID, nil
	}

	if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return 0, fmt.Errorf("failed to query quote: %w", result.Error)
	}

	// Quote doesn't exist, return a new ID (will be inserted later during SaveQuotes)
	return QuoteID(node.Generate().Int64()), nil
}
