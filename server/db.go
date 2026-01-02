package main

import (
	"fmt"
	"sync"

	. "libble/shared"

	"github.com/bwmarrin/snowflake"
)

// Database interface defines all database operations
type Database interface {
	// Lifecycle
	Close() error

	// User operations
	CreateUser(libbleID DBID, userGRID string, settings PlayerSettings) error
	GetUsersByGRID(userGRID string) ([]UserSummary, error)
	LoadPlayer(libbleID DBID) (Player, error)
	UpdatePlayer(player Player) error

	// Book/Quote operations
	LoadUserBooks(libbleID DBID) ([]UserBook, error)
	LoadSaveData(libbleID DBID) (SaveData, error)
	SaveBooks(books []UserBook, userID DBID) (map[string]BookId, error)
	SaveQuotes(quotes []Quote) (map[string]QuoteId, error)

	// ID lookups
	GetOrCreateBookID(bookGRID string, node *snowflake.Node) (BookId, error)
	GetOrCreateQuoteID(quoteGRID string, node *snowflake.Node) (QuoteId, error)
}

var (
	db        Database   // Global database instance (implementation determined at build time)
	userLocks sync.Map   // Per-user mutexes for concurrent safety (DBID -> *sync.Mutex)
)

// InitDB initializes the database (implementation determined at build time via build tags)
func InitDB() error {
	var err error
	db, err = NewDatabase() // Build tags determine which NewDatabase() is called

	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	logg.Info("Database initialized successfully")
	return nil
}

// getUserLock returns a mutex for the given user ID to ensure concurrent safety
func getUserLock(userID DBID) *sync.Mutex {
	lock, _ := userLocks.LoadOrStore(userID, &sync.Mutex{})
	return lock.(*sync.Mutex)
}
