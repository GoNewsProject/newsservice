package http

import (
	"context"
	"log/slog"
	"net/http"
	"newsservice/internal/pagination"
	"newsservice/storage"
	"strconv"
	"time"

	httputils "github.com/Fau1con/renderresponse"
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
	api.mux.HandleFunc("/newsdetail/", api.GetDetailedNewsHandler)
	//маршрут для возврата списка новостей
	api.mux.HandleFunc("/newslist/", api.GetNewsListHandler)
	//маршрут для возврата списка  новостей отфильтрованных по контенту
	api.mux.HandleFunc("/newslist/filtered/", api.FilterNewsByContentHandler)
	//маршрут для возврата списка новостей отфильтрованных по дате публикации
	api.mux.HandleFunc("/newslist/filtered/date/", api.FilterNewsByPublishedHandler)
}

func (api *Api) GetDetailedNewsHandler(w http.ResponseWriter, r *http.Request) {
	if !httputils.ValidateMethod(w, r, http.MethodGet, http.MethodOptions) {
		return
	}

	newsIDStr := r.URL.Query().Get("id")
	if newsIDStr == "" {
		httputils.RenderError(w, "Invalid newsID parameter", http.StatusBadRequest)
		return
	}
	newsID, err := strconv.Atoi(newsIDStr)
	if err != nil {
		httputils.RenderError(w, "Invalid newsID format", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	detailPost, err := api.db.GetDetailedNews(ctx, newsID)
	if err != nil {
		httputils.RenderError(w, "Failed to get detailed post from database", http.StatusInternalServerError)
		return
	}

	httputils.RenderJSON(w, detailPost, http.StatusOK)
}

func (api *Api) GetNewsListHandler(w http.ResponseWriter, r *http.Request) {
	if !httputils.ValidateMethod(w, r, http.MethodGet, http.MethodOptions) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	result, err := api.db.GetNewsList(ctx)
	if err != nil {
		http.Error(w, "No news in database", http.StatusOK) // Возможно ли использование RenderError?
	}

	httputils.RenderJSON(w, result, http.StatusOK)
}

func (api *Api) FilterNewsByContentHandler(w http.ResponseWriter, r *http.Request) {
	if !httputils.ValidateMethod(w, r, http.MethodGet, http.MethodOptions) {
		return
	}

	pageStr := r.URL.Query().Get("page")
	if pageStr == "" {
		pageStr = "1"
	}
	page, _ := strconv.Atoi(pageStr)

	filter := r.URL.Query().Get("filter")
	if filter == "" {
		httputils.RenderError(w, "Unknown filter", http.StatusBadRequest)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	filteredData, err := api.db.FilterNewsByContent(ctx, filter)
	if err != nil {
		httputils.RenderError(w, "Failed to get filter data", http.StatusInternalServerError)
		return
	}
	pag := pagination.New(len(filteredData), page)
	results, err := api.db.FilterNewsByContentWithPagination(ctx, filter, (pag.CurrentPage-1)*pag.NewsPerPage, pag.NewsPerPage)
	if err != nil {
		httputils.RenderError(w, "Failed to get filtered by content news with pagination", http.StatusInternalServerError)
	}
	pag.Results = results

	httputils.RenderJSON(w, pag.Results, http.StatusOK)
}

func (api *Api) FilterNewsByPublishedHandler(w http.ResponseWriter, r *http.Request) {
	if !httputils.ValidateMethod(w, r, http.MethodGet, http.MethodOptions) {
		return
	}

	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		httputils.RenderError(w, "Invalid data patameter", http.StatusBadRequest)
		return
	}
	date, err := strconv.Atoi(dateStr) // Или нужно привести к TIMESTAMP?
	if err != nil {
		httputils.RenderError(w, "Invalid date parameter", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	filteredData, err := api.db.FilterNewsByDate(ctx, date)
	if err != nil {
		httputils.RenderError(w, "Failed to get filtered data", http.StatusInternalServerError)
		return
	}
	page := 1

	pag := pagination.New(len(filteredData), page)
	results, err := api.db.FilterNewsByDateWithPagination(ctx, date, (pag.CurrentPage-1)*pag.NewsPerPage, pag.NewsPerPage)
	if err != nil {
		httputils.RenderError(w, "Failed to get filtered by date news with pagination", http.StatusInternalServerError)
		return
	}
	pag.Results = results

	httputils.RenderJSON(w, pag.Results, http.StatusOK)

}
