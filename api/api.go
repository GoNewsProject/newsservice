package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"newsservice/internal/models"
	"newsservice/internal/pagination"
	"newsservice/storage"
	"strconv"
	"strings"
	"time"

	httputils "github.com/Fau1con/renderresponse"
)

type Api struct {
	ctx context.Context
	mux *http.ServeMux
	db  storage.DbInterface
	log *slog.Logger
}

func NewApi(ctx context.Context, mux *http.ServeMux, db storage.DbInterface, log *slog.Logger) *Api {
	api := Api{
		ctx: ctx,
		mux: mux,
		db:  db,
		log: log,
	}
	api.endpoints()
	return &api
}

func (api *Api) Router() http.Handler {
	return api.mux
}

// Метод регистратор endpoint-ов, настраивающий саброутинг.
func (api *Api) endpoints() {
	//маршрут для возврата детальной информации о новости
	api.mux.HandleFunc("/newsdetail/", api.getDetaileNewsHandler)
	//маршрут для возврата списка новостей
	api.mux.HandleFunc("/newslist/", api.getNewsListHandler)
	//маршрут для возврата списка  новостей отфильтрованных по контенту
	api.mux.HandleFunc("/newslist/filtered/", api.getNewsByContentHandler)
	// маршру для проверки наличия новости
	api.mux.HandleFunc("/news/", api.handleNewsExists)
	//маршрут для возврата списка новостей отфильтрованных по дате публикации
	// api.mux.HandleFunc("/newslist/filtered/date/", transport.FilterNewsByPublishedHandler)
}

func (api *Api) getDetaileNewsHandler(w http.ResponseWriter, r *http.Request) {
	if !httputils.ValidateMethod(w, r, http.MethodGet, http.MethodOptions) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	params, err := parseURLParams(r.URL.String())
	if err != nil {
		httputils.RenderError(w, "failed to parse query parameters from URL", http.StatusBadRequest, err)
		return
	}
	newsIDStr, exists := params["newsID"]
	if !exists {
		httputils.RenderError(w, "news ID parameter not found", http.StatusBadRequest)
		return
	}
	newsID, err := strconv.Atoi(newsIDStr)
	if err != nil {
		httputils.RenderError(w, "failed to parse news ID", http.StatusBadRequest, err)
		return
	}

	news, err := api.db.GetDetailedNews(ctx, newsID)
	if err != nil {
		httputils.RenderError(w, "failed to get new from datbase", http.StatusInternalServerError, err)
		return
	}

	httputils.RenderJSON(w, news, http.StatusOK)
}

func (api *Api) getNewsListHandler(w http.ResponseWriter, r *http.Request) {
	if !httputils.ValidateMethod(w, r, http.MethodGet, http.MethodOptions) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	params, err := parseURLParams(r.URL.String())
	if err != nil {
		httputils.RenderError(w, "failed to parse query parameters from URL", http.StatusBadRequest, err)
		return
	}

	pageStr, exists := params["page"]
	if !exists {
		httputils.RenderError(w, "page parameter not found", http.StatusBadRequest)
	} // Нужнен return?
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		httputils.RenderError(w, "failed to parse page parameter", http.StatusBadRequest)
	} // Нужнен return?
	if page < 1 {
		page = 1
	}

	offsetStr, exists := params["offset"]
	if !exists {
		httputils.RenderJSON(w, "offset parameter not found", http.StatusBadRequest)
	} // Нужнен return?
	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		httputils.RenderError(w, "failed to parse offset parameter", http.StatusBadRequest, err)
	} // Нужнен return?
	if offset <= 0 {
		offset = (page - 1) * pagination.NEWS_PER_PAGE
	}

	limitStr, exists := params["limit"]
	if !exists {
		httputils.RenderError(w, "limit parameter not found", http.StatusBadRequest)
	} // Нужнен return?
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		httputils.RenderError(w, "failed to parse limit parameter", http.StatusBadRequest)
	} // Нужнен return?
	if limit <= 0 {
		limit = pagination.NEWS_PER_PAGE
	}

	filter := models.NewsFilter{
		Limit:  limit,
		Offset: offset,
	}

	totalResult, err := api.db.GetNewsCount(ctx, filter)
	if err != nil {
		httputils.RenderError(w, "failed to count total filter news", http.StatusInternalServerError, err)
		return
	}

	paginator := pagination.New(totalResult, page)
	if err := paginator.Validate(); err != nil {
		httputils.RenderError(w, "paginatin validation failed", http.StatusInternalServerError, err)
		return
	}

	news, err := api.db.GetNewsList(ctx, filter)
	if err != nil {
		httputils.RenderError(w, "failed to get news from database", http.StatusInternalServerError, err)
		return
	}

	paginator.SetResults(news)

	httputils.RenderJSON(w, news, http.StatusOK)
}

