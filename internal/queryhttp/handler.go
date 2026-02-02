package queryhttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"deadfish/internal/spanstore"
)

const spansPath = "/api/spans"

const defaultLimit = 100

type Handler struct {
	store spanstore.Store
}

type SpansResponse struct {
	Spans []spanstore.Span `json:"spans"`
}

func NewHandler(store spanstore.Store) http.Handler {
	return &Handler{store: store}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != spansPath {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	params, err := parseQueryParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	spans, err := h.store.QuerySpans(r.Context(), params)
	if err != nil {
		http.Error(w, "failed to query spans", http.StatusInternalServerError)
		return
	}
	resp := SpansResponse{Spans: spans}
	payload, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

func parseQueryParams(r *http.Request) (spanstore.QueryParams, error) {
	values := r.URL.Query()
	service := strings.TrimSpace(values.Get("service"))
	if service == "" {
		return spanstore.QueryParams{}, fmt.Errorf("service is required")
	}
	start, err := parseInt64(values.Get("start"), "start")
	if err != nil {
		return spanstore.QueryParams{}, err
	}
	end, err := parseInt64(values.Get("end"), "end")
	if err != nil {
		return spanstore.QueryParams{}, err
	}
	limit := defaultLimit
	if rawLimit := strings.TrimSpace(values.Get("limit")); rawLimit != "" {
		parsed, err := parseInt(rawLimit, "limit")
		if err != nil {
			return spanstore.QueryParams{}, err
		}
		limit = parsed
	}
	filters, err := parseAttrFilters(values["attr"])
	if err != nil {
		return spanstore.QueryParams{}, err
	}
	return spanstore.QueryParams{
		Service:     service,
		Start:       start,
		End:         end,
		Limit:       limit,
		AttrFilters: filters,
	}, nil
}

func parseInt64(raw, field string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("%s is required", field)
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an int64", field)
	}
	return value, nil
}

func parseInt(raw, field string) (int, error) {
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an int", field)
	}
	if value <= 0 {
		return 0, fmt.Errorf("%s must be > 0", field)
	}
	return value, nil
}

func parseAttrFilters(rawFilters []string) ([]spanstore.AttrFilter, error) {
	filters := make([]spanstore.AttrFilter, 0, len(rawFilters))
	for _, raw := range rawFilters {
		parts := strings.SplitN(raw, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("attr must be key=value")
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" || value == "" {
			return nil, fmt.Errorf("attr must be key=value")
		}
		filters = append(filters, spanstore.AttrFilter{Key: key, Value: value})
	}
	return filters, nil
}
