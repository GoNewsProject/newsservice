package usecase

import (
	"context"
	"newsservice/internal/domain"
)

type NewsStorage interface {
	GetNews(ctx context.Context) ([]domain.Item, error)
}

type NewsGetterUseCase struct {
	storage NewsStorage
}

func NewNewsGetterUseCase(s NewsStorage) *NewsGetterUseCase {
	return &NewsGetterUseCase{storage: s}
}

func (us *NewsGetterUseCase) GetNews(ctx context.Context, limit int) ([]domain.Item, error) {
	return us.storage.GetNews(ctx)
}
