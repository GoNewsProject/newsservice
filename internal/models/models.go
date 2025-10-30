package models

import "time"

type NewsFullDetailed struct {
	NewsID      int       `json:"news_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Content     string    `json:"content"`
	Author      string    `json:"author"`
	PublishedAt time.Time `json:"published_at"`
	Source      string    `json:"source"`
	Link        string    `json:"link"`
	Tag         []string  `json:"tag"`
}

// NewsFilter структура фильтра для поиска новостей
type NewsFilter struct {
	Category string    `json:"category,omitempty"`
	Author   string    `json:"author,omitempty"`
	Date     time.Time `json:"date,omitempty"`
	Limit    int       `json:"limit,omitempty"`
	Offset   int       `json:"offset,omitempty"`
	OrderBy  string    `json:"order_by,omitempty"`
}
