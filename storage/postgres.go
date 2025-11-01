package storage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"newsservice/internal/domain"
	"newsservice/internal/infrastructure/config"
	"newsservice/internal/models"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Storage struct {
	db       *pgxpool.Pool
	log      *slog.Logger
	isClosed bool
	mutex    sync.RWMutex
}

func NewStorage(cfg config.Config, log *slog.Logger) (*Storage, error) {
	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslode=%s",
		cfg.DB.Host, cfg.DB.Port, cfg.DB.UserName, cfg.DB.Password, cfg.DB.DBName, cfg.DB.SSLMode,
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
		db:  db,
		log: log,
	}, nil
}

// Метод для подсчета количества новостей для пагинации
func (s *Storage) GetNewsCount(ctx context.Context, filter models.NewsFilter) (int, error) {
	query := `SELECT COUNT(*) FROM news WHERE 1=1`
	args := []interface{}{}
	argPos := 1

	if filter.Category != "" {
		query += fmt.Sprintf(" AND category = $%d", argPos)
		args = append(args, filter.Category)
		argPos++
	}
	if filter.Author != "" {
		query += fmt.Sprintf(" AND author = $%d", argPos)
		args = append(args, filter.Author)
		argPos++
	}
	if !filter.Date.IsZero() {
		query += fmt.Sprintf(" AND DATE(publisher_at) = $%d", argPos)
		args = append(args, filter.Date.Format("2006-01-02"))
		argPos++
	}

	var count int
	err := s.db.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count news: %w", err)
	}

	return count, nil
}

// Метод для выборки новостей из БД по newsID
func (s *Storage) GetDetailedNews(ctx context.Context, newsID int) (models.NewsFullDetailed, error) {
	query := `SELECT id, title, description, content, author, published_at, source, link FROM news WHERE id = $1;`
	rows := s.db.QueryRow(ctx, query, newsID)

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

// Метод для выборки новостей из БД с фильтрацией и пагинацией
func (s *Storage) GetNewsByFilter(ctx context.Context, filter models.NewsFilter) ([]models.NewsFullDetailed, error) {
	query := `
	SELECT
	news_id,
	title,
	description,
	content,
	author,
	published_at,
	source,
	link,
	category
	FROM news
	WHERE 1=1
	`

	args := []interface{}{}
	argPos := 1

	if filter.Category != "" {
		query += fmt.Sprintf(" AND category = $%d", argPos)
		args = append(args, filter.Category)
		argPos++
	}
	if filter.Author != "" {
		query += fmt.Sprintf(" AND author = $%d", argPos)
		args = append(args, filter.Author)
		argPos++
	}
	if !filter.Date.IsZero() {
		query += fmt.Sprintf(" AND DATE(published_at) = $%d", argPos)
		args = append(args, filter.Date.Format("2006-01-02"))
		argPos++
	}
	if filter.OrderBy != "" {
		query += " ORDER BY " + filter.OrderBy
	} else {
		query += " ORDER BY published_at DESC"
	}
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, filter.Limit)
		argPos++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, filter.Offset)
	}

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query news: %w", err)
	}
	defer rows.Close()

	var news []models.NewsFullDetailed
	for rows.Next() {
		var item models.NewsFullDetailed

		err := rows.Scan(
			&item.NewsID,
			&item.Title,
			&item.Description,
			&item.Content,
			&item.Author,
			&item.PublishedAt,
			&item.Source,
			&item.Link,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan news row: %w", err)
		}

		news = append(news, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return news, nil
}

func (s *Storage) Close() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.isClosed {
		s.log.Debug("Storage already closed")
		return
	}

	if s.db != nil {
		s.db.Close()
		s.db = nil
		s.log.Info("Database connection pool closed successfully")
	}

	s.isClosed = true
}

func (s *Storage) SaveNews(ctx context.Context, feed *domain.Feed) (int, error) {
	if len(feed.Items) == 0 {
		return 0, nil
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		s.log.Error("failed to start transaction", "error", err)
		return 0, fmt.Errorf("failed to start transaction: %w", err)
	}

	//3. Обрабатываем ошибки и паники
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		} else if err != nil {
			if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
				s.log.Error("Failed to rollback transaction", "error", rollbackErr)
				return
			}
		}
	}()

	batch := &pgx.Batch{}
	query := `
	INSERT INTO news(
	title, description, content, author, published_at, source, link)
	VALUES($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (link) DO NOTHING);
	`

	//5. Обрабатываем новости из фида
	savedCount := 0
	for _, item := range feed.Items {
		content := item.Description

		batch.Queue(
			query,
			item.Title,
			item.Description,
			content,
			nil,
			item.PubDate,
			feed.Title,
			item.Link,
			nil,
		)
		savedCount++
	}
	//6. Выполняем batch
	batchResult := tx.SendBatch(ctx, batch)
	if err := batchResult.Close(); err != nil {
		s.log.Error("Failed to execute batch", "error", err)
		return 0, fmt.Errorf("failed to execute batch: %w", err)
	}
	//7. Фиксируем транзакцию
	if err := tx.Commit(ctx); err != nil {
		s.log.Error("Failed to commit transaction", "error", err)
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.log.Info("News saved successfully",
		slog.Int("items_saved", savedCount),
		slog.String("source", feed.Title),
	)

	return savedCount, nil
}

