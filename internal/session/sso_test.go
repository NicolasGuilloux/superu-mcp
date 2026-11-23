package session

import "testing"

func TestSSOValue(t *testing.T) {
	cases := map[string]string{
		"abc.*DEF*": "abc.*DEF*",
		"  abc  ":   "abc",
		"OAUTH_X=1; amlbcookie=01; b870a934f1cc0fa=tok.*AA*; _cfuvid=z": "tok.*AA*",
		"Cookie: b870a934f1cc0fa=tok":                                   "tok",
		"b870a934f1cc0fa=tok=":                                          "tok=",
		"a=1; b=2":                                                      "",
	}
	for in, want := range cases {
		if got := ssoValue(in); got != want {
			t.Errorf("ssoValue(%q) = %q, want %q", in, got, want)
		}
	}
}
