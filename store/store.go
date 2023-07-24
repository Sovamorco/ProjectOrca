package store

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/joomcode/errorx"
	"net"
	"sync"
	"time"

	"github.com/go-redsync/redsync/v4/redis/goredis/v9"

	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/go-redsync/redsync/v4"
	"github.com/redis/go-redis/v9"
	"github.com/uptrace/bun"
	"go.uber.org/zap"
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
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	DB       int    `json:"db"`
}

func (r *RedisConfig) getOptions() *redis.Options {
	return &redis.Options{ //nolint:exhaustruct
		Addr:     net.JoinHostPort(r.Host, fmt.Sprint(r.Port)),
		Username: r.Username,
		Password: r.Password,
		DB:       r.DB,
	}
}

type Store struct {
	*bun.DB
	*redis.Client
	logger           *zap.SugaredLogger
	shutdownFuncs    []func(ctx context.Context)
	unsubscribeFuncs []func(ctx context.Context)
	rs               *redsync.Redsync
}

func NewStore(logger *zap.SugaredLogger, config *Config) *Store {
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(config.DB.getConnString())))
	db := bun.NewDB(sqldb, pgdialect.New())

	client := redis.NewClient(config.Broker.getOptions())
	pool := goredis.NewPool(client)

	return &Store{
		logger:           logger.Named("store"),
		DB:               db,
		Client:           client,
		shutdownFuncs:    make([]func(ctx context.Context), 0),
		unsubscribeFuncs: make([]func(ctx context.Context), 0),
		rs:               redsync.New(pool),
	}
}

func (s *Store) GracefulShutdown(ctx context.Context) {
	s.doShutdownFuncs(ctx)

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

	for _, f := range s.unsubscribeFuncs {
		f := f

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
		f := f

		wg.Add(1)

		go func() {
			defer wg.Done()
			f(ctx)
		}()
	}

	wg.Wait()
}

func (s *Store) Lock(ctx context.Context, key string) error {
	mu := s.rs.NewMutex(key, redsync.WithExpiry(lockExpiry), redsync.WithTries(lockTries))

	err := mu.LockContext(ctx)
	if err != nil {
		return errorx.Decorate(err, "lock")
	}

	cancel := make(chan struct{}, 1)

	go s.extendLoop(ctx, mu, cancel)

	s.shutdownFuncs = append(s.shutdownFuncs, func(ctx context.Context) {
		// make sure this does not lock
		select {
		case cancel <- struct{}{}:
		default:
		}

		_, err := mu.UnlockContext(ctx)
		if err != nil {
			s.logger.Errorf("Error unlocking state: %+v", err)
		}
	})

	return nil
}

func (s *Store) extendLoop(ctx context.Context, mu *redsync.Mutex, cancel chan struct{}) {
	ticker := time.NewTicker(lockExtendFrequency)
	defer ticker.Stop()

	for {
		_, err := mu.ExtendContext(ctx)
		if err != nil {
			s.logger.Errorf("Error extending lock: %+v", err)
		}

		select {
		case <-cancel:
			return
		case <-ticker.C:
		}
	}
}
