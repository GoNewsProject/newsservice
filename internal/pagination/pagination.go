package pagination

import (
	"fmt"
	"newsservice/internal/models"
)

const NEWS_PER_PAGE = 20

type Pagination struct {
	TotalResults int                       `json:"total_results"`
	TotalPages   int                       `json:"total_pages"`
	CurrentPage  int                       `json:"current_page"`
	NewsPerPage  int                       `json:"news_per_page"`
	Results      []models.NewsFullDetailed `json:"results"`
	HasNext      bool                      `json:"has_next"`
	HasPrev      bool                      `json:"has_prev"`
	NextPage     int                       `json:"next_page,omitempty"`
	PrevPage     int                       `json:"prev_page,omitempty"`
}

func New(totalResults int, currentPage int) *Pagination {
	if currentPage < 1 {
		currentPage = 1
	}

	if totalResults < 0 {
		totalResults = 0
	}

	totalPages := calculateTotalPages(totalResults)

	if currentPage > totalPages && totalPages > 0 {
		currentPage = totalPages
	}

	hasNext := currentPage < totalPages
	hasPrev := currentPage > 1

	var nextPage, prevPage int
	if hasNext {
		nextPage = currentPage + 1
	}
	if hasPrev {
		prevPage = currentPage - 1
	}

	return &Pagination{
		TotalResults: totalResults,
		TotalPages:   totalPages,
		CurrentPage:  currentPage,
		NewsPerPage:  NEWS_PER_PAGE,
		HasNext:      hasNext,
		HasPrev:      hasPrev,
		NextPage:     nextPage,
		PrevPage:     prevPage,
	}
}

// calculateTotalPages вычисляет общее количество страниц
func calculateTotalPages(totalResults int) int {
	if totalResults <= 0 {
		return 0
	}

	totalPages := totalResults / NEWS_PER_PAGE
	if totalResults%NEWS_PER_PAGE != 0 {
		totalPages++
	}

	return totalPages
}

// GetOffset возвращает смещение для SQL запроса
func (p *Pagination) GetOffset() int {
	if p.CurrentPage < 1 {
		return 0
	}
	return (p.CurrentPage - 1) * p.NewsPerPage
}

// GetLimit возвращает лимит для SQL запроса
func (p *Pagination) GetLimit() int {
	return p.NewsPerPage
}

// SetResults устанавливает результаты и автоматически обновляет метаданные
func (p *Pagination) SetResults(results []models.NewsFullDetailed) {
	p.Results = results

	// Автоматическая корректировка, если получено неожиданное количество результатов
	if len(results) < p.NewsPerPage && p.CurrentPage == p.TotalPages {
		// На последней странице получили меньше результатов чем ожидалось
		p.TotalResults = (p.TotalPages-1)*p.NewsPerPage + len(results)
		p.TotalPages = calculateTotalPages(p.TotalResults)
		p.updateNavigation()
	}
}

// updateNavigation обновляет навигационные поля после изменения данных
func (p *Pagination) updateNavigation() {
	p.HasNext = p.CurrentPage < p.TotalPages
	p.HasPrev = p.CurrentPage > 1

	p.NextPage = 0
	if p.HasNext {
		p.NextPage = p.CurrentPage + 1
	}

	p.PrevPage = 0
	if p.HasPrev {
		p.PrevPage = p.CurrentPage - 1
	}
}

// Validate проверяет корректность параметров пагинации
func (p *Pagination) Validate() error {
	if p.CurrentPage < 1 {
		return fmt.Errorf("current page cannot be less than 1")
	}
	if p.TotalResults < 0 {
		return fmt.Errorf("total results cannot be negative")
	}
	if p.NewsPerPage < 1 {
		return fmt.Errorf("news per page cannot be less than 1")
	}
	return nil
}
