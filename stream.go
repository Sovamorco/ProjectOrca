package main

import (
	"github.com/jfbus/httprs"
	"github.com/pkg/errors"
	"io"
	"net/http"
)

func streamFromURL(url string) (io.ReadSeeker, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, errors.Wrap(err, "http get")
	}
	return httprs.NewHttpReadSeeker(resp), nil
}
