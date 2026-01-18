package main

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	. "libble/shared"

	"github.com/sahilm/fuzzy"
	dom "honnef.co/go/js/dom/v2"
)

func initGame() {
	debugPrint("Starting game...")

	sendToStart := func() {
		removeData(saveKeyID)
		handlePage()
	}

	// Get libble ID
	libbleIDStr := loadLibbleID()

	var app App

	// Try loading Player from LocalStorage
	if err := loadJson(saveKeyPlayer, &app.User); err != nil {
		debugPrint("Player not in LocalStorage, fetching from server...")
		type PlayerResponse struct {
			Player User `json:"player"`
		}
		var resp PlayerResponse
		if err := fetch("/game/player/"+libbleIDStr, &resp); err != nil {
			log(err, "Failed to fetch player from server")
			sendToStart()
			return
		}
		app.User = resp.Player
		debugPrint("Loaded player from server")
	} else {
		debugPrint("Loaded player from LocalStorage")
	}

	// Try loading Books from LocalStorage
	if err := loadJson(saveKeyBooks, &app.Books); err != nil || len(app.Books) == 0 {
		debugPrint("Books not in LocalStorage, fetching from server...")
		type BooksResponse struct {
			Books []UserBook `json:"books"`
		}
		var resp BooksResponse
		if err := fetch("/game/user-books/"+libbleIDStr, &resp); err != nil {
			log(err, "Failed to fetch books from server")
			sendToStart()
			return
		}
		app.Books = resp.Books
		debugPrint("Loaded %d books from server", len(app.Books))
	} else {
		debugPrint("Loaded %d books from LocalStorage", len(app.Books))
	}

	dailyQuoteID := QuoteID(NilID)
	// Try loading Quotes from LocalStorage
	if err := loadJson(saveKeyQuotes, &app.Quotes); err != nil || len(app.Quotes) <= 0 {
		fetchedQuote, err := fetchDailyQuote(&app)
		if err != nil {
			log(err, "Failed to fetch daily quote")
			sendToStart()
			return
		}

		dailyQuoteID = fetchedQuote
		app.NeedsServer = true
		debugPrint("Loaded today's quote from server")
	} else {
		debugPrint("Loaded %d quotes from LocalStorage", len(app.Quotes))
	}
	app.PopulateLookups()

	if dailyQuoteID == NilID {
		debugPrint("Picking Daily Quote")
		quoteID, err := app.PickDailyQuote()
		log(err, "Error when picking daily quote")
		if quoteID == NilID {
			sendToStart()
			return
		}
		dailyQuoteID = quoteID
	}

	debugPrint("Daily quote is %d", dailyQuoteID)

	if _, err := app.User.InitTodaysGame(app, dailyQuoteID); err != nil {
		log(err, "Failed initializing today's game")
		sendToStart()
		return
	}

	debugPrint("Today's book is %s", app.User.TodaysGame().Book.Book.CleanTitle())

	debugPrint("Setting update autocomplete")

	// convert book map to slice
	bookCount := len(app.Books)
	allBooks := make([]Book, 0, bookCount)
	for _, book := range app.Books {
		if !app.User.Settings.ScrapeOptions.ShouldScrapeQuotes(book) {
			continue
		}
		allBooks = append(allBooks, book.Book)
	}
	debugPrint("Using %d/%d books (from scrape options)", len(allBooks), len(app.Books))

	setupHTML(&app, allBooks)
}

func pickDailyQuote(data *App) (QuoteID, error) {
	if !data.NeedsServer {
		return data.PickDailyQuote()
	}
	return fetchDailyQuote(data)
}

func fetchDailyQuote(app *App) (QuoteID, error) {
	type DailyGameResponse struct {
		Quote Quote    `json:"quote"`
		Book  UserBook `json:"book"`
	}
	var dailyResp DailyGameResponse
	var dailyQuote QuoteID
	libbleID := loadLibbleID()
	if err := fetch("/game/daily/"+libbleID, &dailyResp); err != nil {
		return dailyQuote, err
	}
	app.AddQuote(dailyResp.Quote)
	return dailyResp.Quote.ID, nil
}

