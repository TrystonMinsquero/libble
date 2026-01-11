package main

import (
	"fmt"
	"strings"

	. "libble/shared"

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

			// Step 0: Check for existing users
			showStatus("Checking for existing accounts...")

			type LookupResponse struct {
				Users []UserSummary `json:"users"`
			}

			var lookupResp LookupResponse
			if err := fetch("/user/lookup/"+userGrid, &lookupResp); err != nil {
				log(err, "Failed to lookup users")
				showError(err.Error())
				return
			}

			var libbleID DBID
			libbleIDStr := ""

			// If existing users found, ask user what to do
			if len(lookupResp.Users) > 0 {
				// TODO: Implement UI to show user options
				// For now, just use the first one
				// Future: Show list with game count and last played, let user choose
				libbleID = DBID(lookupResp.Users[0].LibbleID)
				libbleIDStr = fmt.Sprintf("%d", libbleID)
				saveLibbleID(libbleIDStr)
				debugPrint("Using existing user with libble ID: %d", libbleID)
				showStatus("Using existing account, loading your data...")
			} else {
				// No existing users, create new one
				showStatus("Creating your account...")

				reqBody := UserCreateRequest{
					GRID:          userGrid,
					ScrapeOptions: DefaultScrapeOptions(),
				}

				var createResp UserCreateResponse
				if err := post("/user/create", &createResp, reqBody); err != nil {
					log(err, "Failed to create user")
					showError(err.Error())
					return
				}

				if createResp.Error != "" {
					showError(createResp.Error)
					return
				}

				libbleID = createResp.Player.ID
				libbleIDStr = fmt.Sprintf("%d", libbleID)
				saveLibbleID(libbleIDStr)
				debugPrint("Created user with libble ID: %d", libbleID)
			}

			// TODO: switch to some sort of polling method
			// Also streamline getting into the game after finding one good quote
			// Then the server will keep processing so it's ready for tomorrow

			// Step 1: Fetch books
			showStatus("Fetching your books...")
			type BooksResponse struct {
				UserBooks []UserBook `json:"books"`
				Error     string     `json:"error,omitempty"`
			}
			var booksResp BooksResponse
			if err := fetch("/scrape/gr/user-books/"+libbleIDStr, &booksResp); err != nil {
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

			// Step 2: Fetch quotes for each book in parallel
			type QuotesResponse struct {
				Quotes []Quote `json:"quotes"`
				Error  string  `json:"error,omitempty"`
			}

			type bookResult struct {
				quotes []Quote
				index  int
			}

			resultChan := make(chan bookResult, len(books))
			completed := 0
			var allQuotes []Quote

			// Launch goroutines for each book
			for i, userBook := range books {
				go func(idx int, ub UserBook) {
					book := ub.Book
					var quotesResp QuotesResponse
					quotesURL := fmt.Sprintf("/scrape/gr/quotes/%s/%s", libbleIDStr, book.BookGRID)
					if err := fetch(quotesURL, &quotesResp); err != nil {
						log(err, "Failed to fetch quotes for book "+book.CleanTitle())
						resultChan <- bookResult{quotes: nil, index: idx}
						return
					}

					if quotesResp.Error != "" {
						logErr("Error fetching quotes for book %s:\n%s", book.CleanTitle(), quotesResp.Error)
						resultChan <- bookResult{quotes: nil, index: idx}
						return
					}

					resultChan <- bookResult{quotes: quotesResp.Quotes, index: idx}
				}(i, userBook)
			}

			// Collect results as they come in
			for range books {
				result := <-resultChan
				completed++
				if result.quotes != nil {
					allQuotes = append(allQuotes, result.quotes...)
				}
				showProgress(completed, len(books), len(allQuotes))
			}

			close(resultChan)

			showStatus(fmt.Sprintf("Successfully loaded %d quotes from %d books!", len(allQuotes), len(books)))

			// Create save data
			data := NewSaveData(libbleID, userGrid, books, allQuotes)
			debugPrint("Successfully loaded user data: %d", data.Player.ID)

			if err := saveAllData(data); err != nil {
				log(err, "Failed to save data, will start requesting data")
			}
		}()
	}

	form.AddEventListener("submit", false, func(e dom.Event) {
		e.PreventDefault()
		pressSubmit()
	})
}
