package main

import (
	"fmt"
	. "libble/shared"

	dom "honnef.co/go/js/dom/v2"
)

type hintUI struct {
	kind   Hint
	elem   *dom.HTMLButtonElement
	canUse func(g Game) bool
	use    func(g *Game) string
}

const usedHintClass = "used-hint"

// TODO: Make some kind of interface so it's less error prone to add a new hint.
// Current functions we need to update is `createHints()`, `hintEmoji`, and `hintTooltip`
func createHints(hintsParent dom.HTMLElement) []hintUI {
	return []hintUI{{
		kind: HintTime,
		elem: addHintElem(HintTime, hintsParent),
		canUse: func(g Game) bool {
			// TODO: check user settings on whether to show the hint
			_, err := g.Book.UserData.LastReadDate()
			if err == nil {
				return true
			}
			debugPrint("Can't use time hint: %v", err)
			return false
		},
		use: func(g *Game) string {
			date, err := g.Book.UserData.LastReadDate()
			if err != nil {
				log(err, "Trying to use time hint but cant get date")
				return ""
			}
			msg := fmt.Sprintf("You read this book in %s of %d",
				date.Month().String(), date.Year())
			if !g.UsedHint(HintTime) {
				g.UseHint(HintTime)
			}
			return msg
		},
	}, {
		kind: HintSelfRating,
		elem: addHintElem(HintSelfRating, hintsParent),
		canUse: func(g Game) bool {
			if g.Book.UserData.Stars > 0 {
				return true
			}
			debugPrint("Can't use %s because there are no stars", HintSelfRating)
			return false
		},
		use: func(g *Game) string {
			msg := fmt.Sprintf("You gave this book %d stars", g.Book.UserData.Stars)
			if !g.UsedHint(HintSelfRating) {
				g.UseHint(HintSelfRating)
			}
			return msg
		},
	}, {
		kind: HintAuthorInitial,
		elem: addHintElem(HintAuthorInitial, hintsParent),
		canUse: func(g Game) bool {
			initials := g.Book.Book.AuthorInitials()
			if initials != "" {
				return true
			}
			debugPrint("Can't use %s. Author: %s", HintAuthorInitial, g.Book.Book.Author)
			return initials != ""
		},
		use: func(g *Game) string {
			initials := g.Book.Book.AuthorInitials()
			if initials == "" {
				logErr("Trying to use author initials but it's empty")
				return ""
			}
			if !g.UsedHint(HintAuthorInitial) {
				g.UseHint(HintAuthorInitial)
			}
			return fmt.Sprintf("The author's initials are %s", initials)
		},
	}}
}

func hintTooltip(kind Hint) string {
	switch kind {
	case HintTime:
		return "See when you last read the book"
	case HintSelfRating:
		return "See how many stars you gave the book"
	case HintAuthorInitial:
		return "See the initials of the author"
	}
	logErr("Hint '%s' does not have a tooltip", string(kind))
	return ""
}

func hintEmoji(kind Hint) string {
	switch kind {
	case HintTime:
		return "🕗"
	case HintSelfRating:
		return "⭐️"
	case HintAuthorInitial:
		return "🖊️"
	}
	logErr("Hint '%s' does not have a set emoji", string(kind))
	return "💡"
}

func addHintElem(kind Hint, parent dom.HTMLElement) *dom.HTMLButtonElement {
	doc := dom.GetWindow().Document()
	e := doc.CreateElement("button")
	e.SetTextContent(string(hintEmoji(kind)))

	e.Class().Add("hint-btn")
	e.Class().Add("game-input")
	tooltip := hintTooltip(kind)
	if tooltip != "" {
		e.SetAttribute("title", tooltip)
	}
	parent.AppendChild(e)
	button, ok := e.(*dom.HTMLButtonElement)
	if !ok {
		logErr("Failed to get button element for %s", string(kind))
	}
	return button
}