func setupHTML(app *App, allBooks Books) {
	doc := dom.GetWindow().Document()

	game := app.User.TodaysGame()

	defer func() {
		if r := recover(); r != nil {
			logErr("Recovered from panic setting up html:\n%v", r)
		}
	}()

	quoteElement := getElemByID(doc, "quote")
	if quoteElement != nil {
		quoteElement.SetTextContent(game.Quote.Text)
	}

	input := getElemByIDAs[*dom.HTMLInputElement](doc, "title")
	suggestions := getElemByIDAs[dom.HTMLElement](doc, "titleSuggestions")
	guessForm := getElemByID(doc, "guessForm")

	feedback := getElemByIDAs[dom.HTMLElement](doc, "feedbackBox")
	setFeedback := func(msg string, status string) {
		if feedback != nil {
			setFeedbackElem(feedback, msg, status)
		}
	}

	gameInputs := doc.GetElementsByClassName("game-input")
	submitBtn := getElemByIDAs[*dom.HTMLButtonElement](doc, "submitBtn")
	shareBtn := getElemByIDAs[*dom.HTMLButtonElement](doc, "shareBtn")
	gameProgress := getElemByIDAs[dom.HTMLElement](doc, "gameProgress")

	hintSection := getElemByIDAs[dom.HTMLElement](doc, "hintSection")
	skipBtn := getElemByIDAs[*dom.HTMLButtonElement](doc, "skipBtn")

	hints := createHints(hintSection)
	updateGameProgress := func() {
		if gameProgress == nil {
			return
		}
		var sb strings.Builder
		addEmojiStrip(game, &sb)
		gameProgress.SetTextContent(sb.String())
		// TODO: show emoji strip above book title if it starts wrapping
	}

	updateInputStates := func() {
		defer func() {
			if r := recover(); r != nil {
				logErr("Failed to update inputs")
			}
		}()
		completed := game.Completed()
		disabled := completed
		for _, e := range gameInputs {
			e.Underlying().Set("disabled", disabled)
		}

		setVisible(hintSection, !completed)

		started := game.Started()
		setVisible(skipBtn, !completed && !started)
		setEnabled(skipBtn, !completed && !started)
		for _, hint := range hints {
			if game.UsedHint(hint.kind) {
				hint.elem.Class().Add(usedHintClass)
			}
			show := !completed && started && hint.canUse(*game)
			setVisible(hint.elem, show)
			setEnabled(hint.elem, show)
		}

		updateGameProgress()

		setVisible(submitBtn, !completed)
		setVisible(shareBtn, completed)
	}

	handleRevist := func() bool {
		if !game.Completed() {
			setVisible(submitBtn, true)
			setVisible(hintSection, true)
			return true
		}
		if game.Won() {
			setFeedback("Congrats! You've already won for today,\ncome back tomorrow to play again.", FBStatusSuccess)
		} else {
			setFeedback("Looks like you didn't get it this time :(\nCome back tomorrow and try again!", FBStatusError)
		}
		input.SetPlaceholder(game.Book.Book.CleanTitle())

		setVisible(submitBtn, false)
		setVisible(hintSection, false)
		setVisible(shareBtn, true)
		shareBtn.SetDisabled(false)
		return false
	}

	handleRevist()
	updateInputStates()

	// setup submit
	guessForm.AddEventListener("submit", false, func(e dom.Event) {
		e.PreventDefault()
		if handleRevist() {
			go func() {
				onSubmit(input, app, setFeedback)
				updateInputStates()
			}()
		}
	})

	// setup skip button
	skipBtn.AddEventListener("click", false, func(e dom.Event) {
		e.PreventDefault()
		if game.Attempts() <= 0 && handleRevist() {
			// Run in goroutine to avoid blocking the event loop
			go func() {
				err := onSkip(app, setFeedback)
				log(err, "Failed skipping current quote")

				quoteElement.SetTextContent(game.Quote.Text)
				updateInputStates()
			}()
		}
	})

	// setup hints
	for _, hint := range hints {
		if hint.elem == nil {
			continue
		}
		hint.elem.AddEventListener("click", false, func(e dom.Event) {
			debugPrint("Used hint: " + string(hint.kind))
			e.PreventDefault()
			if handleRevist() {
				go func() {
					msg := hint.use(game)
					if msg != "" {
						setFeedback(msg, FBStatusHint)
					}
					updateInputStates()
					saveErr := saveNonStaticData(*app)
					log(saveErr, "Failed saving data after using hint")
				}()
			}

		})
	}

	// setup share button
	shareBtn.AddEventListener("click", false, func(e dom.Event) {
		e.PreventDefault()
		if game.Completed() {
			results := generateResultsString(game)
			err := shareText(results)
			if err == nil {
				return
			}

			err = copyToClipboard(results)
			prevText := shareBtn.TextContent()
			if err == nil {
				shareBtn.SetTextContent("📋 Copied!")
			} else {
				log(err, "Failed to share")
				shareBtn.SetTextContent("Sorry, failed to copy")
			}
			shareBtn.SetDisabled(true)

			go func() {
				time.Sleep(time.Second * 3)
				shareBtn.SetTextContent(prevText)
				shareBtn.SetDisabled(false)
			}()
		} else {
			logErr("How did you click this?")
		}
	})

	setupAutocomplete(input, suggestions, allBooks)

	// Setup user icon
	setupUserIcon()
}

