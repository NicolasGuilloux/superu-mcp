package coursesu

import (
	"fmt"

	"github.com/NicolasGuilloux/superu-mcp/internal/client"
)

func checkOK(resp *client.Response, label string) error {
	if resp.StatusCode == 200 {
		return nil
	}
	return newErr(fmt.Sprintf("%s failed: HTTP %d", label, resp.StatusCode),
		map[string]any{"body_preview": preview(resp.Body, 200)})
}

type DomainError struct {
	Msg   string
	Extra map[string]any
}

func (e *DomainError) Error() string { return e.Msg }

func newErr(msg string, extra map[string]any) *DomainError {
	return &DomainError{Msg: msg, Extra: extra}
}

func preview(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
