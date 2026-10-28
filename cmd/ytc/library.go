package main

import (
	"fmt"
	"strings"
)

type item struct {
	Key      string `json:"key"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
}

// matches reports whether every word of query appears in the item's title or subtitle.
func (it item) matches(query string) bool {
	text := strings.ToLower(it.Title + " " + it.Subtitle)
	for _, word := range strings.Fields(strings.ToLower(query)) {
		if !strings.Contains(text, word) {
			return false
		}
	}
	return true
}

func libraryItems(kind string) ([]item, error) {
	r, err := request("library", map[string]any{"kind": kind})
	return r.Items, err
}

func listLibrary(kind string) error {
	items, err := libraryItems(kind)
	if err != nil {
		return err
	}
	for _, it := range items {
		fmt.Printf("%s\t%s\n", it.Title, it.Subtitle)
	}
	return nil
}

func playFromLibrary(kind, query string) error {
	items, err := libraryItems(kind)
	if err != nil {
		return err
	}
	var found []item
	for _, it := range items {
		if strings.EqualFold(it.Title, query) {
			found = []item{it}
			break
		}
		if it.matches(query) {
			found = append(found, it)
		}
	}
	switch len(found) {
	case 0:
		return fmt.Errorf("nothing in %s matches %q", kind, query)
	case 1:
		_, err := request("play_item", map[string]any{"kind": kind, "key": found[0].Key})
		return err
	}
	titles := make([]string, len(found))
	for i, it := range found {
		titles[i] = "  " + it.Title
	}
	return fmt.Errorf("%q matches more than one:\n%s", query, strings.Join(titles, "\n"))
}
