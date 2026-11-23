package coursesu

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/NicolasGuilloux/superu-mcp/internal/client"
	"github.com/NicolasGuilloux/superu-mcp/internal/model"
	"github.com/NicolasGuilloux/superu-mcp/internal/parse"
	"github.com/NicolasGuilloux/superu-mcp/internal/session"
)

// Cart-AddProduct is sent with update=true, so qty is the line's absolute
// quantity and qty 0 removes the line. CartRemove and CartSet delegate here.
//
// coursesu.com forgets the chosen store when the session is renewed, and
// Cart-AddProduct then answers with an empty body. When a store is saved,
// CartAdd chooses it again and retries once.
func CartAdd(ctx context.Context, productID string, qty int) (model.CartMutationResult, error) {
	res, err := cartAdd(ctx, productID, qty)
	var de *DomainError
	if err == nil || !errors.As(err, &de) || !de.emptyCart {
		return res, err
	}
	cookies, cerr := session.RequireCookies()
	if cerr != nil || cookies["storeId"] == "" {
		return res, err
	}
	if _, serr := SetStore(ctx, cookies["storeId"]); serr != nil {
		return res, err
	}
	return cartAdd(ctx, productID, qty)
}

func cartAdd(ctx context.Context, productID string, qty int) (model.CartMutationResult, error) {
	cookies, err := session.RequireCookies()
	if err != nil {
		return model.CartMutationResult{}, err
	}
	csrf, err := primeCSRF(ctx, cookies)
	if err != nil {
		return model.CartMutationResult{}, err
	}

	qtyStr := strconv.Itoa(qty)
	data := map[string]string{
		"pid": productID, "Quantity": qtyStr, "update": "true",
		"modifiableOrderID": "", "fullview": "false", "location": "undefined",
		"quantity": qtyStr, "requestedQuantity": "",
		"isLuckyProduct": "false", "pageId": "", "pageType": "",
		"operationId": "", "sponsoringType": "", "sponsoringCampaignId": "",
		"csrf_token": csrf,
	}
	resp, err := client.PostForm(ctx, "/Cart-AddProduct?format=ajax", data,
		client.BaseURL+"/recherche?q=a", cookies)
	if err != nil {
		return model.CartMutationResult{}, err
	}
	session.Persist(cookies, resp.SetCookies)

	if len(resp.Body) == 0 {
		return model.CartMutationResult{}, emptyCartErr(
			fmt.Sprintf("Cart-AddProduct returned an empty response (HTTP %d) and "+
				"the cart was unchanged: no store is chosen. Choose one with "+
				"`coursesu stores` and `coursesu store-set` (MCP: stores_find, store_set)",
				resp.StatusCode),
			map[string]any{"status_code": resp.StatusCode})
	}

	dl, err := parse.DataLayer(resp.Body)
	if err != nil {
		return model.CartMutationResult{}, err
	}
	if dl == nil {
		return model.CartMutationResult{}, newErr(
			"could not parse dataLayer from response", map[string]any{
				"status_code":  resp.StatusCode,
				"ok":           resp.StatusCode == 200,
				"body_preview": preview(resp.Body, 500),
			})
	}
	summary := parse.CartSummary(dl)
	return model.CartMutationResult{
		StatusCode: resp.StatusCode,
		OK:         resp.StatusCode == 200,
		Cart:       &summary,
	}, nil
}

func CartRemove(ctx context.Context, productID string) (model.CartMutationResult, error) {
	return CartAdd(ctx, productID, 0)
}

func CartSet(ctx context.Context, productID string, qty int) (model.CartMutationResult, error) {
	return CartAdd(ctx, productID, qty)
}

func CartList(ctx context.Context) (model.CartSummary, error) {
	cookies, err := session.RequireCookies()
	if err != nil {
		return model.CartSummary{}, err
	}
	resp, err := client.GetAjax(ctx, "/Cart-ShowMinicartProducts?start=0&format=ajax",
		client.BaseURL+"/", cookies)
	if err != nil {
		return model.CartSummary{}, err
	}
	if err := checkOK(resp, "cart-list"); err != nil {
		return model.CartSummary{}, err
	}
	session.Persist(cookies, resp.SetCookies)

	dl := parse.MinicartDataLayer(resp.Body)
	if dl == nil {
		if parse.MinicartIsEmpty(resp.Body) {
			return parse.CartSummary(map[string]any{}), nil
		}
		return model.CartSummary{}, newErr(
			"could not parse data-tc-minicart-products from minicart response",
			map[string]any{
				"status_code":  resp.StatusCode,
				"body_preview": preview(resp.Body, 500),
			})
	}
	return parse.CartSummary(dl), nil
}

func CartClear(ctx context.Context) (model.CartMutationResult, error) {
	cookies, err := session.RequireCookies()
	if err != nil {
		return model.CartMutationResult{}, err
	}
	csrf, err := primeCSRF(ctx, cookies)
	if err != nil {
		return model.CartMutationResult{}, err
	}
	resp, err := client.PostForm(ctx, "/Cart-Empty?format=ajax",
		map[string]string{"csrf_token": csrf, "pageId": "", "pageType": ""},
		client.BaseURL+"/recherche?q=a", cookies)
	if err != nil {
		return model.CartMutationResult{}, err
	}
	if err := checkOK(resp, "cart-empty"); err != nil {
		return model.CartMutationResult{}, err
	}
	session.Persist(cookies, resp.SetCookies)

	summary, err := CartList(ctx)
	if err != nil {
		return model.CartMutationResult{}, err
	}
	return model.CartMutationResult{
		StatusCode: resp.StatusCode,
		OK:         resp.StatusCode == 200,
		Cart:       &summary,
	}, nil
}

func primeCSRF(ctx context.Context, cookies map[string]string) (string, error) {
	primer, err := client.Get(ctx, "/recherche?q=a", client.BaseURL+"/", cookies)
	if err != nil {
		return "", err
	}
	if err := checkOK(primer, "csrf primer fetch"); err != nil {
		return "", err
	}
	csrf := parse.CSRF(primer.Body)
	if csrf == "" {
		return "", newErr(
			"could not extract csrf_token from page "+
				"(session probably expired — try: coursesu login)", nil)
	}
	client.MergeCookies(cookies, primer.SetCookies)
	return csrf, nil
}
