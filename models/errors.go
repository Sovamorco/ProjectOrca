package models

import "errors"

var (
	ErrEmptyQueue      = errors.New("queue is empty")
	ErrNoTrack         = errors.New("no track")
	ErrNoVC            = errors.New("no voice connection")
	ErrBrokenStreamURL = errors.New("broken stream url")
	ErrCMDStuck        = errors.New("command stuck")
)
