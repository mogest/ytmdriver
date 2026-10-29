package main

import (
	"errors"
	"reflect"
	"testing"
)

func TestMatchingItems(t *testing.T) {
	albums := []item{
		{Key: "2", Title: "Djesse Vol. 2", Subtitle: "Album • Jacob Collier • 2019"},
		{Key: "1", Title: "Djesse Vol. 1", Subtitle: "Album • Jacob Collier • 2018"},
		{Key: "4", Title: "Djesse Vol. 4", Subtitle: "Album • Jacob Collier • 2024"},
	}
	tests := []struct {
		name  string
		items []item
		query string
		want  []string
	}{
		{"title number beats subtitle year", albums, "jesse 1", []string{"1"}},
		{"case insensitive title words", albums, "JESSE 4", []string{"4"}},
		{"artist fallback", albums, "collier 2024", []string{"4"}},
		{"words across title and subtitle", albums, "jesse 2019", []string{"2"}},
		{"ambiguous titles", albums, "jesse", []string{"2", "1", "4"}},
		{"ambiguous subtitles", albums, "jacob", []string{"2", "1", "4"}},
		{"no match", albums, "missing", nil},
		{"exact title beats partial title", []item{
			{Key: "live", Title: "Djesse Vol. 1 Live"},
			albums[1],
		}, "djesse vol. 1", []string{"1"}},
		{"duplicate exact titles stay ambiguous", []item{
			{Key: "a", Title: "Favourites"},
			{Key: "b", Title: "Favourites"},
		}, "favourites", []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, it := range matchingItems(tt.items, tt.query) {
				got = append(got, it.Key)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("matchingItems(%q) keys = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestSearchArgs(t *testing.T) {
	tests := []struct {
		args          []string
		query, filter string
	}{
		{[]string{"pink", "moon"}, "pink moon", ""},
		{[]string{"--albums", "ok", "computer"}, "ok computer", "albums"},
	}
	for _, tt := range tests {
		query, filter, err := searchArgs(tt.args)
		if query != tt.query || filter != tt.filter || err != nil {
			t.Errorf("searchArgs(%q) = %q, %q, %v", tt.args, query, filter, err)
		}
	}
	for _, args := range [][]string{nil, {"--songs"}, {"--artists", "nick", "drake"}} {
		if _, _, err := searchArgs(args); !errors.Is(err, errUsage) {
			t.Errorf("searchArgs(%q): got %v, want usage error", args, err)
		}
	}
}
