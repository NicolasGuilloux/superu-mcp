package parse

import (
	"os"
	"testing"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

func TestProducts(t *testing.T) {
	body := readFixture(t, "search.html")
	got, err := Products(body)
	if err != nil {
		t.Fatalf("Products: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 products (deduped), got %d", len(got))
	}
	p := got[0]
	if p.ID != "7131519" || p.Name == nil || *p.Name != "Pain de mie" {
		t.Fatalf("product[0] wrong: %+v", p)
	}
	if p.Category != "Épicerie > Pain > Mie" {
		t.Fatalf("category wrong: %q", p.Category)
	}
	if got[1].Brand != nil {
		t.Fatalf("product[1].Brand should be nil, got %v", *got[1].Brand)
	}
}

func TestCSRF(t *testing.T) {
	body := readFixture(t, "search.html")
	if got := CSRF(body); got != "TOKEN-ABC-123" {
		t.Fatalf("CSRF want TOKEN-ABC-123, got %q", got)
	}
}

func TestDataLayer(t *testing.T) {
	body := readFixture(t, "search.html")
	dl, err := DataLayer(body)
	if err != nil {
		t.Fatalf("DataLayer: %v", err)
	}
	if dl == nil {
		t.Fatal("DataLayer returned nil")
	}
}

func TestMinicart(t *testing.T) {
	body := readFixture(t, "minicart.html")
	dl := MinicartDataLayer(body)
	if dl == nil {
		t.Fatal("MinicartDataLayer returned nil")
	}
	cart := CartSummary(dl)
	if cart.ItemCount != 2 || cart.TotalQuantity != 3 {
		t.Fatalf("cart summary wrong: %+v", cart)
	}
}

func TestMinicartEmpty(t *testing.T) {
	body := readFixture(t, "minicart_empty.html")
	if MinicartDataLayer(body) != nil {
		t.Fatal("empty minicart should have no data-tc-minicart-products")
	}
	if !MinicartIsEmpty(body) {
		t.Fatal("MinicartIsEmpty should be true for the empty fragment")
	}
	if MinicartIsEmpty(readFixture(t, "minicart.html")) {
		t.Fatal("populated minicart wrongly reported empty")
	}
}

func TestAdobeProducts(t *testing.T) {
	body := readFixture(t, "adobe_products.json")
	got, err := AdobeProducts(body)
	if err != nil {
		t.Fatalf("AdobeProducts: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 product, got %d", len(got))
	}
	p := got[0]
	if p.ID != "441457" || p.Name == nil ||
		*p.Name != "Beurre moulé demi-sel 80%MG, plaquette de 500g" {
		t.Fatalf("product fields wrong: %+v", p)
	}
	if p.EAN == nil || *p.EAN != "3256224252054" {
		t.Fatalf("EAN wrong: %v", p.EAN)
	}
	if p.Category != "Produits laitiers, oeufs et fromages > "+
		"Beurres, crèmes et oeufs > Beurres" {
		t.Fatalf("category wrong: %q", p.Category)
	}
	if p.Rating == nil || *p.Rating != 4.6348 {
		t.Fatalf("rating wrong: %v", p.Rating)
	}
}

func TestToFloatPtr(t *testing.T) {
	if v := toFloatPtr(4.6348); v == nil || *v != 4.6348 {
		t.Fatalf("number: got %v", v)
	}
	if v := toFloatPtr("3.5"); v == nil || *v != 3.5 {
		t.Fatalf("numeric string: got %v", v)
	}
	if toFloatPtr("unknown") != nil {
		t.Fatal("non-numeric string should yield nil")
	}
}

func TestOrders(t *testing.T) {
	body := readFixture(t, "orders.html")
	orders, err := Orders(body)
	if err != nil {
		t.Fatalf("Orders: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("want 1 completed order (current ignored), got %d", len(orders))
	}
	o := orders[0]
	if o.ID != "48124163" {
		t.Fatalf("order id wrong: %q", o.ID)
	}
	if o.Date == nil || *o.Date != "14/05/2026" {
		t.Fatalf("order date wrong: %v", o.Date)
	}
	if o.Channel == nil || *o.Channel != "Drive" {
		t.Fatalf("order channel wrong: %v", o.Channel)
	}
	if o.Status == nil || *o.Status != "Retirée" {
		t.Fatalf("order status wrong: %v", o.Status)
	}
	if o.Store == nil || *o.Store != "Hyper U la chapelle-sur-erdre" {
		t.Fatalf("order store wrong: %v", o.Store)
	}
	if o.Total == nil || *o.Total != 87.50 {
		t.Fatalf("order total wrong: %v", o.Total)
	}
	if len(o.Items) != 2 || o.Items[0].Quantity != 2 {
		t.Fatalf("order items wrong: %+v", o.Items)
	}
}

func TestParseEuro(t *testing.T) {
	cases := map[string]float64{
		"87,50 €":    87.50,
		"1 234,56 €": 1234.56,
		"0,99 €":     0.99,
	}
	for in, want := range cases {
		got := parseEuro(in)
		if got == nil || *got != want {
			t.Fatalf("parseEuro(%q) = %v, want %v", in, got, want)
		}
	}
	if parseEuro("not a price") != nil {
		t.Fatal("parseEuro should return nil on garbage")
	}
}
