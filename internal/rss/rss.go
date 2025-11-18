package rss

import (
	"fmt"
	"log"
	"strings"
	"time"

	"newsservice/internal/models"

	strip "github.com/grokify/html-strip-tags-go"
	"github.com/mmcdole/gofeed"
)

// Метод - парсер источника RSS. На вход получается строку с URL источника, вовращает слайс объектов или ошибку.
func Parse(source string) ([]models.NewsFullDetailed, error) {
	parser := gofeed.NewParser()
	var news []models.NewsFullDetailed
	var new models.NewsFullDetailed
	feed, err := parser.ParseURL(source)
	if err != nil {
		log.Printf("Parsing error - %v", err)
		return nil, err
	}
	for _, item := range feed.Items {
		new, err = FeedItemToNews(item)
		if err != nil {
			log.Println(err)
		}
		news = append(news, new)
	}
	return news, nil
}

// Метод - конвертер объекта gofeed.Item, предоставляемаого библиотекой gofeed (объект статьи после парсинга XML),
// в объект статьи models.NewsFullDetailed. Возращает ошибку при наличии.
func FeedItemToNews(item *gofeed.Item) (news models.NewsFullDetailed, err error) {
	if item == nil {
		return news, fmt.Errorf("item is nil")
	}
	var t time.Time
	if item.Published != "" {
		published := strings.ReplaceAll(item.Published, ",", "")
		t, err = time.Parse("Mon 2 Jan 2006 15:04:05 -0700", published)
		if err != nil {
			t, err = time.Parse("Mon 2 Jan 2006 15:04:05 GMT", published)
		}
		if err != nil {
			// Если не удалось распарсить, используем текущее время
			t = time.Now()
		}
	} else {
		t = time.Now()
	}

	news.Content = item.Description
	news.Content = strip.StripTags(news.Content)
	author := convertAuthorToString(item.Author)

	news = models.NewsFullDetailed{
		Title:       item.Title,
		Description: news.Content,
		Content:     news.Content,
		Author:      author,
		Link:        item.Link,
		Source:      item.Link,
		PublishedAt: t,
	}

	// news = models.NewsFullDetailed{
	// 	Title:       item.Title,
	// 	Content:     news.Content,
	// 	PublishedAt: news.PublishedAt,
	// 	Link:        item.Link,
	// }
	return news, nil
}

func convertAuthorToString(author *gofeed.Person) string {
	if author == nil {
		return ""
	}
	if author.Name != "" {
		return author.Name
	}
	if author.Email != "" {
		return author.Email
	}

	return ""
}
