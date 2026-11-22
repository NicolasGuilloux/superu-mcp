package parse

import (
	"encoding/json"
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/NicolasGuilloux/superu-mcp/internal/model"
	"github.com/PuerkitoBio/goquery"
)

func doc(body string) (*goquery.Document, error) {
	return goquery.NewDocumentFromReader(strings.NewReader(body))
}

type productTile struct {
	ID      string  `json:"id"`
	Name    *string `json:"name"`
	EAN     *string `json:"EAN"`
	Brand   *string `json:"brand"`
	Cat1    *string `json:"product_cat1"`
	Cat2    *string `json:"product_cat2"`
	Cat3    *string `json:"product_cat3"`
	Picture *string `json:"product_url_picture"`
}

func Products(body string) ([]model.Product, error) {
	d, err := doc(body)
	if err != nil {
		return nil, err
	}
	out := make([]model.Product, 0)
	seen := map[string]bool{}
	d.Find("[data-tc-product-tile]").Each(func(_ int, s *goquery.Selection) {
		raw, ok := s.Attr("data-tc-product-tile")
		if !ok {
			return
		}
		var t productTile
		if json.Unmarshal([]byte(raw), &t) != nil {
			return
		}
		if t.ID == "" || seen[t.ID] {
			return
		}
		seen[t.ID] = true
		cats := make([]string, 0, 3)
		for _, c := range []*string{t.Cat1, t.Cat2, t.Cat3} {
			if c != nil && *c != "" {
				cats = append(cats, *c)
			}
		}
		out = append(out, model.Product{
			ID:       t.ID,
			Name:     t.Name,
			EAN:      t.EAN,
			Brand:    t.Brand,
			Category: strings.Join(cats, " > "),
			Image:    t.Picture,
		})
	})
	return out, nil
}

type adobeProduct struct {
	ID       string  `json:"id"`
	Name     *string `json:"name"`
	EAN      *string `json:"EAN"`
	Brand    *string `json:"brand"`
	Cat1     *string `json:"product_cat1"`
	Cat2     *string `json:"product_cat2"`
	Cat3     *string `json:"product_cat3"`
	Picture  *string `json:"product_url_picture"`
	Notation any     `json:"notation"`
}

func AdobeProducts(body string) ([]model.ProductDetail, error) {
	var raw []adobeProduct
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		return nil, err
	}
	out := make([]model.ProductDetail, 0, len(raw))
	for _, p := range raw {
		cats := make([]string, 0, 3)
		for _, c := range []*string{p.Cat1, p.Cat2, p.Cat3} {
			if c != nil && *c != "" {
				cats = append(cats, *c)
			}
		}
		out = append(out, model.ProductDetail{
			ID:       p.ID,
			Name:     p.Name,
			EAN:      p.EAN,
			Brand:    p.Brand,
			Category: strings.Join(cats, " > "),
			Image:    p.Picture,
			Rating:   toFloatPtr(p.Notation),
		})
	}
	return out, nil
}

func CSRF(body string) string {
	d, err := doc(body)
	if err != nil {
		return ""
	}
	v, _ := d.Find(`input[name="csrf_token"]`).First().Attr("value")
	return v
}

func DataLayer(body string) (map[string]any, error) {
	d, err := doc(body)
	if err != nil {
		return nil, err
	}
	var found map[string]any
	d.Find("script").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		text := s.Text()
		i := strings.Index(text, "var dataLayer")
		if i < 0 {
			return true
		}
		eq := strings.Index(text[i:], "=")
		if eq < 0 {
			return true
		}
		dec := json.NewDecoder(strings.NewReader(text[i+eq+1:]))
		var obj map[string]any
		if dec.Decode(&obj) == nil {
			found = obj
			return false
		}
		return true
	})
	return found, nil
}

var dateRe = regexp.MustCompile(`\d{2}/\d{2}/\d{4}`)

