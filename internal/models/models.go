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
