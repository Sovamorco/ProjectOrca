package models

import (
	"github.com/uptrace/bun"
)

type RemoteBot struct {
	bun.BaseModel `bun:"table:bots" exhaustruct:"optional"`

	ID     string `bun:",pk"`
	Token  string
	Locker string
}

func NewRemoteBot(id, token, locker string) *RemoteBot {
	return &RemoteBot{
		ID:     id,
		Token:  token,
		Locker: locker,
	}
}
