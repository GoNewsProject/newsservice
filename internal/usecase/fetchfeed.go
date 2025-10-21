package usecase

import (
	"context"
	"io"
	"newsservice/internal/domain"
)

// FeedFetcher — интерфейс для получения данных из источника.
type FeedFetcher interface {
	Fetch(ctx context.Context, url string) (io.ReadCloser, error)
}

// FeedParser — интерфейс для парсинга данных в доменную модель.
type FeedParser interface {
	Parse(ctx context.Context, reader io.Reader) (*domain.Feed, error)
}

// FeedStorage — интерфейс для сохранения фида.
type FeedStorage interface {
	SaveNews(ctx context.Context, feed *domain.Feed) (int, error)
}
