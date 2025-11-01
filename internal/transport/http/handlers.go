package http

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"newsservice/internal/models"
	"newsservice/internal/pagination"
	"newsservice/storage"
	"strconv"
	"strings"
	"time"

	kfk "github.com/Fau1con/kafkawrapper"
	httputils "github.com/Fau1con/renderresponse"
)

type NewsHandler struct {
	storage  storage.NewsStorage
	producer *kfk.Producer
}

func NewNewsHandler(storage storage.NewsStorage, producer *kfk.Producer) *NewsHandler {
	return &NewsHandler{
		storage:  storage,
		producer: producer,
	}
}

func (h *NewsHandler) HandleDetailedNews(ctx context.Context, c *kfk.Consumer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !httputils.ValidateMethod(w, r, http.MethodGet) {
			h.producer.SendMessage(ctx, "news_detail", []byte("failed to check request method"))
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		msg, err := c.GetMessages(ctx)
		if err != nil {
			h.producer.SendMessage(ctx, "news_detail", []byte("failed to read message from Kafka"))
			httputils.RenderError(w, "failed to read message from Kafka", http.StatusInternalServerError)
			return
		}

		params, err := parseURLParams(string(msg.Value))
		if err != nil {
			h.producer.SendMessage(ctx, "news_detail", []byte("failed to parse URL params"))
			httputils.RenderError(w, "failed to parse URL params", http.StatusInternalServerError)
			return
		}

		newsIDStr, exists := params["newsID"]
		if !exists || newsIDStr == "" {
			h.producer.SendMessage(ctx, "news_detail", []byte("news ID parameter is required"))
			httputils.RenderError(w, "news ID parameter is required", http.StatusBadRequest)
			return
		}
		newsID, err := strconv.Atoi(newsIDStr)
		if err != nil {
			h.producer.SendMessage(ctx, "news_detail", []byte("failed to parse news ID parameter"))
			httputils.RenderError(w, "failed to parse news ID parameter", http.StatusBadRequest)
			return
		}

		news, err := h.storage.GetDetailedNews(ctx, newsID)
		if err != nil {
			h.producer.SendMessage(ctx, "news_detail", []byte("failed to get news from database"))
			httputils.RenderError(w, "failed to get news from database", http.StatusInternalServerError)
			return
		}

		h.producer.SendMessage(ctx, "news_detail", []byte(fmt.Sprintf("Retrieved news ID %d", newsID)))
		httputils.RenderJSON(w, news, http.StatusOK)
	}
}

func (h *NewsHandler) HandleGetNewsList(ctx context.Context, c *kfk.Consumer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !httputils.ValidateMethod(w, r, http.MethodGet) {
			h.producer.SendMessage(ctx, "news_list", []byte("failed to check request method"))
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		msg, err := c.GetMessages(ctx)
		if err != nil {
			h.producer.SendMessage(ctx, "news_list", []byte("failed to read message from Kafka"))
			httputils.RenderError(w, "failed to read message from Kafka", http.StatusInternalServerError)
			return
		}

		params, err := parseURLParams(string(msg.Value))
		if err != nil {
			h.producer.SendMessage(ctx, "news_list", []byte("failed to parse URL params"))
			httputils.RenderError(w, "failed to parse URL params", http.StatusInternalServerError)
			return
		}

		pageStr, exists := params["page"]
		if !exists || pageStr == "" {
			h.producer.SendMessage(ctx, "news_list", []byte("page parameter is required"))
			httputils.RenderError(w, "page parameter is required", http.StatusBadRequest)
			return
		}
		page, err := strconv.Atoi(pageStr)
		if err != nil {
			h.producer.SendMessage(ctx, "news_list", []byte("failed to parse page parameter"))
			httputils.RenderError(w, "failed to parse page parameter", http.StatusBadRequest)
			return
		}

		if page < 1 {
			page = 1
		}

		filter := models.NewsFilter{
			Limit:  pagination.NEWS_PER_PAGE,
			Offset: (page - 1) * pagination.NEWS_PER_PAGE,
		}

		totalResult, err := h.storage.GetNewsCount(ctx, filter)
		if err != nil {
			h.producer.SendMessage(ctx, "news_list", []byte("failed to get news count"))
			httputils.RenderError(w, "failed to get news count", http.StatusInternalServerError)
			return
		}

		paginator := pagination.New(totalResult, page)
		if err := paginator.Validate(); err != nil {
			h.producer.SendMessage(ctx, "news_list", []byte("pagination validation failed"))
			httputils.RenderError(w, "pagination validation failed", http.StatusBadRequest)
			return
		}

		news, err := h.storage.GetNewsByFilter(ctx, filter)
		if err != nil {
			h.producer.SendMessage(ctx, "news_list", []byte("failed to get news from database"))
			httputils.RenderError(w, "failed to get news from database", http.StatusInternalServerError)
			return
		}

		paginator.SetResults(news)

		h.producer.SendMessage(ctx, "news_list", []byte(fmt.Sprintf(
			"Retrieved %d news on page %d/%d",
			len(news), paginator.CurrentPage, paginator.TotalPages,
		)))
		httputils.RenderJSON(w, paginator, http.StatusOK)
	}
}

