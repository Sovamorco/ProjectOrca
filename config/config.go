package config

import (
	"context"

	"ProjectOrca/store"

	"github.com/sovamorco/errorx"
	"github.com/sovamorco/gommon/config"
)

const (
	configNameDev = "config.dev.yaml"
)

type Youtube struct {
	Cookies string `mapstructure:"cookies"`
}

type Spotify struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
}

type VK struct {
	Token string `mapstructure:"token"`
}

type YandexMusic struct {
	Token string `mapstructure:"token"`
}

type Config struct {
	Port         int  `mapstructure:"port"`
	UseDevLogger bool `mapstructure:"use_dev_logger"`

	Youtube     Youtube           `mapstructure:"youtube"`
	Spotify     *Spotify          `mapstructure:"spotify"`
	VK          *VK               `mapstructure:"vk"`
	YandexMusic *YandexMusic      `mapstructure:"yandex_music"`
	DB          store.DBConfig    `mapstructure:"db"`
	Redis       store.RedisConfig `mapstructure:"redis"`
}

func LoadConfig(ctx context.Context) (*Config, error) {
	var res Config

	err := config.LoadConfig(ctx, configNameDev, &res)
	if err != nil {
		return nil, errorx.Decorate(err, "load config")
	}

	return &res, nil
}
