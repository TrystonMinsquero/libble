package main

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"
	"time"

	. "libble/shared"

	"github.com/sahilm/fuzzy"
	dom "honnef.co/go/js/dom/v2"
)

func saveKey(jsonName string) string {
	return "libble." + jsonName
}

type FieldPredicate func(string) bool

func saveAllDataFiltered(data SaveData, filter FieldPredicate) error {
	v := reflect.ValueOf(data)
	t := v.Type()
	var err error
	err = nil
	for i := range t.NumField() {
		field := t.Field(i)
		jsonName := field.Tag.Get("json")
		if jsonName != "" && filter(jsonName) {
			err = errors.Join(err, saveJson(saveKey(jsonName), v.Field(i).Interface()))
		}
	}
	return err
}

func saveAllData(data SaveData) error {
	return saveAllDataFiltered(data, func(s string) bool { return true })
}

func saveNonStaticData(data SaveData) error {
	return saveAllDataFiltered(data, func(s string) bool { return !IsStaticSaveDataField(s) })
}

func loadAllData(data *SaveData) error {
	pv := reflect.ValueOf(data)
	v := pv.Elem()
	t := v.Type()
	var err error
	err = nil
	for i := range t.NumField() {
		fieldType := t.Field(i)
		jsonName := fieldType.Tag.Get("json")
		field := v.Field(i).Addr().Interface()
		if jsonName != "" {
			err = errors.Join(err, loadJson(saveKey(jsonName), field))
		}
	}
	return err
}

func initGame() {
	fmt.Println("Starting game...")
	var data SaveData
	// Load save data from local storage
	if err := loadAllData(&data); err != nil {
		log(err, "Failed loading data when starting game")
	}

	if _, err := initTodaysGame(&data); err != nil {
		log(err, "Failed initializing today's game")
	}

	fmt.Println("Setting update autocomplete")

	// convert book map to slice
	bookCount := len(data.Books)
	allBooks := make([]Book, 0, bookCount)
	for _, book := range data.Books {
		allBooks = append(allBooks, book.Book)
	}

	setupHTML(&data, allBooks)
}
func toDate(t time.Time) time.Time {
	return t.Truncate(24 * time.Hour)
}

func todaysDate() time.Time {
	return time.Now().Truncate(24 * time.Hour)
}

func todaysGame(data *SaveData) *Game {
	player := &data.Player
	if len(player.Games) > 0 {
		lastGame := &player.Games[len(player.Games)-1]
		if todaysDate().Equal(toDate(lastGame.Date)) {
			return lastGame
		}
	}
	return nil
}

func initTodaysGame(data *SaveData) (game *Game, err error) {
	if game = todaysGame(data); game != nil {
		err = game.Init(*data)
		return game, err
	}

	dailyQuoteId, err := data.PickDailyQuote()
	if err != nil {
		return game, fmt.Errorf("Failed to pick daily quote when making new game:\n%v", err)
	}

	player := &data.Player
	player.Games = append(player.Games, Game{
		QuoteID: dailyQuoteId,
		Date:    todaysDate(),
		Guesses: make([]BookId, 0),
	})
	game = &player.Games[len(player.Games)-1]
	err = game.Init(*data)
	return game, err
}

func setupHTML(data *SaveData, allBooks Books) {
	doc := dom.GetWindow().Document()

	game := todaysGame(data)

	defer func() {
		if r := recover(); r != nil {
			logErr(fmt.Sprintf("Recovered from panic setting up html:\n%v", r))
		}
	}()

	quoteElement := doc.GetElementByID("quote")
	if quoteElement != nil {
		quoteElement.SetTextContent(game.Quote.Text)
	}

	input := doc.GetElementByID("title").(*dom.HTMLInputElement)
	suggestions := doc.GetElementByID("titleSuggestions").(dom.HTMLElement)
	guessForm := doc.GetElementByID("guessForm")

	feedback, feedbackOk := doc.GetElementByID("feedbackBox").(dom.HTMLElement)
	if !feedbackOk {
		logErr("Failed to get html element with id 'feedbackBox'")
	}
	setFeedback := func(msg string, status string) {
		if feedbackOk {
			setFeedbackElem(feedback, msg, status)
		}
	}

	statusBox, statusBoxOk := doc.GetElementByID("statusBox").(dom.HTMLElement)
	if !statusBoxOk {
		logErr("Failed to get html element with id 'statusBox'")
	}
	setStatus := func(msg string, status string) {
		if statusBoxOk {
			setFeedbackElem(statusBox, msg, status)
			setFeedback("", "")
		}
	}

	inputs := doc.GetElementsByClassName("game-input")
	setInputsEnabled := func(enabled bool) {
		defer func() {
			if r := recover(); r != nil {
				logErr("Failed to disable inputs")
			}
		}()
		for _, e := range inputs {
			e.Underlying().Set("disabled", !enabled)
		}
	}

	if game.Attempts() > 0 {
		if game.AttemptsLeft() <= 0 {
			feedback.SetTextContent("You al")

		}
		msg := fmt.Sprintf("Welcome back! You have %d guesses remaining", game.AttemptsLeft())
		setStatus(msg, "")
	}

	handleRevist := func() bool {
		if !game.Completed() {
			return true
		}
		if game.Won() {
			setStatus("Congrats! You've already won for today, \ncome back tomorrow to play again.", SuccessFBStatus)
		} else {
			setStatus("Looks like you didn't get it this time :(\nCome back tomorrow and try again!", "")
		}
		return false
	}
	ongoing := handleRevist()
	setInputsEnabled(ongoing)

	// setup submit
	guessForm.AddEventListener("submit", false, func(e dom.Event) {
		e.PreventDefault()
		if handleRevist() {
			completed := onSubmit(input, data, setFeedback)
			setInputsEnabled(!completed)
		}
	})

	setupAutocomplete(input, suggestions, allBooks)
}