func (api *Api) getNewsByContentHandler(w http.ResponseWriter, r *http.Request) {
	if !httputils.ValidateMethod(w, r, http.MethodGet, http.MethodOptions) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	params, err := parseURLParams(r.URL.String())
	if err != nil {
		httputils.RenderError(w, "failed to parse query parameters from URL", http.StatusBadRequest, err)
		return
	}

	pageStr, exists := params["page"]
	if !exists {
		httputils.RenderError(w, "page parameter not found", http.StatusBadRequest)
	} // Нужнен return?
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		httputils.RenderError(w, "failed to parse page parameter", http.StatusBadRequest)
	} // Нужнен return?
	if page < 1 {
		page = 1
	}

	offsetStr, exists := params["offset"]
	if !exists {
		httputils.RenderJSON(w, "offset parameter not found", http.StatusBadRequest)
	} // Нужнен return?
	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		httputils.RenderError(w, "failed to parse offset parameter", http.StatusBadRequest, err)
	} // Нужнен return?
	if offset <= 0 {
		offset = (page - 1) * pagination.NEWS_PER_PAGE
	}

	limitStr, exists := params["limit"]
	if !exists {
		httputils.RenderError(w, "limit parameter not found", http.StatusBadRequest)
	} // Нужнен return?
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		httputils.RenderError(w, "failed to parse limit parameter", http.StatusBadRequest)
	} // Нужнен return?
	if limit <= 0 {
		limit = pagination.NEWS_PER_PAGE
	}

	dateStr, exists := params["date"]
	if !exists {
		httputils.RenderError(w, "date patameter not found", http.StatusBadRequest)
	} // Нужнен return?
	parsedDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		httputils.RenderError(w, "failed to parse data", http.StatusBadRequest, err)
	}

	category := params["category"]
	author := params["author"]

	filter := models.NewsFilter{
		Category: category,
		Author:   author,
		Date:     parsedDate,
		Limit:    limit,
		Offset:   offset,
	}

	totalResult, err := api.db.GetNewsCount(ctx, filter)
	if err != nil {
		httputils.RenderError(w, "failed to count total filter news", http.StatusInternalServerError, err)
		return
	}
	paginator := pagination.New(totalResult, page)
	if err := paginator.Validate(); err != nil {
		httputils.RenderError(w, "pagination validation failes", http.StatusInternalServerError, err)
		return
	}

	news, err := api.db.GetFilteredNews(ctx, filter)
	if err != nil {
		httputils.RenderError(w, "failed to get filtered news from database", http.StatusInternalServerError, err)
		return
	}

	paginator.SetResults(news)

	httputils.RenderJSON(w, news, http.StatusOK)
}

func (api *Api) handleNewsExists(w http.ResponseWriter, r *http.Request) {
	if !httputils.ValidateMethod(w, r, http.MethodGet, http.MethodOptions) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	params, err := parseURLParams(r.URL.String())
	if err != nil {
		httputils.RenderError(w, "failed to parse query parameters from URL", http.StatusBadRequest, err)
		return
	}
	newsIDStr, exist := params["newsID"]
	if !exist {
		httputils.RenderError(w, "news ID parameter not found in URL", http.StatusBadRequest)
		return
	}
	newsID, err := strconv.Atoi(newsIDStr)
	if err != nil {
		httputils.RenderError(w, "failed to parse news ID parameter", http.StatusBadRequest, err)
		return
	}
	exists, err := api.db.NewsExists(ctx, newsID)
	if err != nil {
		httputils.RenderError(w, "failed to check news existance in database", http.StatusInternalServerError, err)
		return
	}
	if !exists {
		httputils.RenderJSON(w, "news not found", http.StatusNotFound)
		return
	}
	if exists {
		httputils.RenderJSON(w, "news found", http.StatusOK)
	}
}

// parseURLParams извлекаетпараметры из URL
func parseURLParams(input string) (map[string]string, error) {
	parts := strings.Split(input, "?")
	if len(parts) < 2 {
		return nil, fmt.Errorf("no query parameters found")
	}

	values, err := url.ParseQuery(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to parse query parameters: %v", err)
	}

	params := make(map[string]string)
	for key, value := range values {
		if len(value) > 0 {
			params[key] = value[0]
		}
	}

	return params, nil
}