func (h *NewsHandler) HandleFilterNewsByContent(ctx context.Context, c *kfk.Consumer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !httputils.ValidateMethod(w, r, http.MethodGet) {
			h.producer.SendMessage(ctx, "filtered_content", []byte("failed to check request method"))
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		msg, err := c.GetMessages(ctx)
		if err != nil {
			h.producer.SendMessage(ctx, "filtered_content", []byte("failed to read message from Kafka"))
			httputils.RenderError(w, "failed to read message from Kafka", http.StatusInternalServerError)
			return
		}

		params, err := parseURLParams(string(msg.Value))
		if err != nil {
			h.producer.SendMessage(ctx, "filtered_content", []byte("failed to parse URL params"))
			httputils.RenderError(w, "failed to parse URL params", http.StatusInternalServerError)
			return
		}

		pageStr := params["page"]
		limitStr := params["limit"]
		category := params["category"]
		author := params["author"]
		dateStr := params["date"]

		page, err := strconv.Atoi(pageStr)
		if err != nil {
			h.producer.SendMessage(ctx, "filtered_content", []byte("failed to parse page parameter"))
			httputils.RenderError(w, "failed to parse page parameter", http.StatusBadRequest)
		}

		limitStr, exists := params["limit"]
		if !exists || limitStr == "" {
			h.producer.SendMessage(ctx, "filtered_content", []byte("limit parameter is required"))
			httputils.RenderError(w, "limit parameter is required", http.StatusBadRequest)
		}
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			h.producer.SendMessage(ctx, "filtered_content", []byte("failed to parse limit parameter"))
			httputils.RenderError(w, "failed to parse limit parameter", http.StatusBadRequest)
		}

		dateStr, exists = params["date"]
		if !exists || dateStr == "" {
			h.producer.SendMessage(ctx, "filtered_content", []byte("date parameter is required"))
			httputils.RenderError(w, "date parameter is required", http.StatusBadRequest)
		}
		parsedDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			h.producer.SendMessage(ctx, "filtered_content", []byte("invalid date format, expected YYYY-MM-DD"))
			httputils.RenderError(w, "invalid date format, expected YYYY-MM-DD", http.StatusBadRequest)
			return
		}

		filter := models.NewsFilter{
			Category: category,
			Author:   author,
			Date:     parsedDate,
			Limit:    limit,
			Offset:   (page - 1) * limit,
		}

		totalResult, err := h.storage.GetNewsCount(ctx, filter)
		if err != nil {
			h.producer.SendMessage(ctx, "filtered_content", []byte("failed to get news count"))
			httputils.RenderError(w, "failed to get news count", http.StatusInternalServerError)
			return
		}

		paginator := pagination.New(totalResult, page)
		if err := paginator.Validate(); err != nil {
			h.producer.SendMessage(ctx, "filtered_content", []byte("pagination validation failed"))
			httputils.RenderError(w, "pagination validation failed", http.StatusBadRequest)
			return
		}

		news, err := h.storage.GetNewsByFilter(ctx, filter)
		if err != nil {
			h.producer.SendMessage(ctx, "filtered_content", []byte("failed to get news from database"))
			httputils.RenderError(w, "failed to get news from database", http.StatusInternalServerError)
			return
		}

		paginator.SetResults(news)

		h.producer.SendMessage(ctx, "filtered_content", []byte(fmt.Sprintf(
			"Retrieved %d news on page %d/%d",
			len(news), paginator.CurrentPage, paginator.TotalPages,
		)))

		httputils.RenderJSON(w, paginator, http.StatusOK)
	}
}

