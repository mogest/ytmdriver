package main

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

type item struct {
	Key      string `json:"key"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
}

// matches reports whether every word of query appears in the item's title or subtitle.
func (it item) matches(query string) bool {
	return matchesWords(it.Title+" "+it.Subtitle, query)
}

func matchesWords(text, query string) bool {
	text = strings.ToLower(text)
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

var searchFilters = []string{"songs", "albums", "videos", "playlists"}

// searchArgs splits an optional leading --filter, e.g. --songs, from the words to search for.
func searchArgs(args []string) (query, filter string, err error) {
	if len(args) > 0 && strings.HasPrefix(args[0], "--") {
		filter, args = args[0][2:], args[1:]
		if !slices.Contains(searchFilters, filter) {
			return "", "", errUsage
		}
	}
	if len(args) == 0 {
		return "", "", errUsage
	}
	return strings.Join(args, " "), filter, nil
}

func searchItems(query, filter string) ([]item, error) {
	r, err := request("search", map[string]any{"query": query, "filter": filter})
	return r.Items, err
}

func printItems(items []item, err error) error {
	if err != nil {
		return err
	}
	for _, it := range items {
		fmt.Printf("%s\t%s\n", it.Title, it.Subtitle)
	}
	return nil
}

func printResults(items []item, err error) error {
	if err != nil {
		return err
	}
	for i, it := range items {
		fmt.Printf("%d\t%s\t%s\n", i+1, it.Title, it.Subtitle)
	}
	return nil
}

// playCommand strips a --queue flag, which adds to the end of the queue instead of playing.
func playCommand(args []string) (string, []string) {
	if i := slices.Index(args, "--queue"); i >= 0 {
		return "queue_result", slices.Delete(slices.Clone(args), i, i+1)
	}
	return "play_item", args
}

// playResult plays a result already on the search results page, without searching again.
func playResult(cmd string, n int) error {
	items, err := libraryItems("search")
	if err != nil {
		return err
	}
	if n < 1 || n > len(items) {
		return fmt.Errorf("no result %d among the %d search results showing", n, len(items))
	}
	_, err = request(cmd, map[string]any{"kind": "search", "key": items[n-1].Key})
	return err
}

func playSearch(cmd, query, filter string) error {
	items, err := searchItems(query, filter)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return fmt.Errorf("nothing found for %q", query)
	}
	_, err = request(cmd, map[string]any{"kind": "search", "key": items[0].Key})
	return err
}

func playFromLibrary(kind, query string) error {
	items, err := libraryItems(kind)
	if err != nil {
		return err
	}
	it, err := pick(items, kind, query)
	if err != nil {
		return err
	}
	_, err = request("play_item", map[string]any{"kind": kind, "key": it.Key})
	return err
}

// saveToPlaylist saves the current track to the playlist matching query, or with no query, the one last saved to.
func saveToPlaylist(query string) error {
	playlist := readConfig("playlist")
	if query != "" {
		items, err := libraryItems("playlists")
		if err != nil {
			return err
		}
		it, err := pick(items, "playlists", query)
		if err != nil {
			return err
		}
		playlist = it.Key
	} else if playlist == "" {
		return errors.New("no playlist saved to yet; name one")
	}
	if _, err := request("save_to_playlist", map[string]any{"playlist": playlist}); err != nil {
		return err
	}
	writeConfig("playlist", playlist)
	return nil
}

// pick returns the one item matching query, or an error listing the candidates if the best matches tie.
func pick(items []item, where, query string) (item, error) {
	found := matchingItems(items, query)
	switch len(found) {
	case 0:
		return item{}, fmt.Errorf("nothing in %s matches %q", where, query)
	case 1:
		return found[0], nil
	}
	titles := make([]string, len(found))
	for i, it := range found {
		titles[i] = "  " + it.Title
	}
	return item{}, fmt.Errorf("%q matches more than one:\n%s", query, strings.Join(titles, "\n"))
}

// matchingItems returns candidates from the first matching tier: exact title,
// words in the title, then words in the combined title and subtitle.
func matchingItems(items []item, query string) []item {
	var exact, titles, combined []item
	for _, it := range items {
		switch {
		case strings.EqualFold(it.Title, query):
			exact = append(exact, it)
		case matchesWords(it.Title, query):
			titles = append(titles, it)
		case it.matches(query):
			combined = append(combined, it)
		}
	}
	if len(exact) > 0 {
		return exact
	}
	if len(titles) > 0 {
		return titles
	}
	return combined
}
