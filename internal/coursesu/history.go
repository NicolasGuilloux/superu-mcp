package coursesu

import (
	"context"

	"github.com/NicolasGuilloux/superu-mcp/internal/client"
	"github.com/NicolasGuilloux/superu-mcp/internal/model"
	"github.com/NicolasGuilloux/superu-mcp/internal/parse"
	"github.com/NicolasGuilloux/superu-mcp/internal/session"
)

func History(ctx context.Context, limit int) (model.HistoryResult, error) {
	cookies, err := session.RequireCookies()
	if err != nil {
		return model.HistoryResult{}, err
	}
	resp, err := client.Get(ctx, "/mon-compte/mes-commandes", client.BaseURL+"/", cookies)
	if err != nil {
		return model.HistoryResult{}, err
	}
	if err := checkOK(resp, "history"); err != nil {
		return model.HistoryResult{}, err
	}
	session.Persist(cookies, resp.SetCookies)

	orders, err := parse.Orders(resp.Body)
	if err != nil {
		return model.HistoryResult{}, err
	}
	if limit > 0 && limit < len(orders) {
		orders = orders[:limit]
	}
	return model.HistoryResult{Count: len(orders), Orders: orders}, nil
}
