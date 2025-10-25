package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"newsservice/internal/models"
	"newsservice/internal/transport"
	"os"
	"strings"
	"time"

	kfk "github.com/Fau1con/kafkawrapper"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
)

// Config содержит настройки приложения
type Config struct {
	RSSsources []string `json:"source"`
	Interval   int      `json:"interval"`
	Brokers    []string `json:"brokers"`
	Topic      []string `json:"topic"`
}

// Run запускает приложение Newsservice
func Run() error {
	ctxmain := context.Background()

	// Подключение к новостной БД
	pool, err := postgres.New()
	if err != nil {
		log.Printf("Error DB connection: %v", err)
		return err
	}
	defer pool.DB.Close()

	// Инициализация API
	apiInstance := api.New(pool)

	// Инициализация Kafka клиентов
	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	if kafkaBrokers == "" {
		kafkaBrokers = "kafka:9093"
	}
	consumer, err := kfk.NewConsumer([]string{kafkaBrokers}, "news_input")
	if err != nil {
		log.Printf("Kafka consumer creating error: %v\n", err)
		return err
	}
	producer, err := kfk.NewProducer([]string{kafkaBrokers})
	log.Printf("Producer created! Broker: %v", kafkaBrokers)
	if err != nil {
		log.Printf("Kafka creating producer error: %v\n", err)
		return err
	}

	// Каналы для обработки новостей
	newsStream := make(chan []models.NewsFullDetailed)
	errorStream := make(chan error)

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Запуск парсеров RSS
	for _, source := range config.RSSsources {
		go asynParser(ctxmain, source, pool, newsStream, errorStream, config.Interval)
	}

	// Горутина для добавления новостей в БД
	go func() {
		for new := range errorStream {
			log.Println("Error:", err)
		}
	}()

	// Горутина для обработки Kafka сообщений
	go func() {
		for {
			log.Println("Start getting message and redirecting")
			msg, err := consumer.GetMessages(ctxmain)
			if err != nil {
				log.Printf("Failed to read message fron Kafka: %v\n", err)
			}
			data, err := sendRequestToLocalhost(string(msg.Value))
			if err != nil {
				log.Printf("Failed to read data from Kafka message: %v\n", err)
			}
			// Маршрутизация по типам запросов
			if strings.Contains(string(msg.Value), "/newsdetail") {
				err := producer.SendMessage(ctxmain, config.Topic[1], data)
				if err != nil {
					log.Printf("Failedto write message to Kafka: %v\n", err)
					return
				}
			}
			if strings.Contains(string(msg.Value), "/newslist/?n=") {
				err := producer.SendMessage(ctxmain, config.Topic[2], data)
				if err != nil {
					log.Printf("Failed to write message to Kafka: %v\n", err)
					return
				}
			}
			if strings.Contains(string(msg.Value), "/newslist/filtered/?category=") {
				err := producer.SendMessage(ctxmain, config.Topic[3], data)
				if err != nil {
					log.Printf("Failed to write message to Kafka: %v\n", err)
					return
				}
			}
			if strings.Contains(string(msg.Value), "newslist/filtered/date/?date=") {
				err := producer.SendMessage(ctxmain, config.Topic[4], data)
				if err != nil {
					log.Printf("Failed to write message to Kafka: %v\n", err)
					return
				}
			}
		}
	}()

	// Настройка роутера и middleware
	var handler http.Handler = apiInstance.Router()
	handler = transport.RequestIDMiddleware(handler)
	handler = transport.LoggingMiddleware(log)(handler)

	err = godotenv.Load()
	if err != nil {
		log.Fatalf("Failed to load .env file")
		return err
	}
	port := os.Getenv("PORT")

	log.Printf("Server newsservice APP start working at port %v\n", port)
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

	body, err := io.ReadAll(resp.Body) // вместо ioutil.ReadAll Стоит ли ограничить разер файла?
	if err != nil {
		log.Printf("Failed to read response: %v\n", err)
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("Unexpected response code: %d\n", resp.StatusCode)
		// Нужен ли return?
	}
	return body, nil
}
