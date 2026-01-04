package main

import dom "honnef.co/go/js/dom/v2"

func getElemByID(doc dom.Document, ID string) dom.Element {
	elem := doc.GetElementByID(ID)
	if elem == nil {
		logErr("Failed to find %s in the dom", ID)
	}
	return elem
}

func getElemByIDAs[T any](doc dom.Document, ID string) T {
	var empty T
	if elem := getElemByID(doc, ID); elem != nil {
		if result, ok := elem.(T); ok {
			return result
		}
		logErr("Failed to cast %s to %T", ID, empty)
	}
	return empty
}

func setEnabled(elem dom.HTMLElement, enabled bool) {
	if elem == nil {
		return
	}
	if e := elem.Underlying(); !e.IsNull() {
		e.Set("disabled", !enabled)
	}
}

func setVisible(elem dom.HTMLElement, visible bool) {
	if elem == nil {
		return
	}
	if visible {
		elem.RemoveAttribute("hidden")
	} else {
		elem.SetAttribute("hidden", "")
	}
}
