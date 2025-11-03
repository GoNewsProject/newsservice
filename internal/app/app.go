package app

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"newsservice/api"
	"newsservice/internal/infrastructure/config"
	"newsservice/internal/models"
	"newsservice/internal/rss"
	transport "newsservice/internal/transport/http"
	"newsservice/storage"
	"os"
	"strconv"
	"strings"
	"time"

	kfk "github.com/Fau1con/kafkawrapper"
)

// Run запускает приложение Newsservice
func Run() error {
	ctxmain := context.Background()

	cfg, err := config.LoadConfig("config/dev.yaml")
	if err != nil {
		return fmt.Errorf("failed to loag config: %w", err)
	}

	ctxMain, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug, // cfg.logging.level
	}))

	// Подключение к новостной БД
	pool, err := storage.NewStorage(*cfg, log)
	if err != nil {
		log.Error("Error DB connection", "error", err)
		return err
	}
	defer pool.Close()

	// Инициализация API
	apiInstance := api.NewApi(ctxMain, http.NewServeMux(), pool, log)

	// Инициализация Kafka клиентов
	kafkaBrokers := cfg.Kafka.Brokers
	if len(kafkaBrokers) == 0 {
		kafkaBrokers[0] = "kafka:9093"
	}
	consumer, err := kfk.NewConsumer(kafkaBrokers, "news_input")
	if err != nil {
		log.Error("Kafka consumer creating error",
			slog.Any("%v\n", err))
		return err
	}
	producer, err := kfk.NewProducer(kafkaBrokers)
	log.Info("Producer created! Broker: ",
		slog.Any("%v", kafkaBrokers))
	if err != nil {
		log.Error("Kafka creating producer error",
			slog.Any("%v\n", err))
		return err
	}

	// Каналы для обработки новостей
	newsStream := make(chan []models.NewsFullDetailed)
	errorStream := make(chan error)

	// Запуск парсеров RSS
	for _, source := range cfg.App.FeedURLs {
		go asynParser(source.URL, newsStream, errorStream, int(cfg.GetAppProcesingInterval()))
	}

	// Горутина для добавления новостей в БД
	go func() {
		for new := range newsStream {
			pool.AddNews(ctxMain, new)
		}
	}()

	// Горутина для обработки ошибок
	go func() {
		for err := range errorStream {
			log.Error("news parsing error", "error", err)
		}
	}()

	// Горутина для обработки Kafka сообщений
	go func() {
		for {
			log.Info("Start getting message and redirecting")
			msg, err := consumer.GetMessages(ctxmain)
			if err != nil {
				log.Error("failed to read message fron Kafka",
					slog.Any("%v\n", err))
			}
			data, err := sendRequestToLocalhost(string(msg.Value))
			if err != nil {
				log.Error("failed to read data from Kafka message",
					slog.Any("%v\n", err))
			}
			// Маршрутизация по типам запросов
			if strings.Contains(string(msg.Value), "/newsdetail") {
				err := producer.SendMessage(ctxMain, cfg.Kafka.Topics.NewsDetail, data)
				if err != nil {
					log.Error("failed to write message to Kafka",
						slog.Any("%v\n", err))
					return
				}
			}
			if strings.Contains(string(msg.Value), "/newslist/?n=") {
				err := producer.SendMessage(ctxmain, cfg.Kafka.Topics.NewsList, data)
				if err != nil {
					log.Error("failed to write message to Kafka",
						slog.Any("%v\n", err))
					return
				}
			}
			if strings.Contains(string(msg.Value), "/newslist/filtered/?category=") {
				err := producer.SendMessage(ctxmain, cfg.Kafka.Topics.FilteredContent, data)
				if err != nil {
					log.Error("Failed to write message to Kafka",
						slog.Any("%v\n", err))
					return
				}
			}
			// if strings.Contains(string(msg.Value), "newslist/filtered/date/?date=") {
			// 	err := producer.SendMessage(ctxmain, config.Topic[4], data)
			// 	if err != nil {
			// 		log.Error("Failed to write message to Kafka",
			// 			slog.Any("%v\n", err))
			// 		return
			// 	}
			// }
		}
	}()

	// Настройка роутера и middleware
	var handler http.Handler = apiInstance.Router()
	handler = transport.CORSMiddleware()(handler)
	handler = transport.RequestIDMiddleware(handler)
	handler = transport.LoggingMiddleware(log)(handler)

	log.Info("Server newsservice APP start working at port",
		slog.Any("%v\n", cfg.HTTP.Port))
	return http.ListenAndServe(":"+strconv.Itoa(cfg.HTTP.Port), handler)
}

// asynParser асинхронно обрабатывает RSS-ленты
func asynParser(source string, news chan<- []models.NewsFullDetailed, errs chan<- error, interval int) {
	for {
		rssnews, err := rss.Parse(source)
		if err != nil {
			errs <- err
			continue
		}
		news <- rssnews
		time.Sleep(time.Duration(interval) * time.Minute)
	}
}

// sendRequestToLocalhost выполняет HTTP запрос к локальному сервису
func sendRequestToLocalhost(path string) ([]byte, error) {
	if path == "" {
		return nil, fmt.Errorf("path cannot be empty")
	}
	url := fmt.Sprintf("http://localhost:6000%s", path)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Printf("Failed to create request: %v\n", err)
		return nil, err
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to send request: %v\n", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read response: %v\n", err)
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("Unexpected response code: %d\n", resp.StatusCode)
	}
	return body, nil
}
