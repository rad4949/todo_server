package config

import (
	"os"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	ServerPort string `env:"SERVER_PORT" envDefault:"8080"`

	DBHost     string `env:"DB_HOST" envDefault:"localhost"`
	DBPort     string `env:"DB_PORT" envDefault:"5432"`
	DBUser     string `env:"DB_USER,required"`
	DBPassword string `env:"DB_PASSWORD,required"`
	DBName     string `env:"DB_NAME" envDefault:"todo"`
	DBSSLMode  string `env:"DB_SSLMODE" envDefault:"disable"`

	JWTSecret        string `env:"JWT_SECRET,required"`
	JWTRefreshSecret string `env:"JWT_REFRESH_SECRET,required"`

	RedisHost string `env:"REDIS_HOST" envDefault:"localhost"`
	RedisPort string `env:"REDIS_PORT" envDefault:"6379"`

	GRPCPort string `env:"GRPC_PORT" envDefault:"50051"`

	KafkaBrokers []string `env:"KAFKA_BROKERS" envSeparator:"," envDefault:"localhost:9092"`
	KafkaTopic   string   `env:"KAFKA_TOPIC" envDefault:"todo.events.v2"`

	OutboxWorkerID       string        `env:"OUTBOX_WORKER_ID" envDefault:"todo-worker-1"`
	OutboxBatchSize      int           `env:"OUTBOX_BATCH_SIZE" envDefault:"10"`
	OutboxPollInterval   time.Duration `env:"OUTBOX_POLL_INTERVAL" envDefault:"2s"`
	OutboxLockTimeout    time.Duration `env:"OUTBOX_LOCK_TIMEOUT" envDefault:"1m"`
	OutboxMaxAttempts    int           `env:"OUTBOX_MAX_ATTEMPTS" envDefault:"5"`
	OutboxRetryBaseDelay time.Duration `env:"OUTBOX_RETRY_BASE_DELAY" envDefault:"5s"`
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := Config{}

	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
