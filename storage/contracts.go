package storage

import (
	"context"
	"newsservice/internal/domain"
	"newsservice/internal/models"
)

type NewsStorage interface {
	GetDetailedNews(ctx context.Context, newsID int) (models.NewsFullDetailed, error)
	GetNewsList(ctx context.Context) ([]models.NewsFullDetailed, error)
	FilterNewsByContent(ctx context.Context, filter string) ([]models.NewsFullDetailed, error)
	FilterNewsByDate(ctx context.Context, date int) ([]models.NewsFullDetailed, error)
	FilterNewsByDateWithPagination(ctx context.Context, date int, offset, limit int) ([]models.NewsFullDetailed, error)
	FilterNewsByContentWithPagination(ctx context.Context, filter string, offset, limit int) ([]models.NewsFullDetailed, error)
	SaveNews(ctx context.Context, feed *domain.Feed) (int, error)
	Close()
}