const (
	// Feedback statuses
	SuccessFBStatus = "successs"
	ErrorFBStatus   = "error"
	WarnFBStatus    = "warning"
)

func setFeedbackElem(e dom.HTMLElement, message string, status string) {
	emoji := ""
	switch status {
	case ErrorFBStatus:
		emoji = "❌"
	case SuccessFBStatus:
		emoji = "🎉"
	case WarnFBStatus:
		emoji = "⚠️"
	}

	if emoji != "" {
		e.SetTextContent(emoji + " " + message)
	} else {
		e.SetTextContent(message)
	}
	e.Class().SetString("feedback " + status)
}

func onSubmit(
	input *dom.HTMLInputElement,
	data *SaveData,
	setFeedback func(msg string, status string),
) bool {

	query := strings.ToLower(strings.TrimSpace(input.Value()))
	game := todaysGame(data)
	target := strings.ToLower(game.Book.Book.CleanTitle())

	defer saveNonStaticData(*data)

	if query == target {
		game.Guesses = append(game.Guesses, game.Quote.BookId)
		attempts := len(game.Guesses)
		s := ""
		if attempts > 1 {
			s = "s"
		}
		message := fmt.Sprintf("Correct! You got it in %d attempt%s", attempts, s)
		setFeedback(message, SuccessFBStatus)
		return true
	} else if bookId := data.FindBookId(query); bookId != NilID {
		if slices.Contains(game.Guesses, bookId) {
			setFeedback("You already tried that guess!", WarnFBStatus)
		} else {
			game.Guesses = append(game.Guesses, bookId)

			if len(game.Guesses) == MaxGuesses {
				msg := fmt.Sprintf("Failed! The answer was \"%s\"", game.Book.Book.CleanTitle())
				setFeedback(msg, ErrorFBStatus)
				return true
			} else {
				msg := fmt.Sprintf("Nope! Try again (%d attempts remaining)",
					MaxGuesses-len(game.Guesses))
				setFeedback(msg, ErrorFBStatus)
			}
		}
	} else {
		setFeedback("That book is not in your library!", WarnFBStatus)
	}
	return false
}

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

	getBook := func(suggestionIndex int) Book {
		suggestion := suggestions[suggestionIndex]
		return allBooks[suggestion.bookIndex]
	}

	updateSuggestions := func() {}

	resetSuggestions := func() {
		currentSelection = 0
		suggestions = suggestions[:0]
		updateSuggestions()
	}

	useSelection := func() {
		input.SetValue(getBook(currentSelection).CleanTitle())
		resetSuggestions()
	}

	setSelection := func(selection int) {
		currentSelection = selection
		updateSuggestions()
		input.SetValue(getBook(currentSelection).CleanTitle())
	}

	updateSuggestions = func() {
		suggestionsParent.SetInnerHTML("")

		setDisplay := func(val string) {
			suggestionsParent.Style().SetProperty("display", val, "important")
		}

		fmt.Printf("Updating suggestions\n")
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
		fmt.Println("Input callback")

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

		switch key {
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
			// TODO: submit game
		case "Tab":
			e.PreventDefault()
			input.SetValue(getBook(currentSelection).CleanTitle())
		case "Escape":
			resetSuggestions()
		}
		fmt.Println("Current:", currentSelection)
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
	logErr(fmt.Sprintf("Fuzzy search is trying to use index %d", i))
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
// 	// 	fmt.Printf("%s has score %d with %s\n", title, score, query)
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