const (
	// Feedback statuses
	FBStatusSuccess = "success"
	FBStatusError   = "error"
	FBStatusWarn    = "warning"
	FBStatusHint    = "hint"
)

func setFeedbackElem(e dom.HTMLElement, message string, status string) {
	if e == nil {
		return
	}

	e.SetTextContent(message)
	e.Class().SetString("feedback " + status)
}

func onSubmit(
	input *dom.HTMLInputElement,
	app *App,
	setFeedback func(msg string, status string),
) bool {

	query := strings.ToLower(strings.TrimSpace(input.Value()))
	game := app.User.TodaysGame()
	target := strings.ToLower(game.Book.Book.CleanTitle())

	defer saveNonStaticData(*app)

	if query == target {
		game.Guesses = append(game.Guesses, game.Quote.BookID)
		attempts := len(game.Guesses)
		s := ""
		if attempts > 1 {
			s = "s"
		}
		message := fmt.Sprintf("Correct! You got it in %d attempt%s", attempts, s)
		setFeedback(message, FBStatusSuccess)
		return true
	} else if bookID := app.FindBookID(query); bookID != NilID {
		if slices.Contains(game.Guesses, bookID) {
			setFeedback("You already tried that guess!", FBStatusWarn)
		} else {
			game.Guesses = append(game.Guesses, bookID)

			if len(game.Guesses) >= game.Settings.MaxGuesses {
				msg := fmt.Sprintf("Failed! The answer was \"%s\"", game.Book.Book.CleanTitle())
				setFeedback(msg, FBStatusError)
				return true
			} else {
				msg := fmt.Sprintf("Nope! Try again (%d attempts remaining)",
					game.Settings.MaxGuesses-len(game.Guesses))
				setFeedback(msg, FBStatusError)
			}
		}
	} else {
		setFeedback("That book is not in your library!", FBStatusWarn)
	}
	return false
}

func onSkip(
	app *App,
	setFeedback func(msg string, status string),
) error {
	game := app.User.TodaysGame()

	if game.Started() {
		return fmt.Errorf("Trying to skip after the game already started")
	}

	// Mark quote as seen so it won't appear again
	if !slices.Contains(app.User.SeenQuotes, game.QuoteID) {
		app.User.SeenQuotes = append(app.User.SeenQuotes, game.QuoteID)
		debugPrint("Skipping quote %d", game.QuoteID)
		if app.NeedsServer {
			syncPlayer(app.User) // Need to sync since pickDailyQuote will fetch data from server
		}
	}

	msg := fmt.Sprintf("Skipped! The answer was \"%s\"", game.Book.Book.CleanTitle())
	setFeedback(msg, FBStatusError)

	dailyQuoteID, err := pickDailyQuote(app)
	if dailyQuoteID == NilID {
		return fmt.Errorf("Failed to repick daily quote when skipping:\n%v", err)
	}
	log(err, "Issue when to repicking daily quote when skipping")
	game.QuoteID = dailyQuoteID
	game.Date = time.Now()
	saveNonStaticData(*app)
	if err := game.Init(*app); err != nil {
		return err
	}
	debugPrint("The book is now %s", game.Book.Book.CleanTitle())
	return nil
}

