package storage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"newsservice/internal/domain"
	"newsservice/internal/models"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Storage struct {
	DB  *pgxpool.Pool
	log *slog.Logger
}

func NewStorage(cfg infastructure.Config, log *slog.Logger) (*Storage, error) {
	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	db, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := db.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info("Database connettion established")
	return &Storage{
		DB:  db,
		log: log,
	}, nil
}

// Метод для выборки из БД новостей по newsID
func (s *Storage) GetDetailedNews(ctx context.Context, newsID int) (models.NewsFullDetailed, error) {
	query := `SELECT id, title, description, content, author, published_at, source, link FROM news WHERE id = $1;`
	rows := s.DB.QueryRow(ctx, query, newsID)

	post := models.NewsFullDetailed{}
	err := rows.Scan(
		&post.NewsID,
		&post.Title,
		&post.Description,
		&post.Author,
		&post.PublishedAt,
		&post.Source,
		&post.Link,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			s.log.Warn("News not found", "newsID", newsID)
			return models.NewsFullDetailed{}, fmt.Errorf("news with ID %d not found", newsID)
		}

		s.log.Error(
			"Failed to get detailed news from database",
			slog.Any("error", err),
			slog.Int("newsID", newsID),
		)
		return models.NewsFullDetailed{}, fmt.Errorf("unable scan row: %w", err)
	}

	return post, nil
}

