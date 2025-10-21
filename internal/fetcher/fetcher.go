package fetcher

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

type HTTPFetcher struct {
	client *http.Client
	log    *slog.Logger
}

func New(log *slog.Logger) *HTTPFetcher {
	return &HTTPFetcher{
		client: http.DefaultClient,
		log:    log,
	}
}

func (f *HTTPFetcher) Fetch(ctx context.Context, url string) (io.ReadCloser, error) {
	log := f.log.With(slog.String("url", url))
	log.Info("Fetching URL")
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Error("Failed to create HTTP request", slog.Any("error", err))
		return nil, fmt.Errorf("failed to create request for url %s: %v", url, err)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		log.Error(
			"HTTP request failed",
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("failed to fetch url %s: %v", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		log.Error(
			"Unexpected status code",
			slog.Int("status_code", resp.StatusCode),
		)
		return nil, fmt.Errorf("unexpected status code %d for url %s", resp.StatusCode, url)
	}
	log.Info("Successfully fetched URL", slog.String("url", url))
	return resp.Body, nil
}
