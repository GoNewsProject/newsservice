package rss

import (
	"log"
	"newsservice/internal/models"
	"reflect"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
)

func TestParse(t *testing.T) {
	validSource := "https://habr.com/ru/rss/hub/go/all/?fl=ru"
	parser := gofeed.NewParser()
	feed, err := parser.ParseURL(validSource)

	if err != nil {
		t.Fatalf("Error parsing URL to feed - %v", err)
	}
	var new models.NewsFullDetailed
	var news []models.NewsFullDetailed
	for _, item := range feed.Items {
		new, err = FeedItemToNews(item)
		if err != nil {
			log.Println(err)
		}
		news = append(news, new)
	}
	if len(news) < 1 {
		t.Fatalf("Error parsing feed to items - %v", err)
	}

}

func TestFeedItemToNews(t *testing.T) {
	type args struct {
		item *gofeed.Item
	}
	tests := []struct {
		name string
		args args
		want models.NewsFullDetailed
	}{
		{
			name: "Valid data",

			args: args{

				item: &gofeed.Item{
					Title:       "Test Title 1",
					Description: "Test Description 1",
					Published:   "Tue, 29 Oct 2024 11:20:40 GMT",
					Link:        "https://github.com/mmcdole/gofeed/blob/v1.3.0/parser.go#L96",
				},
			},

			want: models.NewsFullDetailed{
				Title:       "Test Title 1",
				Content:     "Test Description 1",
				PublishedAt: time.Date(2024, 10, 29, 11, 20, 40, 0, time.UTC),
				Link:        "https://github.com/mmcdole/gofeed/blob/v1.3.0/parser.go#L96",
			},
		},

		{
			name: "Empty data",

			args: args{

				item: &gofeed.Item{
					Title:     "",
					Content:   "",
					Published: "",
					Link:      "",
				},
			},

			want: models.NewsFullDetailed{
				Title:       "",
				Content:     "",
				PublishedAt: time.Time{},
				Link:        "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := FeedItemToNews(tt.args.item); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FeedItemToNewsFullDetailed() = %v, want %v", got, tt.want)
			}
		})
	}
}