func parseURLParams(input string) (map[string]string, error) {
	parts := strings.Split(input, "?")
	if len(parts) < 2 {
		return nil, fmt.Errorf("no query parameters found")
	}

	values, err := url.ParseQuery(parts[1])
	if err != nil {
		return nil, err
	}

	params := make(map[string]string)
	for key, value := range values {
		if len(value) > 0 {
			params[key] = value[0]
		}
	}

	return params, nil
}

// func (h *NewsHandler) HandleFilterNewsByPublished(ctx context.Context, c *kfk.Consumer) http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		if !httputils.ValidateMethod(w, r, http.MethodGet) {
// 			h.producer.SendMessage(ctx, "filter_published", []byte("failed to check request method"))
// 			return
// 		}

// 		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
// 		defer cancel()

// 		msg, err := c.GetMessages(ctx)
// 		if err != nil {
// 			h.producer.SendMessage(ctx, "filter_published", []byte("failed to read message from Kafka"))
// 			httputils.RenderError(w, "failed to read message from Kafka", http.StatusInternalServerError)
// 			return
// 		}
// 		params, err := parseURLParams(string(msg.Value))
// 		if err != nil {
// 			h.producer.SendMessage(ctx, "filter_published", []byte("failed to parse URL params"))
// 			httputils.RenderError(w, "failed to parse URL params", http.StatusInternalServerError)
// 			return
// 		}

// 		dateStr, exists := params["date"]
// 		if !exists || dateStr == "" {
// 			h.producer.SendMessage(ctx, "filter_published", []byte("date parameter is required"))
// 			httputils.RenderError(w, "date parameter is required", http.StatusBadRequest)
// 			return
// 		}
// 		date, err := strconv.Atoi(dateStr)
// 		if err != nil {
// 			h.producer.SendMessage(ctx, "filter_published", []byte("failed to parse date parameter"))
// 			httputils.RenderError(w, "failed to parse date parameter", http.StatusBadRequest)
// 			return
// 		}

// 		limitStr := params["limit"]
// 		limit := 10
// 		if limitStr != "" {
// 			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
// 				limit = l
// 			} else {
// 				h.producer.SendMessage(ctx, "filter_published", []byte("invalid limit parameter"))
// 				httputils.RenderError(w, "invalid limit parameter", http.StatusBadRequest)
// 				return
// 			}
// 		}
// 		offsetStr := params["offset"]
// 		offset := 0
// 		if offsetStr != "" {
// 			if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
// 				offset = o
// 			} else {
// 				h.producer.SendMessage(ctx, "filter_published", []byte("invalid offset parameter"))
// 				httputils.RenderError(w, "invalid offset parameter", http.StatusBadRequest)
// 				return
// 			}
// 		}
// 		news, err := h.storage.FilterNewsByDateWithPagination(ctx, date, offset, limit)
// 		if err != nil {
// 			h.producer.SendMessage(ctx, "filter_published", []byte("failed to get news fron database"))
// 			httputils.RenderError(w, "failed to get news fron database", http.StatusInternalServerError)
// 			return
// 		}

// 		respResult, err := newsToBytes(news)
// 		if err != nil {
// 			h.producer.SendMessage(ctx, "filter_published", []byte("failed to parse response"))
// 			httputils.RenderError(w, "failed to parse response", http.StatusInternalServerError)
// 			return
// 		}

// 		h.producer.SendMessage(ctx, "filter_published", []byte(respResult))
// 		httputils.RenderJSON(w, respResult, http.StatusOK)
// 	}
// }

// ----------------------------------------------------------------------------------------------------------------------------
// func (api *Api) GetDetailedNewsHandler(w http.ResponseWriter, r *http.Request) {
// 	if !httputils.ValidateMethod(w, r, http.MethodGet, http.MethodOptions) {
// 		return
// 	}

