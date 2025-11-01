package parser

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"newsservice/internal/domain"
	"strings"
	"time"
)

type rssXML struct {
	Channel channelXML `xml:"channel"`
}

type channelXML struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Items       []itemXML `xml:"item"`
}

type itemXML struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

type XMLParser struct {
	log *slog.Logger
}

func New(log *slog.Logger) *XMLParser {
	return &XMLParser{
		log: log,
	}
}

func (p *XMLParser) Parse(ctx context.Context, reader io.Reader) (*domain.Feed, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var rss rssXML
	decoder := xml.NewDecoder(reader)
	if err := decoder.Decode(&rss); err != nil {
		p.log.Error(
			"Failed to decode XML",
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("failed to decode XML: %w", err)
	}
	feed := domain.Feed{
		Title:       rss.Channel.Title,
		Link:        rss.Channel.Link,
		Description: rss.Channel.Description,
		Items:       make([]domain.Item, 0, len(rss.Channel.Items)),
	}
	for _, itemDTO := range rss.Channel.Items {
		pubDate, err := parsePubDate(itemDTO.PubDate)
		if err != nil {
			p.log.Warn(
				"Could not parse item pubDate, skipping item",
				slog.String("pubDate", itemDTO.PubDate),
				slog.String("item_title", itemDTO.Title),
				slog.Any("error", err),
			)
			continue
		}
		item := domain.Item{
			Title:       itemDTO.Title,
			Link:        itemDTO.Link,
			Description: itemDTO.Description,
			PubDate:     pubDate,
		}
		feed.Items = append(feed.Items, item)
	}
	return &feed, nil
}

// parsePubDate - вспомогательная функция для парсинга даты в разных форматах.
func parsePubDate(dateStr string) (time.Time, error) {
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		"Mon, 2 Jan 2006 15:04:05 -0700",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, strings.TrimSpace(dateStr)); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("fail to parse date in any known format: %q", dateStr)
}
