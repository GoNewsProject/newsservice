package domain

import "time"

type Item struct {
	Title       string
	Link        string
	Description string
	PubDate     time.Time
}

type Feed struct {
	Title       string
	Link        string
	Description string
	Items       []Item
}
