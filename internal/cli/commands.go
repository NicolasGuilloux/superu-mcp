package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"

	"golang.org/x/term"

	"github.com/NicolasGuilloux/superu-mcp/internal/coursesu"
	"github.com/NicolasGuilloux/superu-mcp/internal/flaresolverr"
	"github.com/NicolasGuilloux/superu-mcp/internal/session"
	"github.com/spf13/cobra"
)

func run(fn func(ctx context.Context, args []string) (any, error)) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		res, err := coursesu.WithClearance(cmd.Context(), func(ctx context.Context) (any, error) {
			return fn(ctx, args)
		})
		if err != nil {
			return err
		}
		return emit(res)
	}
}

func simpleCmd(use, short string, args cobra.PositionalArgs,
	fn func(ctx context.Context, args []string) (any, error)) *cobra.Command {
	return &cobra.Command{Use: use, Short: short, Args: args, RunE: run(fn)}
}

func arg0[T any](fn func(context.Context) (T, error)) func(context.Context, []string) (any, error) {
	return func(ctx context.Context, _ []string) (any, error) { return fn(ctx) }
}

func arg1[T any](fn func(context.Context, string) (T, error)) func(context.Context, []string) (any, error) {
	return func(ctx context.Context, a []string) (any, error) { return fn(ctx, a[0]) }
}

func newLoginCmd() *cobra.Command {
	var fromFile, email, callback, ssoCookie string
	var link, renew bool
	var firefox bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "log in through a browser window and save the session",
		Long: "Opens a Chromium-family browser (Chromium, Chrome, Brave, Edge) on the\n" +
			"login page with a dedicated profile. With --email, the form is filled in for\n" +
			"you; the password comes from COURSESU_PASSWORD, or is asked for. You tick the\n" +
			"human check and submit; the session is saved once you are logged in.\n" +
			"--firefox keeps the original Firefox flow.\n\n" +
			"Without a local browser: `login --link` prints a link to open anywhere, then\n" +
			"`login --callback '<url>'` finishes with the address you land on. --link\n" +
			"needs FlareSolverr (COURSESU_FLARESOLVERR_URL, default " + flaresolverr.DefaultURL + ").",
		Args: cobra.NoArgs,
		RunE: run(func(ctx context.Context, _ []string) (any, error) {
			if fromFile != "" {
				return coursesu.LoginFromCurl(ctx, fromFile)
			}
			if ssoCookie != "" {
				if err := session.SetSSOCookie(ssoCookie); err != nil {
					return nil, err
				}
				if callback == "" && !renew {
					return map[string]any{"status": "ok", "sso_cookie": "saved"}, nil
				}
			}
			if link {
				return coursesu.LoginLink(ctx)
			}
			if callback != "" {
				return coursesu.LoginCallback(ctx, callback)
			}
			if renew {
				return coursesu.RenewSession(ctx)
			}
			if firefox {
				return coursesu.LoginFirefox(ctx)
			}
			password := os.Getenv("COURSESU_PASSWORD")
			if email != "" && password == "" {
				p, err := readPassword()
				if err != nil {
					return nil, err
				}
				password = p
			}
			return coursesu.LoginChromium(ctx, email, password)
		}),
	}
	cmd.Flags().StringVar(&fromFile, "from-file", "",
		"fallback: parse cookies and User-Agent from a copied cURL file")
	cmd.Flags().StringVar(&email, "email", "", "fill in the login form with this email")
	cmd.Flags().BoolVar(&link, "link", false,
		"print a login link to open in any browser (needs FlareSolverr)")
	cmd.Flags().StringVar(&callback, "callback", "",
		"finish a --link login with the coursesu.com URL containing code= and state=")
	cmd.Flags().StringVar(&ssoCookie, "sso-cookie", "",
		"save the Magasins U SSO cookie (b870a934f1cc0fa on moncompte.magasins-u.com) so the session renews itself")
	cmd.Flags().BoolVar(&renew, "renew", false, "renew the session now with the saved SSO cookie")
	cmd.Flags().BoolVar(&firefox, "firefox", false, "use the Firefox sessionstore flow instead")
	return cmd
}

func newSessionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "session",
		Short: "show whether the saved session is still logged in",
		Args:  cobra.NoArgs,
		RunE: run(func(ctx context.Context, _ []string) (any, error) {
			st, err := coursesu.WithClearance(ctx, coursesu.Status)
			if err != nil {
				st.ExpiredMessage = err.Error()
			}
			return st, nil
		}),
	}
}

func newSearchCmd() *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "search products",
		Args:  cobra.ExactArgs(1),
		RunE: run(func(ctx context.Context, a []string) (any, error) {
			return coursesu.Search(ctx, a[0], limit)
		}),
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "cap the number of results")
	return cmd
}

func newCartAddCmd() *cobra.Command {
	var qty int
	cmd := &cobra.Command{
		Use:   "cart-add <pid>",
		Short: "add a product to the cart",
		Args:  cobra.ExactArgs(1),
		RunE: run(func(ctx context.Context, a []string) (any, error) {
			return coursesu.CartAdd(ctx, a[0], qty)
		}),
	}
	cmd.Flags().IntVar(&qty, "qty", 1, "quantity to add")
	return cmd
}

func newFavoritesCmd() *cobra.Command {
	var opts coursesu.FavoritesOptions
	cmd := &cobra.Command{
		Use:   "favorites",
		Short: "list your frequently bought products",
		Args:  cobra.NoArgs,
		RunE: run(func(ctx context.Context, _ []string) (any, error) {
			return coursesu.Favorites(ctx, opts)
		}),
	}
	f := cmd.Flags()
	f.BoolVar(&opts.OnePage, "one-page", false, "only fetch page 1")
	f.IntVar(&opts.MaxPages, "max-pages", 50, "safety cap on pages")
	f.IntVar(&opts.Limit, "limit", 0, "cap the number of products")
	return cmd
}

func newHistoryCmd() *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "history",
		Short: "list your completed past orders with line items",
		Args:  cobra.NoArgs,
		RunE: run(func(ctx context.Context, _ []string) (any, error) {
			return coursesu.History(ctx, limit)
		}),
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "cap the number of orders")
	return cmd
}

func subcommands() []*cobra.Command {
	return []*cobra.Command{
		newLoginCmd(),
		newSessionCmd(),
		newSearchCmd(),
		newCartAddCmd(),
		newFavoritesCmd(),
		newHistoryCmd(),
		simpleCmd("cart-list", "list current cart contents",
			cobra.NoArgs, arg0(coursesu.CartList)),
		simpleCmd("cart-remove <pid>",
			"remove a product from the cart",
			cobra.ExactArgs(1), arg1(coursesu.CartRemove)),
		simpleCmd("cart-set <pid> <qty>",
			"set a product's quantity",
			cobra.ExactArgs(2), func(ctx context.Context, a []string) (any, error) {
				qty, err := strconv.Atoi(a[1])
				if err != nil {
					return nil, err
				}
				return coursesu.CartSet(ctx, a[0], qty)
			}),
		simpleCmd("cart-clear", "empty the cart",
			cobra.NoArgs, arg0(coursesu.CartClear)),
		simpleCmd("product <pid>...",
			"fetch the raw Adobe-products payload for one or more pids",
			cobra.MinimumNArgs(1), func(ctx context.Context, a []string) (any, error) {
				return coursesu.Product(ctx, a)
			}),
	}
}

// readPassword asks for the password on the terminal without echoing it.
func readPassword() (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", errors.New("no password: set COURSESU_PASSWORD or run in a terminal")
	}
	fmt.Fprint(os.Stderr, "Password: ")
	b, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
