package main

import (
	"fmt"
	"strings"

	libble "libble/shared"

	"honnef.co/go/js/dom/v2"
)

func initStart() {
	doc := dom.GetWindow().Document()
	form := getElemByID(doc, "goodreads-user-form")
	errorMessage := getElemByID(doc, "error-message")
	statusMessage := getElemByID(doc, "status-message")
	progressContainer := getElemByID(doc, "progress-container")
	progressFill := getElemByID(doc, "progress-fill")
	progressText := getElemByID(doc, "progress-text")
	userGridInput := getElemByIDAs[*dom.HTMLInputElement](doc, "userId")
	submitButton := getElemByIDAs[*dom.HTMLButtonElement](doc, "submit-button")

	showError := func(message string) {
		errorMessage.SetTextContent(message)
		errorMessage.Class().Add("visible")
		statusMessage.Class().Remove("visible")
		progressContainer.Class().Remove("visible")
	}

	showStatus := func(message string) {
		statusMessage.SetTextContent(message)
		statusMessage.Class().Add("visible")
		errorMessage.Class().Remove("visible")
	}

	hideMessages := func() {
		errorMessage.Class().Remove("visible")
		statusMessage.Class().Remove("visible")
		progressContainer.Class().Remove("visible")
	}

	showProgress := func(current, total, cummulativeTotal int) {
		progressContainer.Class().Add("visible")
		percentage := (float64(current) / float64(total)) * 100
		progressFill.SetAttribute("style", fmt.Sprintf("width: %.1f%%", percentage))
		progressText.SetTextContent(fmt.Sprintf("%d / %d (%d)", current, total, cummulativeTotal))
	}

	pressSubmit := func() {
		hideMessages()
		userGrid := strings.TrimSpace(userGridInput.Value())

		submitButton.SetDisabled(true)
		submitButton.SetTextContent("Loading...")

		// Perform the fetch in a goroutine
		go func() {
			if canPlay() {
				location().SetHref(PageGame)
				return
			}
			defer func() {
				if canPlay() {
					submitButton.SetTextContent("Let's Play!")
				} else {
					submitButton.SetTextContent("Start Playing")
				}
				submitButton.SetDisabled(false)
			}()

			// Step 1: Fetch books
			showStatus("Fetching your books...")
			type BooksResponse struct {
				UserBooks []libble.UserBook `json:"books"`
				Error     string            `json:"error,omitempty"`
			}
			var booksResp BooksResponse
			if err := fetch("/scrape/gr/user-books/"+userGrid, &booksResp); err != nil {
				log(err, "Unable to fetch books")
				showError(err.Error())
				return
			}

			if booksResp.Error != "" {
				showError("Couldn't grab your books, Sorry!\n" + booksResp.Error)
				return
			}

			books := booksResp.UserBooks
			showStatus(fmt.Sprintf("Found %d books! Now fetching quotes...", len(books)))

			// Step 2: Fetch quotes for each book
			type QuotesResponse struct {
				Quotes []libble.Quote `json:"quotes"`
				Error  string         `json:"error,omitempty"`
			}
			var allQuotes []libble.Quote

			for i, userBook := range books {
				book := userBook.Book
				showProgress(i, len(books), len(allQuotes))

				var quotesResp QuotesResponse
				if err := fetch("/scrape/gr/quotes/"+book.BookGRID, &quotesResp); err != nil {
					log(err, "Failed to fetch quotes for book "+book.CleanTitle())
					continue
				}

				if quotesResp.Error != "" {
					logErr("Error fetching quotes for book %s:\n%s", book.CleanTitle(), quotesResp.Error)
					continue
				}

				allQuotes = append(allQuotes, quotesResp.Quotes...)
			}

			showProgress(len(books), len(books), len(allQuotes))

			showStatus(fmt.Sprintf("Successfully loaded %d quotes from %d books!", len(allQuotes), len(books)))

			// Create save data
			data := libble.NewSaveData(userGrid, books, allQuotes)
			debugPrint("Successfully created new user: %d", data.Player.ID)

			if err := saveAllData(data); err != nil {
				log(err, "Failed to save data")
				showError("Failed to save your data on device, sorry!")
			}
		}()
	}

	form.AddEventListener("submit", false, func(e dom.Event) {
		e.PreventDefault()
		pressSubmit()
	})
}
