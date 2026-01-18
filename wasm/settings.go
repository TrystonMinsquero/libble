package main

import (
	"fmt"

	"honnef.co/go/js/dom/v2"
)

// setupUserIcon initializes the user icon button and its click handler
func setupUserIcon() {
	doc := dom.GetWindow().Document()
	userIconBtn := getElemByID(doc, "userIconBtn")
	if userIconBtn == nil {
		return
	}

	// Add click event listener
	userIconBtn.AddEventListener("click", false, func(event dom.Event) {
		event.PreventDefault()
		showSettingsModal()
	})
}

// showSettingsModal displays the settings modal with appropriate content based on auth state
func showSettingsModal() {
	doc := dom.GetWindow().Document()
	modal := getElemByID(doc, "settingsModal")
	settingsContent := getElemByID(doc, "settingsContent")

	if modal == nil || settingsContent == nil {
		logErr("Settings modal elements not found")
		return
	}

	// Check if user is logged in
	libbleID := loadLibbleID()

	if libbleID == "" {
		// Not logged in - show email verification prompt
		settingsContent.SetInnerHTML(`
			<div class="settings-section">
				<label for="emailInput">Enter your email to verify your account:</label>
				<input type="email" id="emailInput" placeholder="your@email.com" required>
				<button class="settings-btn" id="verifyEmailBtn">Verify Email</button>
			</div>
		`)

		// Add event listener for email verification
		verifyBtn := getElemByID(doc, "verifyEmailBtn")
		if verifyBtn != nil {
			verifyBtn.AddEventListener("click", false, handleEmailVerification)
		}
	} else {
		// Logged in - show settings
		// Try to get existing email from localStorage
		existingEmail, _ := loadData("libble.userEmail")
		emailValue := ""
		if existingEmail != "" {
			emailValue = fmt.Sprintf(` value="%s"`, existingEmail)
		}

		settingsContent.SetInnerHTML(fmt.Sprintf(`
			<div class="settings-section">
				<p><strong>Libble ID:</strong> %s</p>
			</div>
			<div class="settings-section">
				<label for="emailInput">Email (optional for verification):</label>
				<input type="email" id="emailInput" placeholder="your@email.com"%s>
				<button class="settings-btn" id="saveEmailBtn">Save Email</button>
			</div>
			<div class="settings-section">
				<button class="settings-btn" id="logoutBtn">Logout</button>
			</div>
		`, libbleID, emailValue))

		// Add event listeners
		saveEmailBtn := getElemByID(doc, "saveEmailBtn")
		if saveEmailBtn != nil {
			saveEmailBtn.AddEventListener("click", false, handleSaveEmail)
		}

		logoutBtn := getElemByID(doc, "logoutBtn")
		if logoutBtn != nil {
			logoutBtn.AddEventListener("click", false, handleLogout)
		}
	}

	// Show modal
	modal.RemoveAttribute("hidden")

	// Setup close button
	closeBtn := getElemByID(doc, "closeModalBtn")
	if closeBtn != nil {
		closeBtn.AddEventListener("click", false, func(event dom.Event) {
			modal.SetAttribute("hidden", "")
		})
	}

	// Close modal when clicking outside the modal content
	modal.AddEventListener("click", false, func(event dom.Event) {
		target := event.Target()
		if target == modal {
			modal.SetAttribute("hidden", "")
		}
	})
}

// handleEmailVerification handles email verification for non-logged-in users
func handleEmailVerification(event dom.Event) {
	doc := dom.GetWindow().Document()
	emailInput := getElemByIDAs[*dom.HTMLInputElement](doc, "emailInput")
	if emailInput == nil {
		return
	}

	email := emailInput.Value()
	if email == "" {
		logErr("Email is empty")
		return
	}

	// TODO: Call backend API to verify email
	// For now, just store it and show a message
	saveData("libble.userEmail", email)
	debugPrint("Email verification requested for: %s", email)

	// Show feedback in the modal
	settingsContent := getElemByID(doc, "settingsContent")
	if settingsContent != nil {
		settingsContent.SetInnerHTML(`
			<div class="settings-section">
				<p style="color: #fff;">Email verification link sent! Check your inbox.</p>
				<button class="settings-btn" id="closeSettingsBtn">Close</button>
			</div>
		`)

		closeBtn := getElemByID(doc, "closeSettingsBtn")
		if closeBtn != nil {
			closeBtn.AddEventListener("click", false, func(e dom.Event) {
				modal := getElemByID(doc, "settingsModal")
				if modal != nil {
					modal.SetAttribute("hidden", "")
				}
			})
		}
	}
}

// handleSaveEmail handles saving email for logged-in users
func handleSaveEmail(event dom.Event) {
	doc := dom.GetWindow().Document()
	emailInput := getElemByIDAs[*dom.HTMLInputElement](doc, "emailInput")
	if emailInput == nil {
		return
	}

	email := emailInput.Value()
	if email == "" {
		logErr("Email is empty")
		return
	}

	// TODO: Call backend API to save email
	// For now, store in localStorage
	saveData("libble.userEmail", email)
	debugPrint("Saved email: %s", email)

	// Show success feedback
	settingsContent := getElemByID(doc, "settingsContent")
	if settingsContent != nil {
		libbleID := loadLibbleID()
		settingsContent.SetInnerHTML(fmt.Sprintf(`
			<div class="settings-section">
				<p><strong>Libble ID:</strong> %s</p>
			</div>
			<div class="settings-section">
				<p style="color: #90EE90;">✓ Email saved successfully!</p>
				<label for="emailInput">Email (optional for verification):</label>
				<input type="email" id="emailInput" placeholder="your@email.com" value="%s">
				<button class="settings-btn" id="saveEmailBtn">Save Email</button>
			</div>
			<div class="settings-section">
				<button class="settings-btn" id="logoutBtn">Logout</button>
			</div>
		`, libbleID, email))

		// Re-attach event listeners
		saveEmailBtn := getElemByID(doc, "saveEmailBtn")
		if saveEmailBtn != nil {
			saveEmailBtn.AddEventListener("click", false, handleSaveEmail)
		}

		logoutBtn := getElemByID(doc, "logoutBtn")
		if logoutBtn != nil {
			logoutBtn.AddEventListener("click", false, handleLogout)
		}
	}
}

// handleLogout handles user logout
func handleLogout(event dom.Event) {
	doc := dom.GetWindow().Document()

	// Clear localStorage
	localStorage := doc.Underlying().Get("localStorage")
	localStorage.Call("clear")
	debugPrint("Logged out - cleared localStorage")

	// Close modal
	modal := getElemByID(doc, "settingsModal")
	if modal != nil {
		modal.SetAttribute("hidden", "")
	}

	// Redirect to start page
	location().SetHref(PageStart)
}
