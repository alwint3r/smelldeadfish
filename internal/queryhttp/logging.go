package queryhttp

import (
	"log"
	"net/http"
	"time"
)

type Options struct {
	Logger *log.Logger
}

func loggerFromOptions(opts Options) *log.Logger {
	return opts.Logger
}

func logRequestError(logger *log.Logger, handler string, r *http.Request, status int, start time.Time, err error, service string) {
	if logger == nil {
		return
	}
	if status < 400 || status >= 600 {
		return
	}
	errMessage := ""
	if err != nil {
		errMessage = err.Error()
	}
	logger.Printf(
		"msg=request_error handler=%s method=%s path=%s status=%d duration_ms=%d error=%q service=%q",
		handler,
		r.Method,
		r.URL.Path,
		status,
		time.Since(start).Milliseconds(),
		errMessage,
		service,
	)
}
