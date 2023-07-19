package models

import "errors"

var (
	ErrNotPlaying    = errors.New("nothing playing")
	ErrSeekLive      = errors.New("cannot seek live track")
	ErrNotPaused     = errors.New("not paused")
	ErrAlreadyPaused = errors.New("already paused")
	ErrPaused        = errors.New("playback is paused")
)
