package coursesu

import (
	"testing"

	"github.com/NicolasGuilloux/superu-mcp/internal/model"
)

func TestIsEAN(t *testing.T) {
	for code, want := range map[string]bool{
		"3256225425617": true, "12345670": true, "7634896": false,
		"429937": false, "32562254256AB": false, "": false,
	} {
		if got := isEAN(code); got != want {
			t.Errorf("isEAN(%q) = %v, want %v", code, got, want)
		}
	}
}

func TestQuantityIn(t *testing.T) {
	id, q := "429937", 3
	cart := model.CartSummary{Items: []model.CartItem{{ID: &id, Quantity: &q}}}
	if got := quantityIn(cart, "429937"); got != 3 {
		t.Fatalf("got %d, want 3", got)
	}
	if got := quantityIn(cart, "1"); got != 0 {
		t.Fatalf("got %d, want 0", got)
	}
}