func addEmojiStrip(game *Game, sb *strings.Builder) {
	hintIndex := 0
	for i := 0; i < game.Settings.MaxGuesses; i++ {
		for hintIndex < len(game.Hints) {
			hint := game.Hints[hintIndex]
			if hint.GuessIndex != i {
				break
			}

			sb.WriteString(hintEmoji(hint.Kind))
			sb.WriteRune(' ')
			hintIndex++
		}

		if i < len(game.Guesses) {
			if i == len(game.Guesses)-1 && game.Won() {
				sb.WriteRune('🟩')
			} else {
				sb.WriteRune('🟥')
			}
		} else {
			sb.WriteRune('⬜')
		}
		if i < game.Settings.MaxGuesses-1 {
			sb.WriteRune(' ')
		}
	}
}

func generateResultsString(game *Game) string {
	// Create shareable text
	var shareText strings.Builder
	shareText.WriteString("📖 Libble - ")

	// Format date
	dateStr := game.Date.Format("Jan 2 2006")
	shareText.WriteString(dateStr)

	shareText.WriteRune('\n')
	addEmojiStrip(game, &shareText)

	shareText.WriteString("\n\n")
	shareText.WriteString(game.Quote.Text)

	// Copy to clipboard
	return shareText.String()
}

// TODO: Remove the end (...) on quotes
// TODO: Also search by author
// TODO: highlight indexes that matched the text
func setupAutocomplete(
	input *dom.HTMLInputElement,
	suggestionsParent dom.HTMLElement,
	allBooks Books /* available books */) {

	doc := dom.GetWindow().Document()

	type Suggestion struct {
		bookIndex  int
		titleMatch fuzzy.Match
	}

	suggestions := make([]Suggestion, 0, len(allBooks))
	currentSelection := 0
	const maxVisibleSuggestions = 8

	getBookTitle := func(suggestionIndex int) string {
		if suggestionIndex >= len(suggestions) {
			logErr("Trying to get suggestion index %d but only have %d",
				suggestionIndex, len(suggestions))
			return ""
		}
		suggestion := suggestions[suggestionIndex]
		if suggestion.bookIndex >= len(allBooks) {
			logErr("Book index %d is out of bounds of %d. How???", suggestion.bookIndex, len(allBooks))
			return ""
		}
		return allBooks[suggestion.bookIndex].CleanTitle()
	}

	updateSuggestions := func() {}

	resetSuggestions := func() {
		currentSelection = 0
		suggestions = suggestions[:0]
		updateSuggestions()
	}

	useSelection := func() {
		input.SetValue(getBookTitle(currentSelection))
		resetSuggestions()
	}

	setSelection := func(selection int) {
		currentSelection = selection
		updateSuggestions()
		input.SetValue(getBookTitle(currentSelection))
	}

	updateSuggestions = func() {
		suggestionsParent.SetInnerHTML("")

		setDisplay := func(val string) {
			suggestionsParent.Style().SetProperty("display", val, "important")
		}

		if len(suggestions) == 0 {
			setDisplay("none")
			return
		}

		for i, suggestion := range suggestions {
			li := doc.CreateElement("li")

			book := allBooks[suggestion.bookIndex]

			li.SetTextContent(book.CleanTitle())
			if i == currentSelection {
				li.Class().Add("selected")
			}
			li.AddEventListener("click", false, func(e dom.Event) {
				setSelection(i)
				useSelection()
			})
			suggestionsParent.AppendChild(li)
			if i >= maxVisibleSuggestions {
				break
			}
		}
		setDisplay("block")
	}

	input.AddEventListener("input", false, func(e dom.Event) {
		query := strings.TrimSpace(input.Value())

		matches := fuzzy.FindFrom(query, allBooks)
		count := min(len(matches), int(80))
		suggestions = suggestions[:0]
		currentSelection = 0
		for i := range count {
			match := matches[i]
			suggestions = append(suggestions, Suggestion{
				bookIndex:  match.Index,
				titleMatch: match,
			})
		}

		// Find matching books
		updateSuggestions()
	})

	input.AddEventListener("keydown", false, func(e dom.Event) {
		if suggestionsParent.InnerHTML() == "" {
			return
		}
		keyEvent := e.(*dom.KeyboardEvent)
		key := keyEvent.Key()
		// TODO: figure out how to make it select the entire text on single click

		switch key {
		// TODO: figure out how to "scroll" the suggestions so it moves the selection
		// if it goes out of being visible
		case "ArrowDown":
			e.PreventDefault()
			setSelection((currentSelection + 1) % len(suggestions))
		case "ArrowUp":
			e.PreventDefault()
			if currentSelection == 0 {
				setSelection(maxVisibleSuggestions - 1)
			} else {
				setSelection(currentSelection - 1)
			}
		case "Enter":
			e.PreventDefault()
			useSelection()
		case "Tab":
			e.PreventDefault()
			input.SetValue(getBookTitle(currentSelection))
		case "Escape":
			resetSuggestions()
		}
	})

	// Hide suggestions when clicking outside
	doc.AddEventListener("click", false, func(e dom.Event) {
		target := e.Target()
		if target != input && target != suggestionsParent {
			resetSuggestions()
		}
	})
}