// 	newsIDStr := r.URL.Query().Get("id")
// 	if newsIDStr == "" {
// 		httputils.RenderError(w, "Invalid newsID parameter", http.StatusBadRequest)
// 		return
// 	}
// 	newsID, err := strconv.Atoi(newsIDStr)
// 	if err != nil {
// 		httputils.RenderError(w, "Invalid newsID format", http.StatusBadRequest)
// 		return
// 	}

// 	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
// 	defer cancel()

// 	detailPost, err := api.db.GetDetailedNews(ctx, newsID)
// 	if err != nil {
// 		httputils.RenderError(w, "Failed to get detailed post from database", http.StatusInternalServerError)
// 		return
// 	}

// 	httputils.RenderJSON(w, detailPost, http.StatusOK)
// }

// func (api *Api) GetNewsListHandler(w http.ResponseWriter, r *http.Request) {
// 	if !httputils.ValidateMethod(w, r, http.MethodGet, http.MethodOptions) {
// 		return
// 	}

// 	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
// 	defer cancel()

// 	result, err := api.db.GetNewsList(ctx)
// 	if err != nil {
// 		http.Error(w, "No news in database", http.StatusOK) // Возможно ли использование RenderError?
// 	}

// 	httputils.RenderJSON(w, result, http.StatusOK)
// }

// func (api *Api) FilterNewsByContentHandler(w http.ResponseWriter, r *http.Request) {
// 	if !httputils.ValidateMethod(w, r, http.MethodGet, http.MethodOptions) {
// 		return
// 	}

// 	pageStr := r.URL.Query().Get("page")
// 	if pageStr == "" {
// 		pageStr = "1"
// 	}
// 	page, _ := strconv.Atoi(pageStr)

// 	filter := r.URL.Query().Get("filter")
// 	if filter == "" {
// 		httputils.RenderError(w, "Unknown filter", http.StatusBadRequest)
// 	}

// 	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
// 	defer cancel()

// 	filteredData, err := api.db.FilterNewsByContent(ctx, filter)
// 	if err != nil {
// 		httputils.RenderError(w, "Failed to get filter data", http.StatusInternalServerError)
// 		return
// 	}
// 	pag := pagination.New(len(filteredData), page)
// 	results, err := api.db.FilterNewsByContentWithPagination(ctx, filter, (pag.CurrentPage-1)*pag.NewsPerPage, pag.NewsPerPage)
// 	if err != nil {
// 		httputils.RenderError(w, "Failed to get filtered by content news with pagination", http.StatusInternalServerError)
// 	}
// 	pag.Results = results

// 	httputils.RenderJSON(w, pag.Results, http.StatusOK)
// }

// func (api *Api) FilterNewsByPublishedHandler(w http.ResponseWriter, r *http.Request) {
// 	if !httputils.ValidateMethod(w, r, http.MethodGet, http.MethodOptions) {
// 		return
// 	}

// 	dateStr := r.URL.Query().Get("date")
// 	if dateStr == "" {
// 		httputils.RenderError(w, "Invalid data patameter", http.StatusBadRequest)
// 		return
// 	}
// 	date, err := strconv.Atoi(dateStr) // Или нужно привести к TIMESTAMP?
// 	if err != nil {
// 		httputils.RenderError(w, "Invalid date parameter", http.StatusBadRequest)
// 		return
// 	}

// 	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
// 	defer cancel()

// 	filteredData, err := api.db.FilterNewsByDate(ctx, date)
// 	if err != nil {
// 		httputils.RenderError(w, "Failed to get filtered data", http.StatusInternalServerError)
// 		return
// 	}
// 	page := 1

// 	pag := pagination.New(len(filteredData), page)
// 	results, err := api.db.FilterNewsByDateWithPagination(ctx, date, (pag.CurrentPage-1)*pag.NewsPerPage, pag.NewsPerPage)
// 	if err != nil {
// 		httputils.RenderError(w, "Failed to get filtered by date news with pagination", http.StatusInternalServerError)
// 		return
// 	}
// 	pag.Results = results

// 	httputils.RenderJSON(w, pag.Results, http.StatusOK)

// }
