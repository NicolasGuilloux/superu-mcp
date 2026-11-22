package cli

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/NicolasGuilloux/superu-mcp/internal/client"
	"github.com/NicolasGuilloux/superu-mcp/internal/coursesu"
	"github.com/NicolasGuilloux/superu-mcp/internal/session"
)

func emit(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func report(err error) int {
	payload := map[string]any{}

	var sessExpired *client.SessionExpiredError
	var domain *coursesu.DomainError

	switch {
	case errors.As(err, &sessExpired):
		payload["error"] = sessExpired.Error()
		if sessExpired.Location != "" {
			payload["location"] = sessExpired.Location
		}
	case errors.Is(err, session.ErrNotLoggedIn):
		payload["error"] = session.ErrNotLoggedIn.Error()
	case errors.As(err, &domain):
		payload["error"] = domain.Msg
		for k, v := range domain.Extra {
			payload[k] = v
		}
	default:
		payload["error"] = err.Error()
	}

	enc := json.NewEncoder(os.Stderr)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(payload)
	return 1
}
