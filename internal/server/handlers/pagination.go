package handlers

import "strconv"

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

func parsePagination(rawPage, rawPageSize string) (page, pageSize int) {
	page, _ = strconv.Atoi(rawPage)
	pageSize, _ = strconv.Atoi(rawPageSize)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > maxPageSize {
		pageSize = defaultPageSize
	}
	return
}
