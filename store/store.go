package store

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/joomcode/errorx"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"go.uber.org/zap"
)

type Config struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DB       string `json:"db"`
}

func (s *Config) getConnString() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", s.User, s.Password, s.Host, s.Port, s.DB)
}

type Store struct {
	*bun.DB
	logger *zap.SugaredLogger
}

func NewStore(logger *zap.SugaredLogger, config *Config) (*Store, error) {
	mysql, err := sql.Open("mysql", config.getConnString())
	if err != nil {
		return nil, errorx.Decorate(err, "open sql connection")
	}
	db := bun.NewDB(mysql, mysqldialect.New())
	return &Store{
		logger: logger.Named("store"),
		DB:     db,
	}, nil
}

func (s *Store) GracefulShutdown() {
	err := s.Close()
	if err != nil {
		s.logger.Errorf("Error closing bun Store: %+v", err)
	}
}
