package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/lib/pq"
	"github.com/rs/zerolog"

	"github.com/hashicorp/vault-client-go"
	"github.com/joomcode/errorx"
	"github.com/mitchellh/mapstructure"

	"github.com/go-redsync/redsync/v4/redis/goredis/v9"

	"github.com/go-redsync/redsync/v4"
	"github.com/redis/go-redis/v9"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

const (
	lockTries = 1

	lockExpiry          = 15 * time.Second
	lockExtendFrequency = lockExpiry - 3*time.Second

	databaseMountPath = "database"
)

type Config struct {
	DB     DBConfig
	Broker RedisConfig
}

type DBConfig struct {
	// these are necessary fields
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
	DB   string `mapstructure:"db"`
	SSL  bool   `mapstructure:"ssl"`

	// and you either have to supply those
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`

	// or RoleName for vault db credentials
	RoleName string `mapstructure:"role_name"`
}

func (s *DBConfig) getConnString() string {
	sslmode := "disable"
	if s.SSL {
		sslmode = "enable"
	}

	return fmt.Sprintf(
		"postgres://%s:%s@%s/%s?sslmode=%s",
		s.Username,
		s.Password,
		net.JoinHostPort(s.Host, strconv.Itoa(s.Port)),
		s.DB,
		sslmode,
	)
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

func getDBConfig(ctx context.Context, config *DBConfig, vc *vault.Client) (*DBConfig, error) {
	if config.RoleName == "" {
		return config, nil
	} else if vc == nil {
		return nil, errorx.IllegalArgument.New("no vault client passed to vault-interpolated database config")
	}

	res, err := vc.Secrets.DatabaseGenerateCredentials(ctx, config.RoleName, vault.WithMountPath(databaseMountPath))
	if err != nil {
		return nil, errorx.Decorate(err, "read db role")
	}

	var newConf DBConfig

	err = mapstructure.Decode(res.Data, &newConf)
	if err != nil {
		return nil, errorx.Decorate(err, "load vault role credentials")
	}

	newConf.DB = config.DB
	newConf.Host = config.Host
	newConf.Port = config.Port
	newConf.SSL = config.SSL

	return &newConf, nil
}

func createConnectorWrapper(ctx context.Context, config *DBConfig, vc *vault.Client) CreateConnectorFunc {
	return func() (driver.Connector, error) {
		dbConfig, err := getDBConfig(ctx, config, vc)
		if err != nil {
			return nil, errorx.Decorate(err, "get db connection config")
		}

		c, err := pq.NewConnector(dbConfig.getConnString())
		if err != nil {
			return nil, errorx.Decorate(err, "create new connector")
		}

		return c, nil
	}
}

func NewStore(ctx context.Context, config *Config, vc *vault.Client) *Store {
	sqldb := sql.OpenDB(Driver{CreateConnectorFunc: createConnectorWrapper(context.WithoutCancel(ctx), &config.DB, vc)})
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
