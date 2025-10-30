package api

import (
	"context"
	"log/slog"
	"net/http"
	transport "newsservice/internal/transport/http"
	"newsservice/storage"
)

type Api struct {
	mux *http.ServeMux
	db  storage.NewsStorage
	ctx context.Context
	log *slog.Logger
}

func NewApi(db storage.NewsStorage, log *slog.Logger) *Api {
	api := Api{
		mux: http.NewServeMux(),
		db:  db,
		log: log,
		ctx: context.Background(),
	}
	api.endpoints()
	return &api
}

// Метод регистратор endpoint-ов, настраивающий саброутинг.
func (api *Api) endpoints() {
	//маршрут для возврата детальной информации о новости
	api.mux.HandleFunc("/newsdetail/", transport.HandleDetailedNews())
	//маршрут для возврата списка новостей
	api.mux.HandleFunc("/newslist/", transport.HandleGetNewsList())
	//маршрут для возврата списка  новостей отфильтрованных по контенту
	api.mux.HandleFunc("/newslist/filtered/", transport.HandleFilterNewsByContent())
	//маршрут для возврата списка новостей отфильтрованных по дате публикации
	// api.mux.HandleFunc("/newslist/filtered/date/", transport.FilterNewsByPublishedHandler)
}
