//go:build !uiembed

package uiembed

import (
	"errors"
	"net/http"
)

var errUnavailable = errors.New("embedded ui unavailable: build with -tags uiembed")

func Available() bool {
	return false
}

func NewHandler(_ string) (http.Handler, error) {
	return nil, errUnavailable
}
