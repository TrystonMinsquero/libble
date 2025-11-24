package main

import (
	"fmt"
	"slices"
	// "slices"
	"strings"

	. "libble/shared"

	dom "honnef.co/go/js/dom/v2"
)

var data SaveData
var dailyQuote Quote

func initGame() {
	fmt.Println("Starting game...")
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
	dailyQuote = quote

	// Update the DOM with the quote
	doc := dom.GetWindow().Document()
	quoteElement := doc.GetElementByID("quote")
	if quoteElement != nil {
		quoteElement.SetTextContent(dailyQuote.Text)
	}

	fmt.Println("Setting update autocomplete")
	// Set up autocomplete for title input
	setupAutocomplete("title", "titleSuggestions")
}

func setupAutocomplete(inputID string, suggestionsID string) {
	doc := dom.GetWindow().Document()

	input := doc.GetElementByID(inputID).(*dom.HTMLInputElement)
	suggestions := doc.GetElementByID(suggestionsID).(dom.HTMLElement)
	currentSelection := 0

	selectMatch := func(match Match) {
		input.SetValue(match.book.CleanTitle())
		currentSelection = 0
	}

	updateSuggestions := func(matches []Match) {
		suggestions.SetInnerHTML("")

		setDisplay := func(val string) {
			suggestions.Style().SetProperty("display", val, "important")
		}

		fmt.Printf("Updating suggestions\n")
		if len(matches) == 0 {
			setDisplay("none")
			return
		}

		for i, match := range matches {
			li := doc.CreateElement("li")

			li.SetTextContent(match.book.CleanTitle())
			if i == currentSelection {
				li.Class().Add("selected")
			}
			li.AddEventListener("click", false, func(e dom.Event) {
				selectMatch(match)
			})
			suggestions.AppendChild(li)
			if i > 8 {
				break
			}
		}
		setDisplay("block")
	}

	// updateSelection := func() {
	// 	for i, suggestion := range suggestions.QuerySelectorAll("li") {
	// 		if i == currentSelection {
	// 			suggestion.Class().Add("selected")
	// 		} else {
	// 			suggestion.Class().Remove("selected")
	// 		}
	// 	}
	// }
	//

	input.AddEventListener("input", false, func(e dom.Event) {
		query := strings.TrimSpace(input.Value())
		fmt.Println("Input callback")

		// Find matching books
		matches := findMatchingBooks(query)

		updateSuggestions(matches)
	})

	// Hide suggestions when clicking outside
	doc.AddEventListener("click", false, func(e dom.Event) {
		target := e.Target()
		if target != input && target != suggestions {
			updateSuggestions(nil)
		}
	})

	fmt.Printf("Autocomplete setup for %s %s", inputID, suggestionsID)
}
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

func min2(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func min(a, b, c int) int {
	return min2(min2(a, b), c)
}

type Match struct {
	book  Book
	score int
}

func findMatchingBooks(query string) []Match {
	if query == "" {
		return nil
	}
	matches := make([]Match, 0, len(data.Books))

	for _, userBook := range data.Books {
		book := userBook.Book

		title := book.CleanTitle()
		if strings.ToLower(query) != query { // only ignore case if the user used uppercase
			title = strings.ToLower(title)
		}

		score := LevenshteinDistance(query, title)
		fmt.Printf("%s has score %d with %s\n", title, score, query)
		if score > 0 {
			matches = append(matches, Match{
				book:  book,
				score: score,
			})
		}
	}
	slices.SortFunc(matches, func(a Match, b Match) int {
		return b.score - a.score
	})

	return matches
}
