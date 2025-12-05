package main

import (
	"fmt"
	"math"
	"strings"

	. "libble/shared"

	"github.com/sahilm/fuzzy"
	dom "honnef.co/go/js/dom/v2"
)

func initGame() {
	fmt.Println("Starting game...")
	var data SaveData
	// Load save data from local storage
	if err := loadJson("saveData", &data); err != nil {
		log(err, "Failed loading data when starting game")
	}

	// Pick the daily quote
	quoteId, err := data.PickDailyQuote()
	if err != nil {
		log(err, "Failed to pick daily quote")
		return
	}

	// Get the quote from the map
	quote, found := data.Quotes[quoteId]
	if !found {
		logErr("Daily quote not found in quotes map")
		return
	}
	dailyQuote := quote

	// Update the DOM with the quote
	doc := dom.GetWindow().Document()
	quoteElement := doc.GetElementByID("quote")
	if quoteElement != nil {
		quoteElement.SetTextContent(dailyQuote.Text)
	}

	fmt.Println("Setting update autocomplete")

	bookCount := len(data.Books)
	allBooks := make([]Book, 0, bookCount)
	for _, book := range data.Books {
		allBooks = append(allBooks, book.Book)
	}

	// Set up autocomplete for title input
	setupAutocomplete("title", "titleSuggestions", allBooks)
}

func setupAutocomplete(inputID string, suggestionsID string, allBooks Books /* available books */) {
	doc := dom.GetWindow().Document()

	input := doc.GetElementByID(inputID).(*dom.HTMLInputElement)
	suggestionsParent := doc.GetElementByID(suggestionsID).(dom.HTMLElement)

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

		// TODO: Make this
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

	fmt.Printf("Autocomplete setup for %s %s", inputID, suggestionsID)
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
	return b[i].CleanTitle()
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