// ---------------------------------------------------------------------------------------------------------------------------
// Метод для выборки из БД всех новостей
// func (s *Storage) GetNewsList(ctx context.Context) ([]models.NewsFullDetailed, error) {
// 	query := `SELECT id, title, description, content, author, published_at, source, link FROM news ORDER BY published_at DESC;`
// 	rows, err := s.DB.Query(ctx, query)
// 	if err != nil {
// 		s.log.Error(
// 			"Failed to read data from database",
// 			slog.Any("error", err),
// 		)
// 		return nil, fmt.Errorf("failed to read data from database: %w", err)
// 	}
// 	defer rows.Close()

// 	news := []models.NewsFullDetailed{}
// 	for rows.Next() {
// 		post := models.NewsFullDetailed{}
// 		err = rows.Scan(
// 			&post.NewsID,
// 			&post.Title,
// 			&post.Description,
// 			&post.Content,
// 			&post.Author,
// 			&post.PublishedAt,
// 			&post.Source,
// 			&post.Link,
// 		)
// 		if err != nil {
// 			s.log.Error(
// 				"Failed to scan row",
// 				slog.Any("error", err),
// 			)
// 			return nil, fmt.Errorf("unable to scanrow: %w", err)
// 		}
// 		news = append(news, post)
// 	}

// 	return news, nil
// }

// // Метод для выборки из БД новостей с учетом заданного фильтра
// func (s *Storage) FilterNewsByContent(ctx context.Context, filter string) ([]models.NewsFullDetailed, error) {
// 	query := `SELECT id, title, description, published_at, link FROM news WHERE
//               LOWER(content) LIKE $1 OR LOWER(title) LIKE $1 OR LOWER(description) LIKE $1 ORDER BY published_at DESC;`
// 	rows, err := s.DB.Query(ctx, query, "%"+strings.ToLower(filter)+"%")
// 	if err != nil {
// 		s.log.Error(
// 			"Failed to read filtered data from database",
// 			slog.Any("error", err),
// 		)
// 		return nil, fmt.Errorf("failed to read filtered data from database: %w", err)
// 	}
// 	defer rows.Close()

// 	news := []models.NewsFullDetailed{}
// 	for rows.Next() {
// 		new := models.NewsFullDetailed{}
// 		err = rows.Scan(
// 			&new.NewsID,
// 			&new.Title,
// 			&new.Description,
// 			&new.PublishedAt,
// 			&new.Link,
// 		)
// 		if err != nil {
// 			s.log.Error(
// 				"Failed to scan row",
// 				slog.Any("error", err),
// 			)
// 			return nil, fmt.Errorf("unable scan row: %w", err)
// 		}
// 		news = append(news, new)
// 	}
// 	return news, nil
// }

// // Метод для выборки из БД новостей с учетом фильтра даты
// func (s *Storage) FilterNewsByDate(ctx context.Context, date int) ([]models.NewsFullDetailed, error) {
// 	query := `SELECT id, title, description, content, author, published_at, source, link FROM news WHERE published_at = $1 ORDER BY DESC;`

// 	rows, err := s.DB.Query(ctx, query, date)
// 	if err != nil {
// 		s.log.Error(
// 			"Failed to read filtered data from database",
// 			slog.Any("error", err),
// 		)
// 		return nil, fmt.Errorf("failed to read filtered data from database: %w", err)
// 	}
// 	defer rows.Close()
// 	news := []models.NewsFullDetailed{}
// 	for rows.Next() {
// 		post := models.NewsFullDetailed{}
// 		err = rows.Scan(
// 			&post.NewsID,
// 			&post.Title,
// 			&post.Description,
// 			&post.Content,
// 			&post.Author,
// 			&post.PublishedAt,
// 			&post.Source,
// 			&post.Link,
// 		)
// 		if err != nil {
// 			s.log.Error(
// 				"Failed to scan row",
// 				slog.Any("error", err),
// 			)
// 			return nil, fmt.Errorf("unable scan row: %w", err)
// 		}
// 		news = append(news, post)
// 	}
// 	return news, nil
// }

