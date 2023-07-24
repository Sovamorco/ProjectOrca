package store

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"sync"

	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/redis/go-redis/v9"
	"github.com/uptrace/bun"
	"go.uber.org/zap"
)

type Config struct {
	DB     DBConfig
	Broker RedisConfig
}

type DBConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DB       string `json:"db"`
	SSL      bool   `json:"ssl"`
}

func (s *DBConfig) getConnString() string {
	sslmode := "disable"
	if s.SSL {
		sslmode = "enable"
	}

	return fmt.Sprintf(
		"postgres://%s:%s@%s/%s?sslmode=%s",
		s.User,
		s.Password,
		net.JoinHostPort(s.Host, fmt.Sprint(s.Port)),
		s.DB,
		sslmode,
	)
}

type RedisConfig struct {
	Host  string `json:"host"`
	Port  int    `json:"port"`
	Token string `json:"token"`
	DB    int    `json:"db"`
}

func (r *RedisConfig) getOptions() *redis.Options {
	return &redis.Options{ //nolint:exhaustruct
		Addr:     fmt.Sprintf("%s:%d", r.Host, r.Port),
		Password: r.Token,
		DB:       r.DB,
	}
}

type Store struct {
	*bun.DB
	*redis.Client
	logger     *zap.SugaredLogger
	unsubFuncs []func(ctx context.Context)
}

func NewStore(logger *zap.SugaredLogger, config *Config) *Store {
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(config.DB.getConnString())))
	db := bun.NewDB(sqldb, pgdialect.New())

	client := redis.NewClient(config.Broker.getOptions())

	return &Store{
		logger:     logger.Named("store"),
		DB:         db,
		Client:     client,
		unsubFuncs: make([]func(ctx context.Context), 0),
	}
}

func (s *Store) GracefulShutdown() {
	err := s.DB.Close()
	if err != nil {
		s.logger.Errorf("Error closing bun store: %+v", err)
	}

	err = s.Client.Close()
	if err != nil {
		s.logger.Errorf("Error closing redis client: %+v", err)
	}
}

func (s *Store) Unsubscribe(ctx context.Context) {
	var wg sync.WaitGroup

	for _, f := range s.unsubFuncs {
		f := f

		wg.Add(1)

		go func() {
			defer wg.Done()
			f(ctx)
		}()
	}

	wg.Wait()
}
