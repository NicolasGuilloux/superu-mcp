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

Other ways to log in with the CLI: `coursesu login` drives a local Chromium
window, `coursesu login --from-file curl.txt` reads a request copied as cURL
from your browser's DevTools.

Cart changes need a store and a delivery or pickup slot chosen on
coursesu.com first.

## MCP server

`superu-mcp` speaks MCP over stdio. Example for Claude Code:

```sh
claude mcp add superu -- ~/.local/bin/superu-mcp
```

Or in a client's JSON configuration:

```json
{
  "mcpServers": {
    "superu": { "command": "/home/you/.local/bin/superu-mcp" }
  }
}
```

| Tool | What it does |
|------|--------------|
| `search_products` | Search the catalog |
| `get_products` | Details and rating for product ids |
| `list_favorites` | Products you buy often |
| `list_orders` | Past orders with their items |
| `get_cart` | Current cart |
| `add_to_cart` | Add a product |
| `set_cart_quantity` | Change a product's quantity |
| `remove_from_cart` | Remove a product |
| `clear_cart` | Empty the cart |
| `login_start` | Get a login link |
| `login_finish` | Finish logging in with the copied address |

## Docker

Each push to `main` publishes `ghcr.io/nicolasguilloux/superu-mcp` (amd64 and
arm64; tags `latest`, `main`, `sha-…`, and `X.Y.Z` for `vX.Y.Z` git tags). The
image runs `superu-mcp` and also holds the `coursesu` CLI. The session is kept
in a volume, and FlareSolverr is reached at `http://flaresolverr:8191/v1` on a
shared network:

```sh
docker compose up -d          # starts FlareSolverr on the "superu" network

# log in once
docker run -it --rm --network superu -v superu-session:/home/nonroot/.config/coursesu \
  --entrypoint coursesu ghcr.io/nicolasguilloux/superu-mcp login --link
docker run -it --rm --network superu -v superu-session:/home/nonroot/.config/coursesu \
  --entrypoint coursesu ghcr.io/nicolasguilloux/superu-mcp login --callback '<url>'
```

MCP client configuration (stdio, so `-i` is required):

```json
{
  "mcpServers": {
    "superu": {
      "command": "docker",
      "args": ["run", "-i", "--rm", "--network", "superu",
               "-v", "superu-session:/home/nonroot/.config/coursesu",
               "ghcr.io/nicolasguilloux/superu-mcp"]
    }
  }
}
```

The container runs as uid 65532. With a bind mount instead of a named volume,
make the host directory writable by that uid.

## CLI

```
coursesu login [--link | --callback <url> | --from-file <file>]
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
