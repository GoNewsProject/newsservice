package storage

import (
	"context"
	"newsservice/internal/domain"
	"newsservice/internal/models"
)

type NewsStorage interface {
	GetNewsCount(ctx context.Context, filter models.NewsFilter) (int, error)
	GetDetailedNews(ctx context.Context, id int) (models.NewsFullDetailed, error)
	GetNewsByFilter(ctx context.Context, filter models.NewsFilter) ([]models.NewsFullDetailed, error)
	SaveNews(ctx context.Context, feed *domain.Feed) (int, error)
	Close()
}
