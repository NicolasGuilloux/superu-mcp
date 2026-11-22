package coursesu

import (
	"context"
	"testing"

	"github.com/NicolasGuilloux/superu-mcp/internal/model"
)

func fakePages(pages [][]string) favPageFetcher {
	return func(_ context.Context, n int) ([]model.Product, error) {
		if n < 1 || n > len(pages) {
			return []model.Product{}, nil
		}
		out := make([]model.Product, 0, len(pages[n-1]))
		for _, id := range pages[n-1] {
			out = append(out, model.Product{ID: id})
		}
		return out, nil
	}
}

func TestCollectFavoritesStopsAtEmptyPage(t *testing.T) {
	fetch := fakePages([][]string{{"a", "b"}, {"c"}, {}})
	res, err := collectFavorites(context.Background(), fetch, FavoritesOptions{})
	if err != nil {
		t.Fatalf("collectFavorites: %v", err)
	}
	if res.Count != 3 {
		t.Fatalf("want 3 products, got %d", res.Count)
	}
}

func TestCollectFavoritesOnePage(t *testing.T) {
	fetch := fakePages([][]string{{"a", "b"}, {"c"}})
	res, err := collectFavorites(context.Background(), fetch, FavoritesOptions{OnePage: true})
	if err != nil {
		t.Fatalf("collectFavorites: %v", err)
	}
	if res.PagesFetched != 1 || res.Count != 2 {
		t.Fatalf("one-page wrong: pages=%d count=%d", res.PagesFetched, res.Count)
	}
}

func TestCollectFavoritesDedup(t *testing.T) {
	fetch := fakePages([][]string{{"a", "b"}, {"b", "c"}, {}})
	res, err := collectFavorites(context.Background(), fetch, FavoritesOptions{})
	if err != nil {
		t.Fatalf("collectFavorites: %v", err)
	}
	if res.Count != 3 {
		t.Fatalf("want 3 deduped products, got %d", res.Count)
	}
}

func TestCollectFavoritesLimit(t *testing.T) {
	fetch := fakePages([][]string{{"a", "b", "c"}, {}})
	res, err := collectFavorites(context.Background(), fetch, FavoritesOptions{Limit: 2})
	if err != nil {
		t.Fatalf("collectFavorites: %v", err)
	}
	if res.Count != 2 {
		t.Fatalf("want 2 (limited), got %d", res.Count)
	}
}
