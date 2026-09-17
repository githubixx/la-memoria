package model_test

import (
	"testing"

	"github.com/githubixx/la-memoria/bookmarker/model"
)

func TestNormalizeTagsFoldsCaseTrimsWhitespaceAndCollapsesDuplicates(t *testing.T) {
	tags := model.NormalizeTags([]string{"  Go  ", "GO", "go", "Ünïcödé", "ünïcödé", "", "   "})

	if len(tags) != 2 {
		t.Fatalf("normalized tags = %#v, want 2 unique entries", tags)
	}
	if tags[0].DisplayName != "Go" || tags[0].NormalizedName != "go" {
		t.Fatalf("first tag = %#v, want display %q normalized %q", tags[0], "Go", "go")
	}
	if tags[1].NormalizedName != "ünïcödé" {
		t.Fatalf("second tag normalized name = %q, want %q", tags[1].NormalizedName, "ünïcödé")
	}
}

func TestNormalizeTagsReturnsDeterministicNormalizedNameOrder(t *testing.T) {
	first := model.NormalizeTags([]string{"zebra", "apple", "mango"})
	second := model.NormalizeTags([]string{"mango", "zebra", "apple"})

	for index := range first {
		if first[index].NormalizedName != second[index].NormalizedName {
			t.Fatalf("tag order is not deterministic: %#v vs %#v", first, second)
		}
	}
	if first[0].NormalizedName != "apple" || first[len(first)-1].NormalizedName != "zebra" {
		t.Fatalf("tags = %#v, want normalized-name ascending order", first)
	}
}

func TestValidateBookmarkURLAcceptsHTTPHTTPSAndRejectsOthers(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{"https://example.test/reference", false},
		{"http://192.168.1.10/private", false},
		{"ftp://example.test/file", true},
		{"not a url", true},
		{"https://user:pass@example.test", true},
		{"", true},
	}
	for _, test := range tests {
		_, err := model.ValidateBookmarkURL(test.url)
		if (err != nil) != test.wantErr {
			t.Fatalf("ValidateBookmarkURL(%q) error = %v, wantErr %t", test.url, err, test.wantErr)
		}
	}
}

func TestSplitSearchWordsIgnoresPunctuationAndCase(t *testing.T) {
	words := model.SplitSearchWords("Database, transactions! (ACID)")
	want := []string{"database", "transactions", "acid"}
	if len(words) != len(want) {
		t.Fatalf("words = %#v, want %#v", words, want)
	}
	for index, word := range want {
		if words[index] != word {
			t.Fatalf("words = %#v, want %#v", words, want)
		}
	}
}
