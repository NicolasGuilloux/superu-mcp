package coursesu

import (
	"context"
	"net/url"
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
	products, err := parse.AdobeProducts(resp.Body)
	if err != nil {
		return nil, err
	}
	for i := range products {
		enrichFromPage(ctx, &products[i], cookies)
	}
	return products, nil
}

// enrichFromPage adds what only the product page shows: unit price,
// Nutri-Score, ingredients, nutrition values... The page address has a slug,
// so it is found through a search for the EAN. Failures leave p as it is.
func enrichFromPage(ctx context.Context, p *model.ProductDetail, cookies map[string]string) {
	query := p.ID
	if p.EAN != nil && *p.EAN != "" {
		query = *p.EAN
	}
	resp, err := client.Get(ctx, "/recherche?q="+url.QueryEscape(query), client.BaseURL+"/", cookies)
	if err != nil || resp.StatusCode != 200 {
		return
	}
	link := parse.ProductURL(resp.Body, p.ID)
	if link == "" {
		return
	}
	page, err := client.Get(ctx, link, client.BaseURL+"/", cookies)
	if err != nil || page.StatusCode != 200 {
		return
	}
	p.URL = link
	if strings.HasPrefix(link, "/") {
		p.URL = client.BaseURL + link
	}
	p.UnitPrice, p.NutriScore, p.Sections = parse.ProductPage(page.Body)
}