func MinicartDataLayer(body string) map[string]any {
	d, err := doc(body)
	if err != nil {
		return nil
	}
	raw, ok := d.Find("ul[data-tc-minicart-products]").First().
		Attr("data-tc-minicart-products")
	if !ok {
		return nil
	}
	var obj map[string]any
	if json.Unmarshal([]byte(raw), &obj) != nil {
		return nil
	}
	return obj
}

func MinicartIsEmpty(body string) bool {
	return strings.Contains(body, "minicart-next-page-start-upd") &&
		!strings.Contains(body, "data-tc-minicart-products")
}

func CartSummary(dl map[string]any) model.CartSummary {
	out := model.CartSummary{Items: make([]model.CartItem, 0)}
	cart, _ := dl["cart"].(map[string]any)
	rawItems, _ := cart["items"].([]any)
	for _, ri := range rawItems {
		b, err := json.Marshal(ri)
		if err != nil {
			continue
		}
		var ci model.CartItem
		if json.Unmarshal(b, &ci) != nil {
			continue
		}
		out.Items = append(out.Items, ci)
		if ci.Quantity != nil {
			out.TotalQuantity += *ci.Quantity
		}
	}
	out.ItemCount = len(out.Items)
	return out
}

func parseEuro(s string) *float64 {
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = strings.ReplaceAll(s, "€", "")
	s = strings.ReplaceAll(s, ",", ".")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.TrimSpace(s)
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

func strPtr(s string) *string { return &s }

func Orders(body string) ([]model.Order, error) {
	d, err := doc(body)
	if err != nil {
		return nil, err
	}
	out := make([]model.Order, 0)
	seen := map[string]bool{}
	d.Find("div.su-order-block.su-order-completed").Each(func(_ int, s *goquery.Selection) {
		id, ok := s.Attr("data-order-id")
		if !ok || id == "" || seen[id] {
			return
		}
		seen[id] = true
		o := model.Order{ID: id, Items: make([]model.OrderItem, 0)}

		// ponytail: attribute sits on the block or on a nested CTA button
		raw, ok := s.Attr("data-order-products-list")
		if !ok {
			raw, ok = s.Find("[data-order-products-list]").First().
				Attr("data-order-products-list")
		}
		if ok {
			var items []struct {
				ID       any    `json:"id"`
				Name     string `json:"name"`
				Quantity any    `json:"quantity"`
			}
			if json.Unmarshal([]byte(raw), &items) == nil {
				for _, it := range items {
					o.Items = append(o.Items, model.OrderItem{
						ID:       toStr(it.ID),
						Name:     it.Name,
						Quantity: toInt(it.Quantity),
					})
				}
			}
		}

		dateBlock := s.Find("p.su-order-completed-date").First()
		if m := dateRe.FindString(dateBlock.Text()); m != "" {
			o.Date = strPtr(m)
		}
		if ch := strings.TrimSpace(dateBlock.Find("span.su-text-capitalized").
			First().Text()); ch != "" {
			o.Channel = strPtr(ch)
		}
		if st := strings.TrimSpace(s.Find("span.su-order-completed-status").
			First().Text()); st != "" {
			o.Status = strPtr(st)
		}
		numText := strings.Join(strings.Fields(
			s.Find("p.su-order-completed-number").First().Text()), " ")
		if i := strings.Index(numText, " - "); i >= 0 {
			if store := strings.TrimSpace(numText[i+3:]); store != "" {
				o.Store = strPtr(store)
			}
		}
		if tv := s.Find("span.su-order-completed-value").First().Text(); tv != "" {
			o.Total = parseEuro(tv)
		}
		out = append(out, o)
	})
	return out, nil
}

func toStr(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatInt(int64(x), 10)
	default:
		return ""
	}
}

func toInt(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case string:
		n, _ := strconv.Atoi(x)
		return n
	default:
		return 0
	}
}

func toFloatPtr(v any) *float64 {
	switch x := v.(type) {
	case float64:
		return &x
	case string:
		if f, err := strconv.ParseFloat(x, 64); err == nil {
			return &f
		}
	}
	return nil
}
