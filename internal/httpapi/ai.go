package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"timepilot/internal/planner"
)

type planInput struct {
	Goal             string `json:"goal"`
	AvailableMinutes int    `json:"available_minutes"`
}

func (a *API) aiStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.planner.Info())
}

func (a *API) createPlan(w http.ResponseWriter, r *http.Request) {
	if !a.planner.Info().Enabled {
		writeError(w, http.StatusServiceUnavailable, "AI planner is not configured")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input planInput
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "body must contain one JSON object")
		return
	}
	input.Goal = strings.TrimSpace(input.Goal)
	if utf8.RuneCountInString(input.Goal) < 1 || utf8.RuneCountInString(input.Goal) > 500 {
		writeError(w, http.StatusBadRequest, "goal must be 1-500 characters")
		return
	}
	if input.AvailableMinutes < 15 || input.AvailableMinutes > 1440 {
		writeError(w, http.StatusBadRequest, "available_minutes must be between 15 and 1440")
		return
	}

	tasks, err := a.store.List(r.Context())
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	plan, err := a.planner.Plan(r.Context(), planner.Request{
		Goal:             input.Goal,
		AvailableMinutes: input.AvailableMinutes,
		Tasks:            tasks,
	})
	if err != nil {
		a.logger.Error("AI planning failed", "provider", a.planner.Info().Provider, "error", err)
		writeError(w, http.StatusBadGateway, "AI provider request failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"plan":     plan,
		"provider": a.planner.Info().Provider,
		"model":    a.planner.Info().Model,
	})
}
