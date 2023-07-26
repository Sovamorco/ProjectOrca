package main

import (
	"context"

	"ProjectOrca/store"

	"github.com/hashicorp/vault-client-go"
	"github.com/joomcode/errorx"
	"github.com/sovamorco/gommon/config"
)

const (
	configName    = "config.yaml"
	configNameDev = "config.dev.yaml"
)

type Spotify struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
}

type Config struct {
	Port int `mapstructure:"port"`

	Spotify *Spotify          `mapstructure:"spotify"`
	DB      store.DBConfig    `mapstructure:"db"`
	Redis   store.RedisConfig `mapstructure:"redis"`
}

func loadConfigDev(ctx context.Context) (*Config, error) {
	var res Config

	err := config.LoadConfig(ctx, configNameDev, &res)
	if err != nil {
		return nil, errorx.Decorate(err, "load config")
	}

	return &res, nil
}

func loadConfig(ctx context.Context, vc *vault.Client) (*Config, error) {
	var res Config

	err := config.LoadConfigVault(ctx, vc, configName, &res)
	if err != nil {
		return nil, errorx.Decorate(err, "load config")
	}

	return &res, nil
}
