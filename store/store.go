package store

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/joomcode/errorx"
	"github.com/redis/go-redis/v9"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"go.uber.org/zap"
	"sync"
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
}

func (s *DBConfig) getConnString() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", s.User, s.Password, s.Host, s.Port, s.DB)
}

type RedisConfig struct {
	Host  string `json:"host"`
	Port  int    `json:"port"`
	Token string `json:"token"`
	DB    int    `json:"db"`
}

func (r *RedisConfig) getOptions() *redis.Options {
	return &redis.Options{
		Addr:     fmt.Sprintf("%s:%d", r.Host, r.Port),
		Password: r.Token,
		DB:       r.DB,
	}
}

type Store struct {
	*bun.DB
	*redis.Client
	logger     *zap.SugaredLogger
	unsubFuncs []func()
}

func NewStore(logger *zap.SugaredLogger, config *Config) (*Store, error) {
	mysql, err := sql.Open("mysql", config.DB.getConnString())
	if err != nil {
		return nil, errorx.Decorate(err, "open sql connection")
	}
	db := bun.NewDB(mysql, mysqldialect.New())
	client := redis.NewClient(config.Broker.getOptions())
	return &Store{
		logger: logger.Named("store"),
		DB:     db,
		Client: client,
	}, nil
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

func (s *Store) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	ps := s.Client.Subscribe(ctx, channels...)
	s.unsubFuncs = append(s.unsubFuncs, func() {
		err := ps.Unsubscribe(context.TODO(), channels...)
		if err != nil {
			s.logger.Errorf("Error unsubscribing from channels: %+v", err)
		}
	})
	return ps
}

func (s *Store) Unsubscribe() {
	var wg sync.WaitGroup
	for _, f := range s.unsubFuncs {
		wg.Add(1)
		f := f
		go func() {
			defer wg.Done()
			f()
		}()
	}
	wg.Wait()
}
