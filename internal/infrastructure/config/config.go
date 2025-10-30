package config

import (
	"fmt"
	"log"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type AppConfig struct {
	Name               string    `yaml:"name"`
	ReadTimeout        int       `yaml:"read_timeout"`
	WriteTimeout       int       `yaml:"writetimeout"`
	ConnectTimeout     int       `yaml:"connect_timeout"`
	ProcessingInterval int       `yaml:"processing_interval"`
	FeedURLs           []FeedURL `yaml:"feed_urls"`
}

type FeedURL struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

type HTTPConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type DBConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	UserName string `yaml:"username"`
	Password int    `yaml:"password"`
	DBName   string `yaml:"db_name"`
	SSLMode  string `yaml:"sslmode"`
}

type KafkaTopics struct {
	NewsInput string `yaml:"news_input"`

	NewsDetail      string `yaml:"news_detail"`
	NewsList        string `yaml:"news_list"`
	FilteredContent string `yaml:"filtered_content"`
	FilterPublished string `yaml:"filter_published"`
}

type KafkaConfig struct {
	Brokers        []string          `yaml:"brokers"`
	Topics         KafkaTopics       `yaml:"topics"`
	ConsumerGroups map[string]string `yaml:"consumer_groups"`
}

type Route struct {
	Name    string `yaml:"name"`
	BaseURL string `yaml:"base_url"`
}

type Config struct {
	App     AppConfig     `yaml:"app"`
	HTTP    HTTPConfig    `yaml:"http"`
	Logging LoggingConfig `yaml:"logging"`
	DB      DBConfig      `yaml:"db"`
	Kafka   KafkaConfig   `yaml:"kafka"`
	Routes  []Route       `yaml:"routes"`
}

func (c *Config) GetAppName() string {
	return c.App.Name
}

func (c *Config) GetAppProcesingInterval() time.Duration {
	return time.Duration(c.App.ProcessingInterval) * time.Second // В чем удобнее возвращать(time.Duration или int)?
}

func (c *Config) GetHTTPHost() string {
	return c.HTTP.Host
}

func (c *Config) GetHTTPPort() int {
	return c.HTTP.Port
}

func LoadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		log.Println("Config file is empty")
		return nil, fmt.Errorf("config file is empty")
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		log.Print("failed to read config file")
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	expanded := os.ExpandEnv(string(raw))

	var cfg Config

	if err = yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		log.Println("Failed to parse config yaml")
		return nil, fmt.Errorf("failed to parse config yaml: %w", err)
	}

	return &cfg, nil
}
