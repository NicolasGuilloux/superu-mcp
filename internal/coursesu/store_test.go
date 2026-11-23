package coursesu

import "testing"

func TestParseStores(t *testing.T) {
	body := `<ul><li id="33565" class="store-container" data-store-id="33565">
<h3 class="store-name" data-store-name>Super U - Rennes St Jacques</h3>
<p class="store-distance su-font-open"><span class="bold-text">4,9</span> km</p>
<div class="store-address su-font-open" data-cs-mask>256 rue de nantes, 35136 St Jacques</div>
<div class="store-delivery-option drive-option mode-RETRAIT "><p class="store-available-time-slot">Drive <span>dès aujourd&rsquo;hui</span></p></div>
</li><li id="2" class="store-container" data-store-id="2"><h3 data-store-name>U Express</h3></li></ul>`
	got := parseStores(body)
	if len(got) != 2 {
		t.Fatalf("got %d stores, want 2", len(got))
	}
	want := Store{ID: "33565", Name: "Super U - Rennes St Jacques", Address: "256 rue de nantes, 35136 St Jacques",
		Distance: "4,9", Drive: "Drive dès aujourd’hui"}
	if got[0] != want {
		t.Errorf("got %+v, want %+v", got[0], want)
	}
	if got[1].ID != "2" || got[1].Name != "U Express" || got[1].Drive != "" {
		t.Errorf("second store: %+v", got[1])
	}
}
