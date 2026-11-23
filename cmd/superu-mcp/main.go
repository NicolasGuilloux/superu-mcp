// Command superu-mcp is an MCP server (stdio) for coursesu.com, the
// Système U online supermarket (Super U, Hyper U, U Express).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/NicolasGuilloux/superu-mcp/internal/client"
	"github.com/NicolasGuilloux/superu-mcp/internal/coursesu"
	"github.com/NicolasGuilloux/superu-mcp/internal/session"
)

var version = "dev"

const instructions = `Tools for coursesu.com, the Système U online supermarket (Super U, Hyper U, U Express drive and delivery).

Log in once with login_start then login_finish; the session is saved and reused, and renews itself when login_finish was given the SSO cookie. session_status tells whether it is still logged in. Cart changes only work once a store and a delivery or pickup slot are chosen on coursesu.com. Products are identified by their EAN barcode, as returned by search_products, favorites_list and basket_get. The tools never place or pay for an order: the person checks out on coursesu.com.`

type searchIn struct {
	Query string `json:"query" jsonschema:"product search query, in French, e.g. lait demi-écrémé"`
	Page  int    `json:"page,omitempty" jsonschema:"page number (default: 1)"`
	Size  int    `json:"size,omitempty" jsonschema:"results per page (default: 20)"`
}

type storesIn struct {
	Postcode string `json:"postcode" jsonschema:"French postcode, e.g. 35136"`
	Town     string `json:"town" jsonschema:"town name, e.g. Saint-Jacques-de-la-Lande"`
}

type storeSetIn struct {
	StoreID string `json:"store_id" jsonschema:"store id from stores_find"`
}

type eanIn struct {
	EAN string `json:"ean" jsonschema:"product EAN-13 barcode (a coursesu product id is accepted too)"`
}

type basketAddIn struct {
	EAN      string `json:"ean" jsonschema:"product EAN-13 barcode (a coursesu product id is accepted too)"`
	Quantity int    `json:"quantity,omitempty" jsonschema:"quantity to add (default: 1)"`
}

type basketRemoveIn struct {
	EAN      string `json:"ean" jsonschema:"product EAN-13 barcode (a coursesu product id is accepted too)"`
	Quantity int    `json:"quantity,omitempty" jsonschema:"quantity to remove (0 = remove all, default: 0)"`
}

type orderIn struct {
	OrderNumber string `json:"order_number" jsonschema:"order number, from orders_list"`
}

type favoritesIn struct {
	Limit int `json:"limit,omitempty" jsonschema:"maximum number of products to return"`
}

type loginFinishIn struct {
	CallbackURL string `json:"callback_url" jsonschema:"the www.coursesu.com address containing code= and state=, copied after logging in with the login_start link"`
	SSOCookie   string `json:"sso_cookie,omitempty" jsonschema:"optional: value of the b870a934f1cc0fa cookie of moncompte.magasins-u.com, so the session renews itself when it expires"`
}

type empty struct{}

// callMu serializes tool calls: they share one HTTP client and one
// session file, and the HTTP transports can serve several clients at once.
var callMu sync.Mutex