// Метод для выборки из БД всех новостей
func (s *Storage) GetNewsList(ctx context.Context) ([]models.NewsFullDetailed, error) {
	query := `SELECT id, title, description, content, author, published_at, source, link FROM news ORDER BY published_at DESC;`
	rows, err := s.DB.Query(ctx, query)
	if err != nil {
		s.log.Error(
			"Failed to read data from database",
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("failed to read data from database: %w", err)
	}
	defer rows.Close()

	news := []models.NewsFullDetailed{}
	for rows.Next() {
		post := models.NewsFullDetailed{}
		err = rows.Scan(
			&post.NewsID,
			&post.Title,
			&post.Description,
			&post.Content,
			&post.Author,
			&post.PublishedAt,
			&post.Source,
			&post.Link,
		)
		if err != nil {
			s.log.Error(
				"Failed to scan row",
				slog.Any("error", err),
			)
			return nil, fmt.Errorf("unable to scanrow: %w", err)
		}
		news = append(news, post)
	}

	return news, nil
}

// Метод для выборки из БД новостей с учетом заданного фильтра
func (s *Storage) FilterNewsByContent(ctx context.Context, filter string) ([]models.NewsFullDetailed, error) {
	query := `SELECT id, title, description, published_at, link FROM news WHERE 
              LOWER(content) LIKE $1 OR LOWER(title) LIKE $1 OR LOWER(description) LIKE $1 ORDER BY published_at DESC;`
	rows, err := s.DB.Query(ctx, query, "%"+strings.ToLower(filter)+"%")
	if err != nil {
		s.log.Error(
			"Failed to read filtered data from database",
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("failed to read filtered data from database: %w", err)
	}
	defer rows.Close()

	news := []models.NewsFullDetailed{}
	for rows.Next() {
		new := models.NewsFullDetailed{}
		err = rows.Scan(
			&new.NewsID,
			&new.Title,
			&new.Description,
			&new.PublishedAt,
			&new.Link,
		)
		if err != nil {
			s.log.Error(
				"Failed to scan row",
				slog.Any("error", err),
			)
			return nil, fmt.Errorf("unable scan row: %w", err)
		}
		news = append(news, new)
	}
	return news, nil
}

// Метод для выборки из БД новостей с учетом фильтра даты
func (s *Storage) FilterNewsByDate(ctx context.Context, date int) ([]models.NewsFullDetailed, error) {
	query := `SELECT id, title, description, content, author, published_at, source, link FROM news WHERE published_at = $1 ORDER BY DESC;`

	rows, err := s.DB.Query(ctx, query, date)
	if err != nil {
		s.log.Error(
			"Failed to read filtered data from database",
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("failed to read filtered data from database: %w", err)
	}
	defer rows.Close()
	news := []models.NewsFullDetailed{}
	for rows.Next() {
		post := models.NewsFullDetailed{}
		err = rows.Scan(
			&post.NewsID,
			&post.Title,
			&post.Description,
			&post.Content,
			&post.Author,
			&post.PublishedAt,
			&post.Source,
			&post.Link,
		)
		if err != nil {
			s.log.Error(
				"Failed to scan row",
				slog.Any("error", err),
			)
			return nil, fmt.Errorf("unable scan row: %w", err)
		}
		news = append(news, post)
	}
	return news, nil
}

// Метод для выборки из БД новостей с учетом фильтра даты и пагинацией
func (s *Storage) FilterNewsByDateWithPagination(ctx context.Context, date int, offset, limit int) ([]models.NewsFullDetailed, error) {
	query := `SELECT id, title, description, content, author,published_at, source, link FROM news WHERE published_at = $1 ORDER BY DESC
	SELECT id, title, description, content, author, published_at, source, linl FROM subquery OFFSET $2 LIMIT $3;`

	rows, err := s.DB.Query(ctx, query, date, offset, limit)
	if err != nil {
		s.log.Error(
			"Failed to read filtered data from database",
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("failed to read filtered data from database: %w", err)
	}
	defer rows.Close()
	news := []models.NewsFullDetailed{}
	for rows.Next() {
		post := models.NewsFullDetailed{}
		err = rows.Scan(
			&post.NewsID,
			&post.Title,
			&post.Description,
			&post.Content,
			&post.Author,
			&post.PublishedAt,
			&post.Source,
			&post.Link,
		)
		if err != nil {
			s.log.Error(
				"Failed to scan row",
				slog.Any("error", err),
			)
			return nil, fmt.Errorf("unable scan row: %w", err)
		}
		news = append(news, post)
	}
	return news, nil
}

// Метод для выборки из БД новостей с учетом заданного фильтра и пагинацией
func (s *Storage) FilterNewsByContentWithPagination(ctx context.Context, filter string, offset, limit int) ([]models.NewsFullDetailed, error) {
	query := `WITH subquery AS (SELECT id, title, description, published_at, link FROM news WHERE LOWER(content) LIKE $1 OR LOWER(title) LIKE $1
  OR LOWER(description) LIKE $1 ORDER BY published_at DESC) SELECT id, title, description, published_at, link FROM subquery OFFSET $2 LIMIT $3;`
	rows, err := s.DB.Query(ctx, query, ("%" + strings.ToLower(filter) + "%"), offset, limit)
	if err != nil {
		s.log.Error(
			"Failed to read filtered data from database",
			slog.Any("error", err),
		)
	}
	defer rows.Close()

	news := []models.NewsFullDetailed{}
	for rows.Next() {
		new := models.NewsFullDetailed{}
		err = rows.Scan(
			&new.NewsID,
			&new.Title,
			&new.Description,
			&new.PublishedAt,
			&new.Link,
		)
		if err != nil {
			s.log.Error(
				"Failed to scan row",
				slog.Any("error", err),
			)
			return nil, fmt.Errorf("unable scan row: %w", err)
		}
		news = append(news, new)
	}
	return news, nil
}

func (s *Storage) Close() {
	s.log.Info("Closing database connecting pool")
	s.DB.Close()
}

// Метод для сохрания новостей в БД
func (s *Storage) SaveNews(ctx context.Context, feed *domain.Feed) (int, error) {
	if len(feed.Items) == 0 {
		return 0, nil
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		s.log.Error(
			"Failed to begin transaction",
			slog.Any("error", err),
		)
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
				s.log.Error(
					"Failed to rollback transaction",
					slog.Any("error", rollbackErr),
				)
			}
		}
	}()

	batch := &pgx.Batch{}
	query := `
	INSERT INTO news (title, content, pub_date, link)
	VALUES ($1, $2, $3, $4)
	ON CONFLICT (link) DO NOTHING;
	`
	for _, item := range feed.Items {
		batch.Queue(
			query,
			item.Title,
			item.Description,
			item.PubDate,
			item.Link,
		)
	}

	batchResult := tx.SendBatch(ctx, batch)
	if err := batchResult.Close(); err != nil {
		s.log.Error(
			"Failed to execute batch",
			slog.Any("error", err),
		)
		return 0, fmt.Errorf("failed to execute bath: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		s.log.Error(
			"Failed to commit transaction",
			slog.Any("error", err),
		)
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return len(feed.Items), nil
}
