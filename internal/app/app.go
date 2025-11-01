package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"newsservice/internal/infrastructure/config"
	"newsservice/internal/models"
	transport "newsservice/internal/transport/http"
	"newsservice/storage"
	"os"
	"strings"
	"time"

	kfk "github.com/Fau1con/kafkawrapper"
	"github.com/joho/godotenv"
)

// Config содержит настройки приложения
type Config struct {
	RSSsources []string `json:"source"`
	Interval   int      `json:"processing_interval"`
	Brokers    []string `json:"brokers"`
	Topic      []string `json:"topic"`
}

// Run запускает приложение Newsservice
func Run() error {
	ctxmain := context.Background()

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to loag config: %w", err)
	}

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Подключение к новостной БД
	pool, err := storage.NewStorage(*cfg, log)
	if err != nil {
		log.Error("Error DB connection", "error", err)
		return err
	}
	defer pool.Close()

	// Инициализация API
	apiInstance := api.New(pool)

	// Инициализация Kafka клиентов
	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	if kafkaBrokers == "" {
		kafkaBrokers = "kafka:9093"
	}
	consumer, err := kfk.NewConsumer([]string{kafkaBrokers}, "news_input")
	if err != nil {
		log.Error("Kafka consumer creating error",
			slog.Any("%v\n", err))
		return err
	}
	producer, err := kfk.NewProducer([]string{kafkaBrokers})
	log.Info("Producer created! Broker: %v", kafkaBrokers)
	if err != nil {
		log.Error("Kafka creating producer error",
			slog.Any("%v\n", err))
		return err
	}

	// Каналы для обработки новостей
	newsStream := make(chan []models.NewsFullDetailed)
	errorStream := make(chan error)

	// Запуск парсеров RSS
	for _, source := range config.FeedURLs {
		go asynParser(ctxmain, source, pool, newsStream, errorStream, config.Interval)
	}

	// Горутина для добавления новостей в БД
	go func() {
		for new := range errorStream {
			log.Error("Error:", err)
		}
	}()

	// Горутина для обработки Kafka сообщений
	go func() {
		for {
			log.Info("Start getting message and redirecting")
			msg, err := consumer.GetMessages(ctxmain)
			if err != nil {
				log.Error("Failed to read message fron Kafka",
					slog.Any("%v\n", err))
			}
			data, err := sendRequestToLocalhost(string(msg.Value))
			if err != nil {
				log.Error("Failed to read data from Kafka message",
					slog.Any("%v\n", err))
			}
			// Маршрутизация по типам запросов
			if strings.Contains(string(msg.Value), "/newsdetail") {
				err := producer.SendMessage(ctxmain, config.Topic[1], data)
				if err != nil {
					log.Error("Failedto write message to Kafka",
						slog.Any("%v\n", err))
					return
				}
			}
			if strings.Contains(string(msg.Value), "/newslist/?n=") {
				err := producer.SendMessage(ctxmain, config.Topic[2], data)
				if err != nil {
					log.Error("Failed to write message to Kafka",
						slog.Any("%v\n", err))
					return
				}
			}
			if strings.Contains(string(msg.Value), "/newslist/filtered/?category=") {
				err := producer.SendMessage(ctxmain, config.Topic[3], data)
				if err != nil {
					log.Error("Failed to write message to Kafka",
						slog.Any("%v\n", err))
					return
				}
			}
			if strings.Contains(string(msg.Value), "newslist/filtered/date/?date=") {
				err := producer.SendMessage(ctxmain, config.Topic[4], data)
				if err != nil {
					log.Error("Failed to write message to Kafka",
						slog.Any("%v\n", err))
					return
				}
			}
		}
	}()

	// Настройка роутера и middleware
	var handler http.Handler = apiInstance.Router()
	handler = transport.CORSMiddleware()(handler)
	handler = transport.RequestIDMiddleware(handler)
	handler = transport.LoggingMiddleware(log)(handler)

	err = godotenv.Load()
	if err != nil {
		log.Error("Failed to load .env file")
		return err
	}
	port := os.Getenv("PORT")

	log.Info("Server newsservice APP start working at port %v\n", port)
	return http.ListenAndServe(port, handler)
}

// parseConfigFile парсит JSON файл с настройками
func parseConfigFile(filename string) (Config, error) {
	var data []byte
	configFile, err := os.Open(filename)
	if err != nil {
		log.Printf("Failed to open config file: %v\n", err)
		return Config{}, err
	}
	defer configFile.Close()

	data, err = os.ReadFile("./config.json")
	if err != nil {
		log.Fatal(err)
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		log.Printf("Failed to unmarshal config: %v\n", err)
		return Config{}, err
	}
	return config, nil
}

// asynParser асинхронно обрабатывает RSS-ленты
func asynParser(ctx context.Context, source string, db DB.DBInterface, news chan<- []models.NewsFullDetailed, errs chan<- error, interval int) {
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