// // Метод для выборки из БД новостей с учетом фильтра даты и пагинацией
// func (s *Storage) FilterNewsByDateWithPagination(ctx context.Context, date int, offset, limit int) ([]models.NewsFullDetailed, error) {
// 	query := `SELECT id, title, description, content, author,published_at, source, link FROM news WHERE published_at = $1 ORDER BY DESC
// 	SELECT id, title, description, content, author, published_at, source, linl FROM subquery OFFSET $2 LIMIT $3;`

// 	rows, err := s.DB.Query(ctx, query, date, offset, limit)
// 	if err != nil {
// 		s.log.Error(
// 			"Failed to read filtered data from database",
// 			slog.Any("error", err),
// 		)
// 		return nil, fmt.Errorf("failed to read filtered data from database: %w", err)
// 	}
// 	defer rows.Close()
// 	news := []models.NewsFullDetailed{}
// 	for rows.Next() {
// 		post := models.NewsFullDetailed{}
// 		err = rows.Scan(
// 			&post.NewsID,
// 			&post.Title,
// 			&post.Description,
// 			&post.Content,
// 			&post.Author,
// 			&post.PublishedAt,
// 			&post.Source,
// 			&post.Link,
// 		)
// 		if err != nil {
// 			s.log.Error(
// 				"Failed to scan row",
// 				slog.Any("error", err),
// 			)
// 			return nil, fmt.Errorf("unable scan row: %w", err)
// 		}
// 		news = append(news, post)
// 	}
// 	return news, nil
// }

// // Метод для выборки из БД новостей с учетом заданного фильтра и пагинацией
// func (s *Storage) FilterNewsByContentWithPagination(ctx context.Context, filter string, offset, limit int) ([]models.NewsFullDetailed, error) {
// 	query := `WITH subquery AS (SELECT id, title, description, published_at, link FROM news WHERE LOWER(content) LIKE $1 OR LOWER(title) LIKE $1
//   OR LOWER(description) LIKE $1 ORDER BY published_at DESC) SELECT id, title, description, published_at, link FROM subquery OFFSET $2 LIMIT $3;`
// 	rows, err := s.DB.Query(ctx, query, ("%" + strings.ToLower(filter) + "%"), offset, limit)
// 	if err != nil {
// 		s.log.Error(
// 			"Failed to read filtered data from database",
// 			slog.Any("error", err),
// 		)
// 	}
// 	defer rows.Close()

// 	news := []models.NewsFullDetailed{}
// 	for rows.Next() {
// 		new := models.NewsFullDetailed{}
// 		err = rows.Scan(
// 			&new.NewsID,
// 			&new.Title,
// 			&new.Description,
// 			&new.PublishedAt,
// 			&new.Link,
// 		)
// 		if err != nil {
// 			s.log.Error(
// 				"Failed to scan row",
// 				slog.Any("error", err),
// 			)
// 			return nil, fmt.Errorf("unable scan row: %w", err)
// 		}
// 		news = append(news, new)
// 	}
// 	return news, nil
// }

// func (s *Storage) Close() {
// 	s.log.Info("Closing database connecting pool")
// 	s.DB.Close()
// }

// // Метод для сохрания новостей в БД
// func (s *Storage) SaveNews(ctx context.Context, feed *domain.Feed) (int, error) {
// 	if len(feed.Items) == 0 {
// 		return 0, nil
// 	}
// 	tx, err := s.DB.Begin(ctx)
// 	if err != nil {
// 		s.log.Error(
// 			"Failed to begin transaction",
// 			slog.Any("error", err),
// 		)
// 		return 0, fmt.Errorf("failed to begin transaction: %w", err)
// 	}

// 	defer func() {
// 		if err != nil {
// 			if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
// 				s.log.Error(
// 					"Failed to rollback transaction",
// 					slog.Any("error", rollbackErr),
// 				)
// 			}
// 		}
// 	}()

// 	batch := &pgx.Batch{}
// 	query := `
// 	INSERT INTO news (title, content, pub_date, link)
// 	VALUES ($1, $2, $3, $4)
// 	ON CONFLICT (link) DO NOTHING;
// 	`
// 	for _, item := range feed.Items {
// 		batch.Queue(
// 			query,
// 			item.Title,
// 			item.Description,
// 			item.PubDate,
// 			item.Link,
// 		)
// 	}

// 	batchResult := tx.SendBatch(ctx, batch)
// 	if err := batchResult.Close(); err != nil {
// 		s.log.Error(
// 			"Failed to execute batch",
// 			slog.Any("error", err),
// 		)
// 		return 0, fmt.Errorf("failed to execute bath: %w", err)
// 	}
// 	if err := tx.Commit(ctx); err != nil {
// 		s.log.Error(
// 			"Failed to commit transaction",
// 			slog.Any("error", err),
// 		)
// 		return 0, fmt.Errorf("failed to commit transaction: %w", err)
// 	}

// 	return len(feed.Items), nil
// }
