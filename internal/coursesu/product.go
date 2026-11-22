package coursesu

import (
	"context"
	"strings"

	"github.com/NicolasGuilloux/superu-mcp/internal/client"
	"github.com/NicolasGuilloux/superu-mcp/internal/model"
	"github.com/NicolasGuilloux/superu-mcp/internal/parse"
	"github.com/NicolasGuilloux/superu-mcp/internal/session"
)

func Product(ctx context.Context, productIDs []string) ([]model.ProductDetail, error) {
	cookies := session.LoadCookies()
	resp, err := client.PostForm(ctx, "/Product-AdobeProducts?format=ajax",
		map[string]string{"productslist": strings.Join(productIDs, ",")},
		client.BaseURL+"/", cookies)
	if err != nil {
		return nil, err
	}
	if err := checkOK(resp, "product"); err != nil {
		return nil, err
	}
	return parse.AdobeProducts(resp.Body)
}