// call runs fn with the saved session, refreshing the Cloudflare clearance
// when needed, and returns its result as JSON text.
func call[T any](ctx context.Context, fn func(context.Context) (T, error)) (*mcp.CallToolResult, any, error) {
	callMu.Lock()
	defer callMu.Unlock()
	out, err := coursesu.WithClearance(ctx, fn)
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

func newServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "superu-mcp", Version: version},
		&mcp.ServerOptions{Instructions: instructions})

	mcp.AddTool(server, &mcp.Tool{Name: "search_products",
		Description: "Search the coursesu.com product catalog. Returns product ids, names, brands, EANs and categories."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in searchIn) (*mcp.CallToolResult, any, error) {
			size := in.Size
			if size <= 0 {
				size = 20
			}
			return call(ctx, func(ctx context.Context) (any, error) {
				return coursesu.SearchPage(ctx, in.Query, in.Page, size, size)
			})
		})

	mcp.AddTool(server, &mcp.Tool{Name: "product_detail",
		Description: "Get the sheet of a product by EAN barcode: name, brand, category, image and customer rating."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in eanIn) (*mcp.CallToolResult, any, error) {
			return call(ctx, func(ctx context.Context) (any, error) {
				pid, err := coursesu.ResolveProductID(ctx, in.EAN)
				if err != nil {
					return nil, err
				}
				return coursesu.Product(ctx, []string{pid})
			})
		})

	mcp.AddTool(server, &mcp.Tool{Name: "favorites_list",
		Description: "List the products the person buys often (their \"produits préférés\"). Requires login."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in favoritesIn) (*mcp.CallToolResult, any, error) {
			return call(ctx, func(ctx context.Context) (any, error) {
				return coursesu.Favorites(ctx, coursesu.FavoritesOptions{MaxPages: 50, Limit: in.Limit})
			})
		})

	mcp.AddTool(server, &mcp.Tool{Name: "stores_find",
		Description: "List the drive stores near a postcode and town, nearest first, with their ids. Requires login."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in storesIn) (*mcp.CallToolResult, any, error) {
			return call(ctx, func(ctx context.Context) (any, error) {
				return coursesu.FindStores(ctx, in.Postcode, in.Town)
			})
		})

	mcp.AddTool(server, &mcp.Tool{Name: "store_set",
		Description: "Choose the store (drive pickup) that searches and the cart use. Do it before adding to the cart. Requires login."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in storeSetIn) (*mcp.CallToolResult, any, error) {
			return call(ctx, func(ctx context.Context) (any, error) { return coursesu.SetStore(ctx, in.StoreID) })
		})

	mcp.AddTool(server, &mcp.Tool{Name: "basket_get",
		Description: "Get current shopping cart contents. Requires login."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, any, error) {
			return call(ctx, func(ctx context.Context) (any, error) { return coursesu.CartList(ctx) })
		})

	mcp.AddTool(server, &mcp.Tool{Name: "basket_add",
		Description: "Add a product to the shopping cart by EAN barcode. Requires login and a store and slot chosen on coursesu.com."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in basketAddIn) (*mcp.CallToolResult, any, error) {
			qty := in.Quantity
			if qty <= 0 {
				qty = 1
			}
			return call(ctx, func(ctx context.Context) (any, error) {
				pid, err := coursesu.ResolveProductID(ctx, in.EAN)
				if err != nil {
					return nil, err
				}
				return coursesu.CartAdd(ctx, pid, qty)
			})
		})

	mcp.AddTool(server, &mcp.Tool{Name: "basket_remove",
		Description: "Remove a product from the shopping cart by EAN barcode. Quantity 0 removes all. Requires login."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in basketRemoveIn) (*mcp.CallToolResult, any, error) {
			return call(ctx, func(ctx context.Context) (any, error) {
				pid, err := coursesu.ResolveProductID(ctx, in.EAN)
				if err != nil {
					return nil, err
				}
				return coursesu.CartRemoveQuantity(ctx, pid, in.Quantity)
			})
		})

	mcp.AddTool(server, &mcp.Tool{Name: "basket_clear",
		Description: "Empty the whole shopping cart. Ask the person before using it. Requires login."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, any, error) {
			return call(ctx, func(ctx context.Context) (any, error) { return coursesu.CartClear(ctx) })
		})

	mcp.AddTool(server, &mcp.Tool{Name: "orders_list",
		Description: "List recent orders with their date, store, total and articles. Requires login."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, any, error) {
			return call(ctx, func(ctx context.Context) (any, error) { return coursesu.History(ctx, 0) })
		})

	mcp.AddTool(server, &mcp.Tool{Name: "order_detail",
		Description: "Get detailed information about a specific order including all articles. Requires login."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in orderIn) (*mcp.CallToolResult, any, error) {
			return call(ctx, func(ctx context.Context) (any, error) {
				return coursesu.FindOrder(ctx, in.OrderNumber)
			})
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
				if in.SSOCookie != "" {
					if err := session.SetSSOCookie(in.SSOCookie); err != nil {
						return nil, err
					}
				}
				return coursesu.LoginCallback(ctx, in.CallbackURL)
			})
		})

	mcp.AddTool(server, &mcp.Tool{Name: "session_status",
		Description: "Tell whether the saved coursesu.com session is still logged in, since when, and whether it renews itself."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, any, error) {
			return call(ctx, func(ctx context.Context) (any, error) { return coursesu.Status(ctx) })
		})

	return server
}

