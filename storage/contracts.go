package storage

import (
	"context"
	"newsservice/internal/models"
)

// Интерфейс базы данных
type DbInterface interface {
	GetDetailedNews(ctx context.Context, id int) (models.NewsFullDetailed, error)
	GetNewsList(ctx context.Context, filter models.NewsFilter) ([]models.NewsFullDetailed, error)
	GetFilteredNews(ctx context.Context, fiter models.NewsFilter) ([]models.NewsFullDetailed, error)
	GetNewsCount(ctx context.Context, filter models.NewsFilter) (int, error)
	NewsExists(ctx context.Context, id int) (bool, error)
	AddNews(ctx context.Context, news []models.NewsFullDetailed) (int, error)
}
