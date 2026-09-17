package model

import "strconv"

// PageLink is one rendered pagination control: a numbered page, or an
// ellipsis placeholder where the full page range is omitted.
type PageLink struct {
	Page     int
	Label    string
	Current  bool
	Ellipsis bool
}

// Pagination describes one bounded, centered pagination control.
type Pagination struct {
	Page     int
	PageSize int
	LastPage int
	Links    []PageLink
}

// LastPage reports the final available page for totalItems at pageSize,
// always at least 1.
func LastPage(totalItems, pageSize int) int {
	if totalItems <= 0 || pageSize <= 0 {
		return 1
	}
	pages := (totalItems + pageSize - 1) / pageSize
	if pages < 1 {
		return 1
	}
	return pages
}

// ClampPage resolves a requested page to the nearest valid page: values
// below 1 become 1, values beyond the last page become the last page.
func ClampPage(requestedPage, totalItems, pageSize int) int {
	lastPage := LastPage(totalItems, pageSize)
	if requestedPage < 1 {
		return 1
	}
	if requestedPage > lastPage {
		return lastPage
	}
	return requestedPage
}

// pageLinkSpread is how many adjacent pages appear on each side of the
// current page before an ellipsis is used.
const pageLinkSpread = 2

// NewPagination builds a bounded first/adjacent/selected/final pagination
// control. An empty or single-page collection produces no links so callers
// can omit pagination controls entirely.
func NewPagination(requestedPage, totalItems, pageSize int) Pagination {
	lastPage := LastPage(totalItems, pageSize)
	page := ClampPage(requestedPage, totalItems, pageSize)
	pagination := Pagination{Page: page, PageSize: pageSize, LastPage: lastPage}
	if lastPage <= 1 {
		return pagination
	}

	low := page - pageLinkSpread
	if low < 1 {
		low = 1
	}
	high := page + pageLinkSpread
	if high > lastPage {
		high = lastPage
	}

	if low > 1 {
		pagination.Links = append(pagination.Links, PageLink{Page: 1, Label: "1"})
		if low > 2 {
			pagination.Links = append(pagination.Links, PageLink{Ellipsis: true})
		}
	}
	for value := low; value <= high; value++ {
		pagination.Links = append(pagination.Links, PageLink{Page: value, Label: strconv.Itoa(value), Current: value == page})
	}
	if high < lastPage {
		if high < lastPage-1 {
			pagination.Links = append(pagination.Links, PageLink{Ellipsis: true})
		}
		pagination.Links = append(pagination.Links, PageLink{Page: lastPage, Label: strconv.Itoa(lastPage)})
	}
	return pagination
}
