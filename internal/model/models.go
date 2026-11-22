package model

type Product struct {
	ID       string  `json:"id"`
	Name     *string `json:"name"`
	EAN      *string `json:"ean"`
	Brand    *string `json:"brand"`
	Category string  `json:"category"`
	Image    *string `json:"image"`
}

type CartItem struct {
	ID       *string `json:"id"`
	Name     *string `json:"name"`
	EAN      *string `json:"ean"`
	Brand    *string `json:"brand"`
	Price    *string `json:"price"` // the site sends price as a JSON string ("1.78")
	Quantity *int    `json:"quantity"`
}

type CartSummary struct {
	Items         []CartItem `json:"items"`
	ItemCount     int        `json:"item_count"`
	TotalQuantity int        `json:"total_quantity"`
}

type CartMutationResult struct {
	StatusCode int          `json:"status_code"`
	OK         bool         `json:"ok"`
	Cart       *CartSummary `json:"cart"`
}

type FavoritesResult struct {
	Count        int       `json:"count"`
	PagesFetched int       `json:"pages_fetched"`
	Products     []Product `json:"products"`
}

type LoginResult struct {
	Status           string   `json:"status"`
	Source           string   `json:"source"`
	CookiesCaptured  int      `json:"cookies_captured"`
	SessionFile      string   `json:"session_file"`
	CookieNames      []string `json:"cookie_names"`
	SessionstorePath *string  `json:"sessionstore_path"`
}

type OrderItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

type Order struct {
	ID      string      `json:"id"`
	Date    *string     `json:"date"`
	Channel *string     `json:"channel"`
	Status  *string     `json:"status"`
	Store   *string     `json:"store"`
	Total   *float64    `json:"total"`
	Items   []OrderItem `json:"items"`
}

type HistoryResult struct {
	Count  int     `json:"count"`
	Orders []Order `json:"orders"`
}

type ProductDetail struct {
	ID       string   `json:"id"`
	Name     *string  `json:"name"`
	EAN      *string  `json:"ean"`
	Brand    *string  `json:"brand"`
	Category string   `json:"category"`
	Image    *string  `json:"image"`
	Rating   *float64 `json:"rating"`
}
