package pagination

import "newsservice/internal/models"

type Pagination struct {
	TotalResults int
	TotalPages   int
	CurrentPage  int
	NewsPerPage  int
	Results      []models.NewsFullDetailed
}

const NEWS_PER_PAGE = 20

func New(totalResults int, currentPage int) *Pagination {
	return &Pagination{
		TotalResults: totalResults,
		TotalPages:   PageCounter(totalResults),
		CurrentPage:  currentPage,
		NewsPerPage:  NEWS_PER_PAGE,
	}
}

func PageCounter(totalResults int) int {
	totalPages := 1
	totalPages = totalResults / NEWS_PER_PAGE
	if totalPages*NEWS_PER_PAGE < totalResults {
		totalPages++
	}
	return totalPages
}
