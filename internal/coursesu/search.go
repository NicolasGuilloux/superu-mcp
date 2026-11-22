package coursesu

import (
	"context"
	"net/url"

	"github.com/NicolasGuilloux/superu-mcp/internal/client"
	"github.com/NicolasGuilloux/superu-mcp/internal/model"
	"github.com/NicolasGuilloux/superu-mcp/internal/parse"
	"github.com/NicolasGuilloux/superu-mcp/internal/session"
)

func Search(ctx context.Context, query string, limit int) ([]model.Product, error) {
	cookies := session.LoadCookies()
	resp, err := client.Get(ctx, "/recherche?q="+url.QueryEscape(query), client.BaseURL+"/", cookies)
	if err != nil {
		return nil, err
	}
	if err := checkOK(resp, "search"); err != nil {
		return nil, err
	}
	products, err := parse.Products(resp.Body)
	if err != nil {
		return nil, err
	}
	if limit > 0 && limit < len(products) {
		products = products[:limit]
	}
	return products, nil
}
