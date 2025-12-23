package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"

	. "libble/shared"

	"github.com/bwmarrin/snowflake"
	_ "modernc.org/sqlite"
)

const dbPath = "saves/libble.db"

var (
	db        *sql.DB
	userLocks sync.Map // DBID -> *sync.Mutex
)

// InitDB initializes the SQLite database and creates tables if needed
func InitDB() error {
	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Create tables
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		libble_id INTEGER PRIMARY KEY,
		user_gr_id TEXT NOT NULL,
		settings TEXT NOT NULL,
		seen_quote_ids TEXT NOT NULL,
		games TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS books (
		book_id INTEGER PRIMARY KEY,
		book_gr_id TEXT UNIQUE NOT NULL,
		title TEXT NOT NULL,
		author TEXT NOT NULL,
		author_gr_id TEXT NOT NULL,
		avg_rating REAL NOT NULL,
		rating_count INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS user_books (
		user_id INTEGER NOT NULL,
		book_id INTEGER NOT NULL,
		stars INTEGER NOT NULL,
		dates_read TEXT NOT NULL,
		date_added TEXT NOT NULL,
		PRIMARY KEY (user_id, book_id),
		FOREIGN KEY (user_id) REFERENCES users(libble_id),
		FOREIGN KEY (book_id) REFERENCES books(book_id)
	);

	CREATE TABLE IF NOT EXISTS quotes (
		quote_id INTEGER PRIMARY KEY,
		quote_gr_id TEXT UNIQUE NOT NULL,
		book_id INTEGER NOT NULL,
		text TEXT NOT NULL,
		likes INTEGER NOT NULL,
		FOREIGN KEY (book_id) REFERENCES books(book_id)
	);

	CREATE INDEX IF NOT EXISTS idx_user_gr_id ON users(user_gr_id);
	CREATE INDEX IF NOT EXISTS idx_book_gr_id ON books(book_gr_id);
	CREATE INDEX IF NOT EXISTS idx_quote_gr_id ON quotes(quote_gr_id);
	CREATE INDEX IF NOT EXISTS idx_quotes_book_id ON quotes(book_id);
	CREATE INDEX IF NOT EXISTS idx_user_books_user_id ON user_books(user_id);
	`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	logg.Info("Database initialized successfully")
	return nil
}

// getUserLock returns a mutex for the given user ID to ensure concurrent safety
func getUserLock(userID DBID) *sync.Mutex {
	lock, _ := userLocks.LoadOrStore(userID, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

// CreateUser creates a new user in the database
func CreateUser(libbleID DBID, userGRID string, settings PlayerSettings) error {
	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	seenQuotesJSON, err := json.Marshal([]QuoteId{})
	if err != nil {
		return fmt.Errorf("failed to marshal seen quotes: %w", err)
	}

	gamesJSON, err := json.Marshal([]Game{})
	if err != nil {
		return fmt.Errorf("failed to marshal games: %w", err)
	}

	_, err = db.Exec(`
		INSERT INTO users (libble_id, user_gr_id, settings, seen_quote_ids, games)
		VALUES (?, ?, ?, ?, ?)
	`, libbleID, userGRID, string(settingsJSON), string(seenQuotesJSON), string(gamesJSON))

	if err != nil {
		return fmt.Errorf("failed to insert user: %w", err)
	}

	return nil
}

// GetUsersByGRID returns summary info for all users with a given Goodreads user ID
func GetUsersByGRID(userGRID string) ([]UserSummary, error) {
	rows, err := db.Query("SELECT libble_id, games FROM users WHERE user_gr_id = ?", userGRID)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var summaries []UserSummary
	for rows.Next() {
		var id DBID
		var gamesJSON string
		if err := rows.Scan(&id, &gamesJSON); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}

		// Parse games to get count and last played date
		var games []Game
		if err := json.Unmarshal([]byte(gamesJSON), &games); err != nil {
			return nil, fmt.Errorf("failed to unmarshal games: %w", err)
		}

		summary := UserSummary{
			LibbleID:  id,
			GameCount: len(games),
		}

		// Find most recent game date
		if len(games) > 0 {
			mostRecent := games[0].Date
			for _, game := range games {
				if game.Date.After(mostRecent) {
					mostRecent = game.Date
				}
			}
			summary.LastPlayed = mostRecent.Format("2006-01-02")
		}

		summaries = append(summaries, summary)
	}

	if len(summaries) == 0 {
		return nil, fmt.Errorf("no users found")
	}

	return summaries, nil
}

// LoadPlayer loads a player from the database
func LoadPlayer(libbleID DBID) (Player, error) {
	var player Player
	var settingsJSON, seenQuotesJSON, gamesJSON string

	err := db.QueryRow(`
		SELECT libble_id, user_gr_id, settings, seen_quote_ids, games
		FROM users WHERE libble_id = ?
	`, libbleID).Scan(&player.ID, &player.UserGRID, &settingsJSON, &seenQuotesJSON, &gamesJSON)

	if err == sql.ErrNoRows {
		return player, fmt.Errorf("player not found")
	}
	if err != nil {
		return player, fmt.Errorf("failed to query player: %w", err)
	}

	if err := json.Unmarshal([]byte(settingsJSON), &player.Settings); err != nil {
		return player, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	if err := json.Unmarshal([]byte(seenQuotesJSON), &player.SeenQuotes); err != nil {
		return player, fmt.Errorf("failed to unmarshal seen quotes: %w", err)
	}

	if err := json.Unmarshal([]byte(gamesJSON), &player.Games); err != nil {
		return player, fmt.Errorf("failed to unmarshal games: %w", err)
	}

	return player, nil
}

// UpdatePlayer updates a player in the database
func UpdatePlayer(player Player) error {
	settingsJSON, err := json.Marshal(player.Settings)
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	seenQuotesJSON, err := json.Marshal(player.SeenQuotes)
	if err != nil {
		return fmt.Errorf("failed to marshal seen quotes: %w", err)
	}

	gamesJSON, err := json.Marshal(player.Games)
	if err != nil {
		return fmt.Errorf("failed to marshal games: %w", err)
	}

	_, err = db.Exec(`
		UPDATE users
		SET settings = ?, seen_quote_ids = ?, games = ?
		WHERE libble_id = ?
	`, string(settingsJSON), string(seenQuotesJSON), string(gamesJSON), player.ID)

	if err != nil {
		return fmt.Errorf("failed to update player: %w", err)
	}

	return nil
}

// LoadUserBooks loads just the books for a user (without quotes)
func LoadUserBooks(libbleID DBID) ([]UserBook, error) {
	rows, err := db.Query(`
		SELECT b.book_id, b.book_gr_id, b.title, b.author, b.author_gr_id,
		       b.avg_rating, b.rating_count, ub.stars, ub.dates_read, ub.date_added
		FROM books b
		JOIN user_books ub ON b.book_id = ub.book_id
		WHERE ub.user_id = ?
	`, libbleID)
	if err != nil {
		return nil, fmt.Errorf("failed to query books: %w", err)
	}
	defer rows.Close()

	var books []UserBook
	for rows.Next() {
		var ub UserBook
		var datesReadJSON string
		err := rows.Scan(
			&ub.Book.BookId, &ub.Book.BookGRID, &ub.Book.Title, &ub.Book.Author,
			&ub.Book.AuthorGRID, &ub.Book.AvgRating, &ub.Book.RatingCount,
			&ub.UserData.Stars, &datesReadJSON, &ub.UserData.DateAdded,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan book: %w", err)
		}

		if err := json.Unmarshal([]byte(datesReadJSON), &ub.UserData.DatesRead); err != nil {
			return nil, fmt.Errorf("failed to unmarshal dates read: %w", err)
		}

		books = append(books, ub)
	}

	return books, nil
}

// LoadSaveData loads a complete SaveData structure for a user
func LoadSaveData(libbleID DBID) (SaveData, error) {
	var data SaveData

	// Load player
	player, err := LoadPlayer(libbleID)
	if err != nil {
		return data, err
	}
	data.Player = player

	// Load user's books
	rows, err := db.Query(`
		SELECT b.book_id, b.book_gr_id, b.title, b.author, b.author_gr_id,
		       b.avg_rating, b.rating_count, ub.stars, ub.dates_read, ub.date_added
		FROM books b
		JOIN user_books ub ON b.book_id = ub.book_id
		WHERE ub.user_id = ?
	`, libbleID)
	if err != nil {
		return data, fmt.Errorf("failed to query books: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var ub UserBook
		var datesReadJSON string
		err := rows.Scan(
			&ub.Book.BookId, &ub.Book.BookGRID, &ub.Book.Title, &ub.Book.Author,
			&ub.Book.AuthorGRID, &ub.Book.AvgRating, &ub.Book.RatingCount,
			&ub.UserData.Stars, &datesReadJSON, &ub.UserData.DateAdded,
		)
		if err != nil {
			return data, fmt.Errorf("failed to scan book: %w", err)
		}

		if err := json.Unmarshal([]byte(datesReadJSON), &ub.UserData.DatesRead); err != nil {
			return data, fmt.Errorf("failed to unmarshal dates read: %w", err)
		}

		data.Books = append(data.Books, ub)
	}

	// Load quotes for user's books
	if len(data.Books) > 0 {
		bookIDs := make([]interface{}, len(data.Books))
		placeholders := ""
		for i, ub := range data.Books {
			bookIDs[i] = ub.Book.BookId
			if i > 0 {
				placeholders += ","
			}
			placeholders += "?"
		}

		query := fmt.Sprintf(`
			SELECT quote_id, quote_gr_id, book_id, text, likes
			FROM quotes WHERE book_id IN (%s)
		`, placeholders)

		rows, err := db.Query(query, bookIDs...)
		if err != nil {
			return data, fmt.Errorf("failed to query quotes: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var q Quote
			err := rows.Scan(&q.QuoteId, &q.QuoteGRID, &q.BookId, &q.Text, &q.Likes)
			if err != nil {
				return data, fmt.Errorf("failed to scan quote: %w", err)
			}

			// Find the book GRID for this quote
			for _, ub := range data.Books {
				if ub.Book.BookId == q.BookId {
					q.BookGRID = ub.Book.BookGRID
					break
				}
			}

			data.Quotes = append(data.Quotes, q)
		}
	}

	data.PopulateLookups()
	return data, nil
}

// SaveBooks inserts or updates books in the database and returns a map of GRID -> BookId
func SaveBooks(books []UserBook, userID DBID) (map[string]BookId, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	gridToID := make(map[string]BookId)

	for _, ub := range books {
		// Insert or ignore book
		_, err := tx.Exec(`
			INSERT OR IGNORE INTO books (book_id, book_gr_id, title, author, author_gr_id, avg_rating, rating_count)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, ub.Book.BookId, ub.Book.BookGRID, ub.Book.Title, ub.Book.Author,
			ub.Book.AuthorGRID, ub.Book.AvgRating, ub.Book.RatingCount)
		if err != nil {
			return nil, fmt.Errorf("failed to insert book: %w", err)
		}

		// Get the book_id (in case it already existed)
		var bookID BookId
		err = tx.QueryRow("SELECT book_id FROM books WHERE book_gr_id = ?", ub.Book.BookGRID).Scan(&bookID)
		if err != nil {
			return nil, fmt.Errorf("failed to get book_id: %w", err)
		}
		gridToID[ub.Book.BookGRID] = bookID

		// Insert or replace user_book
		datesReadJSON, err := json.Marshal(ub.UserData.DatesRead)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal dates read: %w", err)
		}

		_, err = tx.Exec(`
			INSERT OR REPLACE INTO user_books (user_id, book_id, stars, dates_read, date_added)
			VALUES (?, ?, ?, ?, ?)
		`, userID, bookID, ub.UserData.Stars, string(datesReadJSON), ub.UserData.DateAdded)
		if err != nil {
			return nil, fmt.Errorf("failed to insert user_book: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return gridToID, nil
}

// SaveQuotes inserts or updates quotes in the database and returns a map of GRID -> QuoteId
func SaveQuotes(quotes []Quote) (map[string]QuoteId, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	gridToID := make(map[string]QuoteId)

	for _, q := range quotes {
		// Insert or ignore quote
		_, err := tx.Exec(`
			INSERT OR IGNORE INTO quotes (quote_id, quote_gr_id, book_id, text, likes)
			VALUES (?, ?, ?, ?, ?)
		`, q.QuoteId, q.QuoteGRID, q.BookId, q.Text, q.Likes)
		if err != nil {
			return nil, fmt.Errorf("failed to insert quote: %w", err)
		}

		// Get the quote_id (in case it already existed)
		var quoteID QuoteId
		err = tx.QueryRow("SELECT quote_id FROM quotes WHERE quote_gr_id = ?", q.QuoteGRID).Scan(&quoteID)
		if err != nil {
			return nil, fmt.Errorf("failed to get quote_id: %w", err)
		}
		gridToID[q.QuoteGRID] = quoteID
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return gridToID, nil
}

// GetOrCreateBookID returns the book ID for a given GRID, creating it if needed
func GetOrCreateBookID(bookGRID string, node *snowflake.Node) (BookId, error) {
	var bookID BookId
	err := db.QueryRow("SELECT book_id FROM books WHERE book_gr_id = ?", bookGRID).Scan(&bookID)
	if err == nil {
		return bookID, nil
	}
	if err != sql.ErrNoRows {
		return 0, fmt.Errorf("failed to query book: %w", err)
	}

	// Book doesn't exist, return a new ID (will be inserted later during SaveBooks)
	return BookId(node.Generate().Int64()), nil
}

// GetOrCreateQuoteID returns the quote ID for a given GRID, creating it if needed
func GetOrCreateQuoteID(quoteGRID string, node *snowflake.Node) (QuoteId, error) {
	var quoteID QuoteId
	err := db.QueryRow("SELECT quote_id FROM quotes WHERE quote_gr_id = ?", quoteGRID).Scan(&quoteID)
	if err == nil {
		return quoteID, nil
	}
	if err != sql.ErrNoRows {
		return 0, fmt.Errorf("failed to query quote: %w", err)
	}

	// Quote doesn't exist, return a new ID (will be inserted later during SaveQuotes)
	return QuoteId(node.Generate().Int64()), nil
}
