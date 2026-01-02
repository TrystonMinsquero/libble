//go:build !sqlite

package main

import (
	"context"
	"encoding/json"
	"fmt"

	. "libble/shared"

	"github.com/bwmarrin/snowflake"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresDB implements the Database interface using PostgreSQL
type PostgresDB struct {
	pool *pgxpool.Pool
}

// NewDatabase creates a new PostgreSQL database instance
func NewDatabase() (Database, error) {
	config := LoadDBConfig()

	if !config.IsConfigured() {
		return nil, fmt.Errorf("PostgreSQL configuration required: DATABASE_URL or DB_HOST must be set")
	}

	poolConfig, err := pgxpool.ParseConfig(config.ConnectionURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	// Configure connection pool
	poolConfig.MaxConns = config.MaxConns
	poolConfig.MinConns = config.MinConns

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Create schema
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		libble_id BIGINT PRIMARY KEY,
		user_gr_id VARCHAR(255) NOT NULL,
		settings JSONB NOT NULL,
		seen_quote_ids JSONB NOT NULL,
		games JSONB NOT NULL
	);

	CREATE TABLE IF NOT EXISTS books (
		book_id BIGINT PRIMARY KEY,
		book_gr_id VARCHAR(255) UNIQUE NOT NULL,
		title TEXT NOT NULL,
		author VARCHAR(500) NOT NULL,
		author_gr_id VARCHAR(255) NOT NULL,
		avg_rating DOUBLE PRECISION NOT NULL,
		rating_count INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS user_books (
		user_id BIGINT NOT NULL,
		book_id BIGINT NOT NULL,
		stars SMALLINT NOT NULL,
		dates_read JSONB NOT NULL,
		date_added VARCHAR(50) NOT NULL,
		PRIMARY KEY (user_id, book_id),
		FOREIGN KEY (user_id) REFERENCES users(libble_id) ON DELETE CASCADE,
		FOREIGN KEY (book_id) REFERENCES books(book_id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS quotes (
		quote_id BIGINT PRIMARY KEY,
		quote_gr_id VARCHAR(255) UNIQUE NOT NULL,
		book_id BIGINT NOT NULL,
		text TEXT NOT NULL,
		likes INTEGER NOT NULL,
		FOREIGN KEY (book_id) REFERENCES books(book_id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_user_gr_id ON users(user_gr_id);
	CREATE INDEX IF NOT EXISTS idx_book_gr_id ON books(book_gr_id);
	CREATE INDEX IF NOT EXISTS idx_quote_gr_id ON quotes(quote_gr_id);
	CREATE INDEX IF NOT EXISTS idx_quotes_book_id ON quotes(book_id);
	CREATE INDEX IF NOT EXISTS idx_user_books_user_id ON user_books(user_id);
	`

	if _, err := pool.Exec(context.Background(), schema); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	return &PostgresDB{pool: pool}, nil
}

// Close closes the database connection pool
func (p *PostgresDB) Close() error {
	p.pool.Close()
	return nil
}

// CreateUser creates a new user in the database
func (p *PostgresDB) CreateUser(libbleID DBID, userGRID string, settings PlayerSettings) error {
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

	_, err = p.pool.Exec(context.Background(), `
		INSERT INTO users (libble_id, user_gr_id, settings, seen_quote_ids, games)
		VALUES ($1, $2, $3, $4, $5)
	`, libbleID, userGRID, string(settingsJSON), string(seenQuotesJSON), string(gamesJSON))

	if err != nil {
		return fmt.Errorf("failed to insert user: %w", err)
	}

	return nil
}

// GetUsersByGRID returns summary info for all users with a given Goodreads user ID
func (p *PostgresDB) GetUsersByGRID(userGRID string) ([]UserSummary, error) {
	rows, err := p.pool.Query(context.Background(), "SELECT libble_id, games FROM users WHERE user_gr_id = $1", userGRID)
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
func (p *PostgresDB) LoadPlayer(libbleID DBID) (Player, error) {
	var player Player
	var settingsJSON, seenQuotesJSON, gamesJSON string

	err := p.pool.QueryRow(context.Background(), `
		SELECT libble_id, user_gr_id, settings, seen_quote_ids, games
		FROM users WHERE libble_id = $1
	`, libbleID).Scan(&player.ID, &player.UserGRID, &settingsJSON, &seenQuotesJSON, &gamesJSON)

	if err == pgx.ErrNoRows {
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
func (p *PostgresDB) UpdatePlayer(player Player) error {
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

	_, err = p.pool.Exec(context.Background(), `
		UPDATE users
		SET settings = $1, seen_quote_ids = $2, games = $3
		WHERE libble_id = $4
	`, string(settingsJSON), string(seenQuotesJSON), string(gamesJSON), player.ID)

	if err != nil {
		return fmt.Errorf("failed to update player: %w", err)
	}

	return nil
}

// LoadUserBooks loads just the books for a user (without quotes)
func (p *PostgresDB) LoadUserBooks(libbleID DBID) ([]UserBook, error) {
	rows, err := p.pool.Query(context.Background(), `
		SELECT b.book_id, b.book_gr_id, b.title, b.author, b.author_gr_id,
		       b.avg_rating, b.rating_count, ub.stars, ub.dates_read, ub.date_added
		FROM books b
		JOIN user_books ub ON b.book_id = ub.book_id
		WHERE ub.user_id = $1
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
func (p *PostgresDB) LoadSaveData(libbleID DBID) (SaveData, error) {
	var data SaveData

	// Load player
	player, err := p.LoadPlayer(libbleID)
	if err != nil {
		return data, err
	}
	data.Player = player

	// Load user's books
	rows, err := p.pool.Query(context.Background(), `
		SELECT b.book_id, b.book_gr_id, b.title, b.author, b.author_gr_id,
		       b.avg_rating, b.rating_count, ub.stars, ub.dates_read, ub.date_added
		FROM books b
		JOIN user_books ub ON b.book_id = ub.book_id
		WHERE ub.user_id = $1
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
		// Convert book IDs to int64 array for PostgreSQL
		bookIDs := make([]int64, len(data.Books))
		for i, ub := range data.Books {
			bookIDs[i] = int64(ub.Book.BookId)
		}

		// Use PostgreSQL's ANY operator with array
		rows, err := p.pool.Query(context.Background(), `
			SELECT quote_id, quote_gr_id, book_id, text, likes
			FROM quotes WHERE book_id = ANY($1)
		`, bookIDs)
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
func (p *PostgresDB) SaveBooks(books []UserBook, userID DBID) (map[string]BookId, error) {
	ctx := context.Background()
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	gridToID := make(map[string]BookId)

	for _, ub := range books {
		// Insert or ignore book (ON CONFLICT DO NOTHING)
		_, err := tx.Exec(ctx, `
			INSERT INTO books (book_id, book_gr_id, title, author, author_gr_id, avg_rating, rating_count)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (book_id) DO NOTHING
		`, ub.Book.BookId, ub.Book.BookGRID, ub.Book.Title, ub.Book.Author,
			ub.Book.AuthorGRID, ub.Book.AvgRating, ub.Book.RatingCount)
		if err != nil {
			return nil, fmt.Errorf("failed to insert book: %w", err)
		}

		// Get the book_id (in case it already existed)
		var bookID BookId
		err = tx.QueryRow(ctx, "SELECT book_id FROM books WHERE book_gr_id = $1", ub.Book.BookGRID).Scan(&bookID)
		if err != nil {
			return nil, fmt.Errorf("failed to get book_id: %w", err)
		}
		gridToID[ub.Book.BookGRID] = bookID

		// Insert or update user_book (ON CONFLICT DO UPDATE)
		datesReadJSON, err := json.Marshal(ub.UserData.DatesRead)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal dates read: %w", err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO user_books (user_id, book_id, stars, dates_read, date_added)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (user_id, book_id) DO UPDATE SET
				stars = EXCLUDED.stars,
				dates_read = EXCLUDED.dates_read,
				date_added = EXCLUDED.date_added
		`, userID, bookID, ub.UserData.Stars, string(datesReadJSON), ub.UserData.DateAdded)
		if err != nil {
			return nil, fmt.Errorf("failed to insert user_book: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return gridToID, nil
}

// SaveQuotes inserts or updates quotes in the database and returns a map of GRID -> QuoteId
func (p *PostgresDB) SaveQuotes(quotes []Quote) (map[string]QuoteId, error) {
	ctx := context.Background()
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	gridToID := make(map[string]QuoteId)

	for _, q := range quotes {
		// Insert or ignore quote (ON CONFLICT DO NOTHING)
		_, err := tx.Exec(ctx, `
			INSERT INTO quotes (quote_id, quote_gr_id, book_id, text, likes)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (quote_id) DO NOTHING
		`, q.QuoteId, q.QuoteGRID, q.BookId, q.Text, q.Likes)
		if err != nil {
			return nil, fmt.Errorf("failed to insert quote: %w", err)
		}

		// Get the quote_id (in case it already existed)
		var quoteID QuoteId
		err = tx.QueryRow(ctx, "SELECT quote_id FROM quotes WHERE quote_gr_id = $1", q.QuoteGRID).Scan(&quoteID)
		if err != nil {
			return nil, fmt.Errorf("failed to get quote_id: %w", err)
		}
		gridToID[q.QuoteGRID] = quoteID
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return gridToID, nil
}

// GetOrCreateBookID returns the book ID for a given GRID, creating it if needed
func (p *PostgresDB) GetOrCreateBookID(bookGRID string, node *snowflake.Node) (BookId, error) {
	var bookID BookId
	err := p.pool.QueryRow(context.Background(), "SELECT book_id FROM books WHERE book_gr_id = $1", bookGRID).Scan(&bookID)
	if err == nil {
		return bookID, nil
	}
	if err != pgx.ErrNoRows {
		return 0, fmt.Errorf("failed to query book: %w", err)
	}

	// Book doesn't exist, return a new ID (will be inserted later during SaveBooks)
	return BookId(node.Generate().Int64()), nil
}

// GetOrCreateQuoteID returns the quote ID for a given GRID, creating it if needed
func (p *PostgresDB) GetOrCreateQuoteID(quoteGRID string, node *snowflake.Node) (QuoteId, error) {
	var quoteID QuoteId
	err := p.pool.QueryRow(context.Background(), "SELECT quote_id FROM quotes WHERE quote_gr_id = $1", quoteGRID).Scan(&quoteID)
	if err == nil {
		return quoteID, nil
	}
	if err != pgx.ErrNoRows {
		return 0, fmt.Errorf("failed to query quote: %w", err)
	}

	// Quote doesn't exist, return a new ID (will be inserted later during SaveQuotes)
	return QuoteId(node.Generate().Int64()), nil
}
