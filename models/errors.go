package models

import "errors"

var (
	ErrEmptyQueue   = errors.New("queue is empty")
	ErrShuttingDown = errors.New("shutting down")
	ErrNoTrack      = errors.New("no track")
)