// func fuzzyScore(query, ) {
// 	if (text.startsWith(query)) {
// 		return 1.0;
// 	}
//
// 	// Check if any word in text starts with query
// 	const words = text.split(/\s+/);
// 	for (let word of words) {
// 		if (word.startsWith(query)) {
// 			return 0.9;
// 		}
// 	}
//
// 	// Check if query is contained in text
// 	if (text.includes(query)) {
// 		return 0.8;
// 	}
// }

func LevenshteinDistance(s, t string) int {
	r1, r2 := []rune(s), []rune(t)
	column := make([]int, 1, 64)

	for y := 1; y <= len(r1); y++ {
		column = append(column, y)
	}

	for x := 1; x <= len(r2); x++ {
		column[0] = x

		for y, lastDiag := 1, x-1; y <= len(r1); y++ {
			oldDiag := column[y]
			cost := 0
			if r1[y-1] != r2[x-1] {
				cost = 1
			}
			column[y] = min(column[y]+1, column[y-1]+1, lastDiag+cost)
			lastDiag = oldDiag
		}
	}
	return column[len(r1)]
}

func LevenshteinDistanceNorm(s1, s2 string) float64 {
	distance := LevenshteinDistance(s1, s2)
	maxLength := math.Max(float64(len(s1)), float64(len(s2)))

	if maxLength == 0 { // Handle case where both strings are empty
		return 0.0
	}
	return float64(distance) / maxLength
}

type Books []Book

func (b Books) String(i int) string {
	if i >= 0 && i < len(b) {
		return b[i].CleanTitle()
	}
	logErr("Fuzzy search is trying to use index %d", i)
	return ""
}

func (b Books) Len() int {
	return len(b)
}

// func findMatchingBooks(query string, book []Book) []Suggestion {
// 	if query == "" {
// 		return nil
// 	}
//
// 	fuzzy.FindFromNoSort(query, data.Books)
// 	// matches := make([]Match, 0, len(data.Books))
// 	//
// 	// for _, userBook := range data.Books {
// 	// 	book := userBook.Book
// 	//
// 	// 	title := book.CleanTitle()
// 	// 	if strings.ToLower(query) != query { // only ignore case if the user used uppercase
// 	// 		title = strings.ToLower(title)
// 	// 	}
// 	//
// 	// 	score := LevenshteinDistance(query, title)
// 	// 	logg.Debugf("%s has score %d with %s\n", title, score, query)
// 	// 	if score > 0 {
// 	// 		matches = append(matches, Match{
// 	// 			book:  book,
// 	// 			score: score,
// 	// 		})
// 	// 	}
// 	// }
// 	// slices.SortFunc(matches, func(a Match, b Match) int {
// 	// 	return b.score - a.score
// 	// })
//
// 	return matches
// }
