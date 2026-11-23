package coursesu

import (
	"context"
	"fmt"
	"unicode"

	"github.com/NicolasGuilloux/superu-mcp/internal/model"
)

// isEAN reports whether code looks like a barcode (EAN-8, UPC-A, EAN-13,
// GTIN-14) rather than a coursesu product id, which is 6 or 7 digits.
func isEAN(code string) bool {
	switch len(code) {
	case 8, 12, 13, 14:
	default:
		return false
	}
	for _, r := range code {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// ResolveProductID turns an EAN into the coursesu product id by searching
// for it. Product ids are returned unchanged.
func ResolveProductID(ctx context.Context, code string) (string, error) {
	if !isEAN(code) {
		return code, nil
	}
	products, err := Search(ctx, code, 0)
	if err != nil {
		return "", err
	}
	for _, p := range products {
		if p.EAN != nil && *p.EAN == code {
			return p.ID, nil
		}
	}
	return "", newErr(fmt.Sprintf("no product with EAN %s on coursesu.com", code), nil)
}

// CartQuantity returns how many of a product the cart holds.
func CartQuantity(ctx context.Context, productID string) (int, error) {
	cart, err := CartList(ctx)
	if err != nil {
		return 0, err
	}
	return quantityIn(cart, productID), nil
}

func quantityIn(cart model.CartSummary, productID string) int {
	for _, it := range cart.Items {
		if it.ID != nil && *it.ID == productID && it.Quantity != nil {
			return *it.Quantity
		}
	}
	return 0
}

// CartIncrease adds qty of a product (an id or an EAN) to what the cart
// already holds. Cart-AddProduct itself sets the quantity, so it reads the
// cart first.
func CartIncrease(ctx context.Context, code string, qty int) (model.CartMutationResult, error) {
	pid, err := ResolveProductID(ctx, code)
	if err != nil {
		return model.CartMutationResult{}, err
	}
	have, err := CartQuantity(ctx, pid)
	if err != nil {
		return model.CartMutationResult{}, err
	}
	return CartAdd(ctx, pid, have+qty)
}

// CartRemoveQuantity removes qty of a product from the cart; qty 0 or more
// than the cart holds removes it entirely.
func CartRemoveQuantity(ctx context.Context, productID string, qty int) (model.CartMutationResult, error) {
	have, err := CartQuantity(ctx, productID)
	if err != nil {
		return model.CartMutationResult{}, err
	}
	if have == 0 {
		return model.CartMutationResult{}, newErr("this product is not in the cart", nil)
	}
	if qty <= 0 || qty >= have {
		return CartRemove(ctx, productID)
	}
	return CartSet(ctx, productID, have-qty)
}

// FindOrder returns one past order by its number.
func FindOrder(ctx context.Context, number string) (model.Order, error) {
	h, err := History(ctx, 0)
	if err != nil {
		return model.Order{}, err
	}
	for _, o := range h.Orders {
		if o.ID == number {
			return o, nil
		}
	}
	return model.Order{}, newErr(fmt.Sprintf("no order %s in the order history", number), nil)
}
