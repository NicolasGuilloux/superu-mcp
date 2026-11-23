# superu-mcp

MCP server and CLI for [coursesu.com](https://www.coursesu.com), the online
supermarket of the Système U stores (Super U, Hyper U, U Express): search the
catalog, manage your cart, and read your favorites and past orders from an AI
assistant or from the terminal.

It talks to coursesu.com's private endpoints, not an official API, so a change
on the site can break it. It never places or pays for an order: you check out
on coursesu.com yourself.

Based on [nmorvil/cli-drive-u](https://github.com/nmorvil/cli-drive-u) (MIT).

## Requirements

- Go 1.26 to build.
- [FlareSolverr](https://github.com/FlareSolverr/FlareSolverr), because
  coursesu.com is behind Cloudflare. It must run on the same machine or
  network as the server: Cloudflare ties its clearance to the IP address.

```sh
docker run -d --name flaresolverr -p 8191:8191 ghcr.io/flaresolverr/flaresolverr:latest
```

The tools look for it at `http://localhost:8191/v1`. Set
`COURSESU_FLARESOLVERR_URL` to use another address.

## Build

```sh
make install   # builds coursesu and superu-mcp into ~/.local/bin
```

## Log in

Login goes through the Magasins U account page, which asks for a "Vérifiez
que vous êtes humain" check, so you log in yourself in your own browser. It
works from a headless server:

1. Run `coursesu login --link` (or call the `login_start` tool). It prints a
   login link and keeps a PKCE secret on this machine.
2. In your browser, block the callback so the page does not use the code:
   DevTools, Network request blocking (Chrome: Cmd/Ctrl+Shift+P, "Show
   Network request blocking"), pattern `*Login-OAuthReentryForgerock?*`.
   With uBlock Origin: `||www.coursesu.com^*Login-OAuthReentryForgerock$document`.
3. Open the link in that tab and log in.
4. Copy the blocked address, which contains `code=` and `state=`, and right
   away run `coursesu login --callback '<url>'` (or call `login_finish`). The
   code expires within a few minutes.

The session is saved in `~/.config/coursesu/session.json` (mode 600). When
Cloudflare turns a request away, the MCP server gets a new clearance through
FlareSolverr and retries, keeping your account session. Log in again when the
account session expires.

### Keeping the session alive

The coursesu.com session times out (it was gone within 8 hours in testing).
Two things keep it going:

- In HTTP mode, the server opens an account page every 10 minutes so the
  session does not time out while idle. Set `COURSESU_KEEPALIVE` to another
  number of minutes, or `0` to turn it off.
- Silent renewal, the way coursesu.com renews its own session: an OpenID
  Connect request with `prompt=none` sent with the Magasins U SSO cookie. To
  set it up, after logging in, copy the value of the `b870a934f1cc0fa` cookie
  of `moncompte.magasins-u.com` (DevTools, Application, Cookies) and pass it
  with `coursesu login --sso-cookie '<value>'` (it can go along with
  `--callback`), or as `sso_cookie` to `login_finish`. When the coursesu
  session has expired, the CLI and the MCP server then log in again on their
  own, for as long as the SSO session lasts. `coursesu login --renew` does it
  by hand.

There is no refresh token: the SSO does not allow that grant for the
coursesu.com client. `coursesu session` (or the `session_status` tool) tells
whether the session is still logged in and since when.

Other ways to log in with the CLI: `coursesu login` drives a local Chromium
window, `coursesu login --from-file curl.txt` reads a request copied as cURL
from your browser's DevTools.

Cart changes need a store and a delivery or pickup slot chosen on
coursesu.com first.

## MCP server

The tools follow the same contract as
[intermarche-mcp](https://github.com/NicolasGuilloux/intermarche-mcp).

```
superu-mcp [mcp]             stdio
superu-mcp mcp http [addr]   HTTP on addr (default :8080, or $MCP_ADDR):
                               Streamable HTTP on /mcp
                               HTTP+SSE (older clients) on /sse
                               health check on /healthz
```

stdio, for example with Claude Code:

```sh
claude mcp add superu -- ~/.local/bin/superu-mcp
```

HTTP:

```sh
superu-mcp mcp http :8080
claude mcp add --transport http superu http://localhost:8080/mcp
claude mcp add --transport sse superu-sse http://localhost:8080/sse
```

The HTTP server has no authentication: anyone who reaches it can use your
coursesu.com account. Keep it on localhost or a private network, or put an
authenticating reverse proxy in front of it.

| Tool | Arguments | Description |
| --- | --- | --- |
| `search_products` | `query` (required), `page`, `size` | Search the catalog. |
| `product_detail` | `ean` (required) | Product sheet and customer rating. |
| `favorites_list` | `limit` | Products you buy often. Requires login. |
| `basket_get` | none | Current cart. Requires login. |
| `basket_add` | `ean` (required), `quantity` | Add a product by EAN-13 barcode. Requires login and a store and slot chosen on coursesu.com. |
| `basket_remove` | `ean` (required), `quantity` | Remove a product (0 = remove all). Requires login. |
| `basket_clear` | none | Empty the cart. Requires login. |
| `orders_list` | none | Recent orders with their articles. Requires login. |
| `order_detail` | `order_number` (required) | One order with all its articles. Requires login. |
| `login_start` | none | Get a login link. |
| `login_finish` | `callback_url` (required), `sso_cookie` | Finish logging in with the copied address; the SSO cookie enables silent renewal. |
| `session_status` | none | Whether the session is still logged in, since when, and whether it renews itself. |

Where a tool takes an `ean`, a coursesu product id is accepted too. Unlike
intermarche-mcp there are no `store_*` tools: the store and slot are chosen on
coursesu.com.

## Docker

Each push to `main` publishes `ghcr.io/nicolasguilloux/superu-mcp` (amd64 and
arm64; tags `latest`, `main`, `sha-…`, and `X.Y.Z` for `vX.Y.Z` git tags). The
image runs `superu-mcp mcp http` on port 8080 by default and also holds the
`coursesu` CLI. `compose.yaml` runs it next to FlareSolverr, with the session
in a volume:

```sh
docker compose up -d     # MCP on http://localhost:8080/mcp and /sse

# log in once
docker compose run --rm --entrypoint coursesu superu login --link
docker compose run --rm --entrypoint coursesu superu login --callback '<url>'
```

For stdio instead, pass `mcp` as the command:

```json
{
  "mcpServers": {
    "superu": {
      "command": "docker",
      "args": ["run", "-i", "--rm", "--network", "superu",
               "-v", "superu-session:/home/nonroot/.config/coursesu",
               "ghcr.io/nicolasguilloux/superu-mcp", "mcp"]
    }
  }
}
```

The container runs as uid 65532. With a bind mount instead of a named volume,
make the host directory writable by that uid.

## CLI

```
coursesu login [--link | --callback <url> | --sso-cookie <value> | --renew | --from-file <file>]
coursesu session
coursesu search <query> [--limit n]
coursesu product <pid>...
coursesu favorites
coursesu history
coursesu cart-list
coursesu cart-add <pid> [--qty n]
coursesu cart-set <pid> <qty>
coursesu cart-remove <pid>
coursesu cart-clear
```

Output is JSON. Run `coursesu <command> --help` for details.

## License

MIT, see [LICENSE](LICENSE).
