// Command superu-mcp is an MCP server (stdio) for coursesu.com, the
// Système U online supermarket (Super U, Hyper U, U Express).
package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/NicolasGuilloux/superu-mcp/internal/client"
	"github.com/NicolasGuilloux/superu-mcp/internal/coursesu"
	"github.com/NicolasGuilloux/superu-mcp/internal/session"
)

var version = "dev"

const instructions = `Tools for coursesu.com, the Système U online supermarket (Super U, Hyper U, U Express drive and delivery).

Log in once with login_start then login_finish; the session is saved and reused. Cart changes only work once a store and a delivery or pickup slot are chosen on coursesu.com. Product ids ("pid") come from search_products, list_favorites or get_cart. The tools never place or pay for an order: the person checks out on coursesu.com.`

type searchIn struct {
	Query string `json:"query" jsonschema:"what to search for, in French, e.g. lait demi-écrémé"`
	Limit int    `json:"limit,omitempty" jsonschema:"maximum number of products to return (default 20)"`
}

type productIn struct {
	ProductIDs []string `json:"product_ids" jsonschema:"product ids (pid) to describe"`
}

type favoritesIn struct {
	Limit int `json:"limit,omitempty" jsonschema:"maximum number of products to return"`
}

type historyIn struct {
	Limit int `json:"limit,omitempty" jsonschema:"maximum number of past orders to return"`
}

type cartAddIn struct {
	ProductID string `json:"product_id" jsonschema:"product id (pid) to add"`
	Quantity  int    `json:"quantity,omitempty" jsonschema:"how many to add (default 1)"`
}

type cartSetIn struct {
	ProductID string `json:"product_id" jsonschema:"product id (pid) already in the cart"`
	Quantity  int    `json:"quantity" jsonschema:"new quantity; 0 removes the product"`
}

type cartRemoveIn struct {
	ProductID string `json:"product_id" jsonschema:"product id (pid) to remove"`
}

type loginFinishIn struct {
	CallbackURL string `json:"callback_url" jsonschema:"the www.coursesu.com address containing code= and state=, copied after logging in with the login_start link"`
}

type empty struct{}

// call runs fn with the saved session. When Cloudflare turns the request
// away, it gets a fresh clearance through FlareSolverr and tries once more.
func call[T any](ctx context.Context, fn func(context.Context) (T, error)) (*mcp.CallToolResult, any, error) {
	client.SetUserAgent(session.UserAgent())
	out, err := fn(ctx)
	if coursesu.IsCloudflareBlock(err) {
		if rerr := coursesu.RefreshClearance(ctx); rerr == nil {
			out, err = fn(ctx)
		} else {
			log.Printf("clearance refresh failed: %v", rerr)
		}
	}
	if err != nil {
		return errorResult(err), nil, nil
	}
	b, merr := json.MarshalIndent(out, "", "  ")
	if merr != nil {
		return nil, nil, merr
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}, nil, nil
}

func errorResult(err error) *mcp.CallToolResult {
	payload := map[string]any{"error": err.Error()}
	var d *coursesu.DomainError
	if de, ok := err.(*coursesu.DomainError); ok {
		d = de
		for k, v := range d.Extra {
			payload[k] = v
		}
	}
	if _, ok := err.(*client.SessionExpiredError); ok || err == session.ErrNotLoggedIn {
		payload["hint"] = "log in again with login_start then login_finish"
	}
	b, _ := json.MarshalIndent(payload, "", "  ")
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}
}

func main() {
	log.SetOutput(os.Stderr)
	server := mcp.NewServer(&mcp.Implementation{Name: "superu-mcp", Version: version},
		&mcp.ServerOptions{Instructions: instructions})

	mcp.AddTool(server, &mcp.Tool{Name: "search_products",
		Description: "Search the coursesu.com catalog. Returns product ids, names, brands, EANs and categories."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in searchIn) (*mcp.CallToolResult, any, error) {
			limit := in.Limit
			if limit <= 0 {
				limit = 20
			}
			return call(ctx, func(ctx context.Context) (any, error) {
				return coursesu.Search(ctx, in.Query, limit)
			})
		})

	mcp.AddTool(server, &mcp.Tool{Name: "get_products",
		Description: "Get details and customer rating for one or more product ids."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in productIn) (*mcp.CallToolResult, any, error) {
			return call(ctx, func(ctx context.Context) (any, error) {
				return coursesu.Product(ctx, in.ProductIDs)
			})
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_favorites",
		Description: "List the products the person buys often (their \"produits préférés\")."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in favoritesIn) (*mcp.CallToolResult, any, error) {
			return call(ctx, func(ctx context.Context) (any, error) {
				return coursesu.Favorites(ctx, coursesu.FavoritesOptions{MaxPages: 50, Limit: in.Limit})
			})
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_orders",
		Description: "List past orders with their date, store, total and items."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in historyIn) (*mcp.CallToolResult, any, error) {
			return call(ctx, func(ctx context.Context) (any, error) {
				return coursesu.History(ctx, in.Limit)
			})
		})

	mcp.AddTool(server, &mcp.Tool{Name: "get_cart",
		Description: "Show the current cart: items, quantities and prices."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, any, error) {
			return call(ctx, func(ctx context.Context) (any, error) { return coursesu.CartList(ctx) })
		})

	mcp.AddTool(server, &mcp.Tool{Name: "add_to_cart",
		Description: "Add a product to the cart. Needs a store and a slot chosen on coursesu.com."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in cartAddIn) (*mcp.CallToolResult, any, error) {
			qty := in.Quantity
			if qty <= 0 {
				qty = 1
			}
			return call(ctx, func(ctx context.Context) (any, error) {
				return coursesu.CartAdd(ctx, in.ProductID, qty)
			})
		})

	mcp.AddTool(server, &mcp.Tool{Name: "set_cart_quantity",
		Description: "Set the quantity of a product already in the cart."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in cartSetIn) (*mcp.CallToolResult, any, error) {
			return call(ctx, func(ctx context.Context) (any, error) {
				return coursesu.CartSet(ctx, in.ProductID, in.Quantity)
			})
		})

	mcp.AddTool(server, &mcp.Tool{Name: "remove_from_cart",
		Description: "Remove a product from the cart."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in cartRemoveIn) (*mcp.CallToolResult, any, error) {
			return call(ctx, func(ctx context.Context) (any, error) {
				return coursesu.CartRemove(ctx, in.ProductID)
			})
		})

	mcp.AddTool(server, &mcp.Tool{Name: "clear_cart",
		Description: "Empty the whole cart. Ask the person before using it."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, any, error) {
			return call(ctx, func(ctx context.Context) (any, error) { return coursesu.CartClear(ctx) })
		})

	mcp.AddTool(server, &mcp.Tool{Name: "login_start",
		Description: "Start logging in to coursesu.com. Returns a link for the person to open in their own " +
			"browser, with instructions. Then call login_finish with the address they copy."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, any, error) {
			return call(ctx, func(ctx context.Context) (any, error) { return coursesu.LoginLink(ctx) })
		})

	mcp.AddTool(server, &mcp.Tool{Name: "login_finish",
		Description: "Finish logging in with the www.coursesu.com address (with code= and state=) that the " +
			"person copied after using the login_start link. Do it right away: the code expires within minutes."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in loginFinishIn) (*mcp.CallToolResult, any, error) {
			return call(ctx, func(ctx context.Context) (any, error) {
				return coursesu.LoginCallback(ctx, in.CallbackURL)
			})
		})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
