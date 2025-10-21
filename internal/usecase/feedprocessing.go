package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

type FeedProcessingUseCase struct {
	fetcher   FeedFetcher
	parser    FeedParser
	storage   FeedStorage
	log       *slog.Logger
	feedNames map[string]string
}

func NewFeedProsessingUseCase(
	fetcher FeedFetcher,
	parser FeedParser,
	storage FeedStorage,
	log *slog.Logger,
	feedNames map[string]string,
) *FeedProcessingUseCase {
	return &FeedProcessingUseCase{
		fetcher:   fetcher,
		parser:    parser,
		storage:   storage,
		log:       log,
		feedNames: feedNames,
	}
}

// ProcessFeed выполняет полный цикл: получение, парсинг и сохранение фида.
func (uc *FeedProcessingUseCase) ProcessFeed(ctx context.Context, url string) error {
	start := time.Now()
	feedName := uc.extractFeedName(url)
	log := uc.log.With(
		slog.String("component", "feed-processor"),
		slog.String("feed", feedName),
		slog.String("url", url),
	)
	log.Info("Processing feed started")

	reader, err := uc.fetcher.Fetch(ctx, url)
	if err != nil {
		log.Error("Feed fetch failed",
			slog.String("stage", "fetch"),
			slog.Any("error", err),
		)
		return fmt.Errorf("fetch failed for %s: %w", feedName, err)
	}
	defer reader.Close()

	log.Debug("Feed fetched successfully", slog.String("stage", "fetch"))

	feed, err := uc.parser.Parse(ctx, reader)
	if err != nil {
		log.Error("Feed parsing error",
			slog.String("stage", "parse"),
			slog.Any("error", err),
		)
		return fmt.Errorf("parse failed for %s: %w", feedName, err)
	}

	log.Debug("Feed parsed succsessfully",
		slog.String("stage", "parse"),
		slog.Int("items_parsed", len(feed.Items)),
	)
	savedCount, err := uc.storage.SaveNews(ctx, feed)
	if err != nil {
		log.Error("Feed save failed",
			slog.String("stage", "save"),
			slog.Any("error", err),
		)
		return fmt.Errorf("save failed for %s: %w", feedName, err)
	}
	duration := time.Since(start)
	log.Info("Feed proessing completed successfully",
		slog.Int("items_found", len(feed.Items)),
		slog.Int("items_saved", savedCount),
		slog.Duration("duration", duration),
	)
	return nil
}

// extractFeedName извлекает читаемое имя фида из URL
func (uc *FeedProcessingUseCase) extractFeedName(url string) string {
	if name, ok := uc.feedNames[url]; ok {
		return name
	}
	parts := strings.Split(url, "/")
	if len(parts) >= 3 {
		domain := parts[2]
		if strings.HasPrefix(domain, "www.") {
			domain = domain[4:]
		}
		return domain
	}
	return "Unknown"
}
