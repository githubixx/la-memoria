package model_test

import (
	"testing"

	"github.com/githubixx/la-memoria/bookmarker/model"
)

func TestClampPageResolvesOutOfRangeRequestsToValidBounds(t *testing.T) {
	tests := []struct {
		requestedPage, totalItems, pageSize, want int
	}{
		{0, 95, 10, 1},
		{-5, 95, 10, 1},
		{1, 95, 10, 1},
		{10, 95, 10, 10},
		{999, 95, 10, 10},
		{1, 0, 10, 1},
	}
	for _, test := range tests {
		got := model.ClampPage(test.requestedPage, test.totalItems, test.pageSize)
		if got != test.want {
			t.Fatalf("ClampPage(%d, %d, %d) = %d, want %d", test.requestedPage, test.totalItems, test.pageSize, got, test.want)
		}
	}
}

func TestNewPaginationProducesNoLinksForEmptyOrSinglePageCollections(t *testing.T) {
	empty := model.NewPagination(1, 0, 10)
	if len(empty.Links) != 0 || empty.LastPage != 1 {
		t.Fatalf("empty collection pagination = %#v, want no links and last page 1", empty)
	}
	singlePage := model.NewPagination(1, 7, 10)
	if len(singlePage.Links) != 0 {
		t.Fatalf("single-page pagination = %#v, want no links", singlePage)
	}
}

func TestNewPaginationBoundsFirstAdjacentSelectedAndFinalPagesWithEllipsis(t *testing.T) {
	pagination := model.NewPagination(25, 500, 10) // 50 pages total, page 25 selected

	if pagination.LastPage != 50 {
		t.Fatalf("last page = %d, want 50", pagination.LastPage)
	}
	first := pagination.Links[0]
	if first.Page != 1 || first.Ellipsis {
		t.Fatalf("first link = %#v, want page 1", first)
	}
	last := pagination.Links[len(pagination.Links)-1]
	if last.Page != 50 || last.Ellipsis {
		t.Fatalf("last link = %#v, want page 50", last)
	}
	hasEllipsis := false
	hasCurrent := false
	for _, link := range pagination.Links {
		if link.Ellipsis {
			hasEllipsis = true
		}
		if link.Current {
			if link.Page != 25 {
				t.Fatalf("current link = %#v, want page 25", link)
			}
			hasCurrent = true
		}
	}
	if !hasEllipsis {
		t.Fatalf("pagination links = %#v, want at least one ellipsis for 50 pages", pagination.Links)
	}
	if !hasCurrent {
		t.Fatalf("pagination links = %#v, want the current page marked", pagination.Links)
	}
}

func TestNewPaginationReachesFirstAdjacentSelectedAndFinalWithinTwoSelections(t *testing.T) {
	pagination := model.NewPagination(25, 500, 10)
	byPage := map[int]bool{}
	for _, link := range pagination.Links {
		if !link.Ellipsis {
			byPage[link.Page] = true
		}
	}
	for _, required := range []int{1, 24, 25, 26, 50} {
		if !byPage[required] {
			t.Fatalf("pagination links = %#v, missing directly reachable page %d", pagination.Links, required)
		}
	}
}
