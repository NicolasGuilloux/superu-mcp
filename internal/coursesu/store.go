package coursesu

import (
	"context"
	"html"
	"net/url"
	"regexp"
	"strings"

	"github.com/NicolasGuilloux/superu-mcp/internal/client"
	"github.com/NicolasGuilloux/superu-mcp/internal/session"
)

// Store is one store from the store locator.
type Store struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Address  string `json:"address"`
	Distance string `json:"distance_km,omitempty"`
	Drive    string `json:"drive,omitempty"`
}

var (
	storeNameRe  = regexp.MustCompile(`(?s)data-store-name>(.*?)</h3>`)
	storeDistRe  = regexp.MustCompile(`(?s)store-distance[^>]*><span[^>]*>(.*?)</span>`)
	storeAddrRe  = regexp.MustCompile(`(?s)class="store-address[^"]*"[^>]*>(.*?)</div>`)
	storeDriveRe = regexp.MustCompile(`(?s)mode-RETRAIT.*?store-available-time-slot">(.*?)</p>`)
	tagRe        = regexp.MustCompile(`<[^>]*>`)
	spaceRe      = regexp.MustCompile(`\s+`)
)

func cleanText(s string) string {
	return strings.TrimSpace(spaceRe.ReplaceAllString(html.UnescapeString(tagRe.ReplaceAllString(s, " ")), " "))
}

func firstMatch(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return cleanText(m[1])
	}
	return ""
}

// parseStores reads the store locator results.
func parseStores(body string) []Store {
	stores := []Store{}
	// Split on each store item, so a regexp never spans two stores.
	parts := strings.Split(body, `class="store-container"`)
	idRe := regexp.MustCompile(`data-store-id="(\d+)"`)
	for _, p := range parts[1:] {
		m := idRe.FindStringSubmatch(p)
		if m == nil {
			continue
		}
		stores = append(stores, Store{
			ID:       m[1],
			Name:     firstMatch(storeNameRe, p),
			Address:  firstMatch(storeAddrRe, p),
			Distance: firstMatch(storeDistRe, p),
			Drive:    firstMatch(storeDriveRe, p),
		})
	}
	return stores
}

// FindStores lists the drive stores near a postcode and town, nearest first.
func FindStores(ctx context.Context, zip, city string) ([]Store, error) {
	cookies, err := session.RequireCookies()
	if err != nil {
		return nil, err
	}
	q := url.Values{"zip_code": {zip}, "city": {city}, "delivery_mode": {"drive"},
		"trustCoordinates": {"false"}}
	resp, err := client.GetAjax(ctx, "/StoreLocator-GeolocateByCPandCity?"+q.Encode(),
		client.BaseURL+"/drive/home", cookies)
	if err != nil {
		return nil, err
	}
	if err := checkOK(resp, "store search"); err != nil {
		return nil, err
	}
	session.Persist(cookies, resp.SetCookies)
	return parseStores(resp.Body), nil
}

// SetStore makes storeID the store the cart and searches use, the way the
// "change store" button of coursesu.com does.
func SetStore(ctx context.Context, storeID string) (map[string]any, error) {
	cookies, err := session.RequireCookies()
	if err != nil {
		return nil, err
	}
	csrf, err := primeCSRF(ctx, cookies)
	if err != nil {
		return nil, err
	}
	q := url.Values{"store_id": {storeID}, "type_delivery": {"RETRAIT"}, "csrf_token": {csrf}}
	resp, err := client.GetAjax(ctx, "/StoreLocator-SetStore?"+q.Encode(),
		client.BaseURL+"/drive/home", cookies)
	if err != nil {
		return nil, err
	}
	session.Persist(cookies, resp.SetCookies)
	if resp.StatusCode != 200 || strings.Contains(resp.Body, `"error":true`) {
		return nil, newErr("coursesu.com did not accept the store",
			map[string]any{"status_code": resp.StatusCode, "body_preview": preview(resp.Body, 300)})
	}
	return map[string]any{"status": "ok", "store_id": storeID,
		"store_cookie": cookies["storeId"]}, nil
}
