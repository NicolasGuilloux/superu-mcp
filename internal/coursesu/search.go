package coursesu

import (
	"context"
	"fmt"
	"net/url"

	"github.com/NicolasGuilloux/superu-mcp/internal/client"
	"github.com/NicolasGuilloux/superu-mcp/internal/model"
	"github.com/NicolasGuilloux/superu-mcp/internal/parse"
	"github.com/NicolasGuilloux/superu-mcp/internal/session"
)

func Search(ctx context.Context, query string, limit int) ([]model.Product, error) {
	return SearchPage(ctx, query, 1, 0, limit)
}

// SearchPage fetches one page of results. page starts at 1; size 0 keeps the
// site's default page size.
func SearchPage(ctx context.Context, query string, page, size, limit int) ([]model.Product, error) {
	cookies := session.LoadCookies()
	path := "/recherche?q=" + url.QueryEscape(query)
	if size > 0 {
		if page < 1 {
			page = 1
		}
		path += fmt.Sprintf("&sz=%d&start=%d", size, (page-1)*size)
	}
	resp, err := client.Get(ctx, path, client.BaseURL+"/", cookies)
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
