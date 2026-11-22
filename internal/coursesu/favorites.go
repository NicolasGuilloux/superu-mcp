package coursesu

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/NicolasGuilloux/superu-mcp/internal/client"
	"github.com/NicolasGuilloux/superu-mcp/internal/model"
	"github.com/NicolasGuilloux/superu-mcp/internal/parse"
	"github.com/NicolasGuilloux/superu-mcp/internal/session"
)

const favBatchSize = 4

type favPageFetcher func(ctx context.Context, n int) ([]model.Product, error)

type FavoritesOptions struct {
	OnePage  bool
	MaxPages int
	Limit    int
}

func Favorites(ctx context.Context, opts FavoritesOptions) (model.FavoritesResult, error) {
	cookies, err := session.RequireCookies()
	if err != nil {
		return model.FavoritesResult{}, err
	}
	fetch := func(ctx context.Context, n int) ([]model.Product, error) {
		url := fmt.Sprintf(
			"/mon-compte/mes-listes?srule=shoplist-default&isPref=true&page=%d", n)
		resp, err := client.Get(ctx, url, client.BaseURL+"/", cookies)
		if err != nil {
			return nil, err
		}
		if err := checkOK(resp, fmt.Sprintf("favorites page %d", n)); err != nil {
			return nil, err
		}
		return parse.Products(resp.Body)
	}
	res, err := collectFavorites(ctx, fetch, opts)
	if err != nil {
		return model.FavoritesResult{}, err
	}
	session.Persist(cookies, nil)
	return res, nil
}

func collectFavorites(ctx context.Context, fetch favPageFetcher, opts FavoritesOptions) (model.FavoritesResult, error) {
	maxPages := opts.MaxPages
	if maxPages <= 0 {
		maxPages = 50
	}
	all := make([]model.Product, 0)
	seen := map[string]bool{}
	pagesFetched := 0
	nextPage := 1

	for pagesFetched < maxPages {
		batch := favBatchSize
		if opts.OnePage {
			batch = 1
		}
		if rem := maxPages - pagesFetched; batch > rem {
			batch = rem
		}

		results := make([][]model.Product, batch)
		g, gctx := errgroup.WithContext(ctx)
		for i := 0; i < batch; i++ {
			i, page := i, nextPage+i
			g.Go(func() error {
				p, err := fetch(gctx, page)
				if err != nil {
					return err
				}
				results[i] = p
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return model.FavoritesResult{}, err
		}

		stop := false
		for i := 0; i < batch; i++ {
			newOnPage := false
			for _, p := range results[i] {
				if seen[p.ID] {
					continue
				}
				seen[p.ID] = true
				all = append(all, p)
				newOnPage = true
			}
			pagesFetched++
			if !newOnPage {
				stop = true
				break
			}
		}
		nextPage += batch
		if stop || opts.OnePage {
			break
		}
	}

	if opts.Limit > 0 && opts.Limit < len(all) {
		all = all[:opts.Limit]
	}
	return model.FavoritesResult{
		Count:        len(all),
		PagesFetched: pagesFetched,
		Products:     all,
	}, nil
}
