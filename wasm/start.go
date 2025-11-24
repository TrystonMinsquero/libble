package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	// . "libble/shared"

	"honnef.co/go/js/dom/v2"
)

func initStart() {
	doc := dom.GetWindow().Document()
	form := doc.GetElementByID("goodreads-user-form")
	errorMessage := doc.GetElementByID("error-message")

	showError := func(message string) {
		errorMessage.SetTextContent(message)
		errorMessage.Class().Add("visible")
	}

	hideError := func() {
		errorMessage.Class().Remove("visible")
	}

	form.AddEventListener("submit", false, func(e dom.Event) {
		e.PreventDefault()
		hideError()
		fmt.Println("Pressed Submit")

		doc = dom.GetWindow().Document()
		submitButton, ok := doc.GetElementByID("submit-button").(*dom.HTMLButtonElement)
		if !ok {
			logErr("Failed to get submit button")
			return
		}
		userGridInput := doc.GetElementByID("userId").(*dom.HTMLInputElement)

		userGrid := strings.TrimSpace(userGridInput.Value())

		submitButton.SetDisabled(true)
		submitText := submitButton.TextContent()
		submitButton.SetTextContent("Loading...")

		// Perform the fetch in a goroutine
		go func() {
			defer func() {
				submitButton.SetDisabled(false)
				submitButton.SetTextContent(submitText)
			}()

			if err := fetch("/user/"+userGrid, &data, http.MethodPost); err != nil {
				log(err, "Unabled to create user data")
				showError(err.Error())
				return
			}

			fmt.Println("Successfully created new user:")
			userId := strconv.FormatUint(uint64(data.User.ID), 10)

			if userId != "" {
				saveData("userId", userId)
				if err := saveJson("saveData", data); err != nil {
					log(err, "Failed to save data for creating user")
				}
				fmt.Println(data)
				location().SetHref(PageGame)
			} else {
				log(fmt.Errorf("UserId is empty"), "Failed to create user")
				showError("Failed to create user")
			}
		}()
	})
}
