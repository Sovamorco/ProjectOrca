package main

import (
	"regexp"
)

var urlRx = regexp.MustCompile(`https?://(?:www\.)?.+`)

func atMostAbs[T int | float32 | float64](num, clamp T) T {
	if num < 0 {
		return max(num, -clamp)
	}
	return min(num, clamp)
}

func empty[T any](c chan T) {
	for len(c) != 0 {
		select {
		case <-c:
		default:
		}
	}
}