const usage = `superu-mcp: MCP server for coursesu.com

Usage:
  superu-mcp [mcp]           MCP server over stdio
  superu-mcp mcp http [addr] MCP server over HTTP (default :8080, or $MCP_ADDR):
                               Streamable HTTP on /mcp, SSE on /sse
`

func main() {
	log.SetOutput(os.Stderr)
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "mcp" {
		args = args[1:]
	}
	var err error
	switch {
	case len(args) == 0 || args[0] == "stdio":
		err = serveStdio()
	case args[0] == "http":
		addr := os.Getenv("MCP_ADDR")
		if len(args) > 1 {
			addr = args[1]
		}
		if addr == "" {
			addr = ":8080"
		}
		err = serveHTTP(addr)
	case args[0] == "-h" || args[0] == "--help" || args[0] == "help":
		fmt.Print(usage)
	case args[0] == "-v" || args[0] == "--version" || args[0] == "version":
		fmt.Println(version)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func serveStdio() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := newServer().Run(ctx, &mcp.StdioTransport{}); err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}

// serveHTTP serves the MCP server over Streamable HTTP on /mcp and over the
// older HTTP+SSE transport on /sse, until SIGINT or SIGTERM.
func serveHTTP(addr string) error {
	server := newServer()
	getServer := func(*http.Request) *mcp.Server { return server }
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcp.NewStreamableHTTPHandler(getServer, nil))
	mux.Handle("/sse", mcp.NewSSEHandler(getServer, nil))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok\n"))
	})
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go keepAlive(ctx)
	go renewLoop(ctx)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	log.Printf("MCP server listening on %s (Streamable HTTP: /mcp, SSE: /sse)", addr)
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}
	log.Print("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// keepAlive opens an account page every COURSESU_KEEPALIVE minutes (default
// 10, 0 disables), so the coursesu session does not time out while idle,
// and renews it silently when it has expired and an SSO cookie is saved.
func keepAlive(ctx context.Context) {
	minutes := 10
	if v := os.Getenv("COURSESU_KEEPALIVE"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			log.Printf("keepalive: bad COURSESU_KEEPALIVE %q, using 10", v)
		} else {
			minutes = n
		}
	}
	if minutes <= 0 {
		return
	}
	t := time.NewTicker(time.Duration(minutes) * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if _, err := session.RequireCookies(); err != nil {
			continue
		}
		callMu.Lock()
		st, err := coursesu.WithClearance(ctx, coursesu.Status)
		callMu.Unlock()
		if err != nil || !st.LoggedIn {
			log.Printf("keepalive: session not logged in: %v", err)
		}
	}
}

// renewLoop renews the session with the SSO cookie once it is
// COURSESU_RENEW_HOURS old (default 23, 0 turns it off), so the session is
// replaced before it expires. It goes by the session's login time, which
// survives restarts, and does nothing without a saved SSO cookie.
func renewLoop(ctx context.Context) {
	hours := 23
	if v := os.Getenv("COURSESU_RENEW_HOURS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			log.Printf("renew: bad COURSESU_RENEW_HOURS %q, using 23", v)
		} else {
			hours = n
		}
	}
	if hours <= 0 {
		return
	}
	every := time.Duration(hours) * time.Hour
	check := time.NewTicker(15 * time.Minute)
	defer check.Stop()
	var lastFail time.Time
	for {
		if at := session.AcquiredAt(); at > 0 && session.SSOCookie() != "" &&
			time.Since(time.Unix(at, 0)) >= every && time.Since(lastFail) >= time.Hour {
			callMu.Lock()
			res, err := coursesu.WithClearance(ctx, coursesu.RenewSession)
			callMu.Unlock()
			if err != nil {
				lastFail = time.Now()
				log.Printf("renew: silent renewal failed, next try in an hour: %v", err)
			} else {
				log.Printf("renew: session renewed (%s)", res.Source)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-check.C:
		}
	}
}
