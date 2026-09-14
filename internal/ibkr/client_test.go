package ibkr

import "testing"

func TestNewTrimsSlash(t *testing.T) {
	if New("http://x/").BaseURL != "http://x" {
		t.Fatal("BaseURL")
	}
}
