package store

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/go-redsync/redsync/v4/redis/goredis/v9"

	"github.com/go-redsync/redsync/v4"
	"github.com/redis/go-redis/v9"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

const (
	lockTries = 1

	lockExpiry          = 15 * time.Second
	lockExtendFrequency = lockExpiry - 3*time.Second
)

type Config struct {
	DB     DBConfig
	Broker RedisConfig
}

type DBConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	DB       string `mapstructure:"db"`
	SSL      bool   `mapstructure:"ssl"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Prefix   string `mapstructure:"prefix"`
}

func (r *RedisConfig) getOptions() *redis.Options {
	return &redis.Options{ //nolint:exhaustruct
		Addr:     net.JoinHostPort(r.Host, strconv.Itoa(r.Port)),
		Username: r.Username,
		Password: r.Password,
	}
}

type Store struct {
	*bun.DB
	*redis.Client
	shutdownFuncs    []func(ctx context.Context)
	unsubscribeFuncs []func(ctx context.Context)
	rs               *redsync.Redsync
	RedisPrefix      string
}

func NewStore(config *Config) *Store {
	sqldb := sql.OpenDB(pgdriver.NewConnector(
		pgdriver.WithAddr(fmt.Sprintf("%s:%d", config.DB.Host, config.DB.Port)),
		pgdriver.WithDatabase(config.DB.DB),
		pgdriver.WithInsecure(!config.DB.SSL),
		pgdriver.WithUser(config.DB.Username),
		pgdriver.WithPassword(config.DB.Password),
		pgdriver.WithApplicationName("orca"),
	))

	db := bun.NewDB(sqldb, pgdialect.New())

	client := redis.NewClient(config.Broker.getOptions())
	pool := goredis.NewPool(client)

	return &Store{
		DB:               db,
		Client:           client,
		shutdownFuncs:    make([]func(ctx context.Context), 0),
		unsubscribeFuncs: make([]func(ctx context.Context), 0),
		rs:               redsync.New(pool),
		RedisPrefix:      config.Broker.Prefix,
	}
}

func (s *Store) Shutdown(ctx context.Context) {
	logger := zerolog.Ctx(ctx)

	logger.Info().Msg("Shutting down")

	s.doShutdownFuncs(ctx)

	err := s.DB.Close()
	if err != nil {
		logger.Error().Err(err).Msg("Error closing bun store")
	}

	err = s.Client.Close()
	if err != nil {
		logger.Error().Err(err).Msg("Error closing redis client")
	}
}

func (s *Store) Unsubscribe(ctx context.Context) {
	logger := zerolog.Ctx(ctx)
	logger.Info().Msg("Unsubscribing")

	var wg sync.WaitGroup

	for _, f := range s.unsubscribeFuncs {
		wg.Add(1)

		go func() {
			defer wg.Done()
			f(ctx)
		}()
	}

	wg.Wait()
}

func (s *Store) doShutdownFuncs(ctx context.Context) {
	var wg sync.WaitGroup

	for _, f := range s.shutdownFuncs {
		wg.Add(1)

		go func() {
			defer wg.Done()

			f(ctx)
		}()
	}

	wg.Wait()
}
