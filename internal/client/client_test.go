package client

import (
	"errors"
	"testing"

	fhttp "github.com/bogdanfinn/fhttp"
)

func TestRaiseIfExpired(t *testing.T) {
	expired := &fhttp.Response{
		StatusCode: 302,
		Header:     fhttp.Header{"Location": {"/connexion?x=1"}},
	}
	err := raiseIfExpired(expired)
	var se *SessionExpiredError
	if !errors.As(err, &se) {
		t.Fatalf("want *SessionExpiredError, got %v", err)
	}
	if se.Location != "/connexion?x=1" {
		t.Fatalf("location wrong: %q", se.Location)
	}

	ok := &fhttp.Response{StatusCode: 200, Header: fhttp.Header{}}
	if raiseIfExpired(ok) != nil {
		t.Fatal("200 should not be expired")
	}

	otherRedirect := &fhttp.Response{
		StatusCode: 302,
		Header:     fhttp.Header{"Location": {"/somewhere-else"}},
	}
	if raiseIfExpired(otherRedirect) != nil {
		t.Fatal("302 to non-/connexion should not be expired")
	}
}
