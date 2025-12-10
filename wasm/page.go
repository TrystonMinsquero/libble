package main

import (
	"fmt"
	"net/url"

	"honnef.co/go/js/dom/v2"
)

const PageGame = "/game.html"
const PageStart = "/start.html"

func location() *dom.URLUtils {
	return dom.GetWindow().Location().URLUtils
}

func isPage(page string) bool {
	return currPage() == page
}

func handlePage() string {
	userId, _ := loadData("userId")

	redirect := func(page string) string {
		curr := currPage()
		if curr != page {
			fmt.Printf("redirect %s -> %s\n", curr, page)
			location().SetHref(page)
			return page
		}
		return curr
	}

	if userId != "" {
		return redirect(PageGame)
	} else {
		return redirect(PageStart)
	}
}

func currPage() string {
	curr := location().Href()
	currUrl, err := url.Parse(curr)
	if err == nil {
		curr = currUrl.Path
	} else {
		log(err, "Failed parsing window.location.href")
	}
	return curr
}
