package inventory

import (
	"strings"
	"testing"

	"pgregory.net/rapid"
)

func TestCurrency_FormatCrypto_Zero(t *testing.T) {
	got := FormatCrypto(0)
	if got != "0 Crypto" {
		t.Fatalf("expected %q got %q", "0 Crypto", got)
	}
}

func TestCurrency_FormatCrypto_One(t *testing.T) {
	got := FormatCrypto(1)
	if got != "1 Crypto" {
		t.Fatalf("expected %q got %q", "1 Crypto", got)
	}
}

func TestCurrency_FormatCrypto_Large(t *testing.T) {
	got := FormatCrypto(1042)
	if got != "1042 Crypto" {
		t.Fatalf("expected %q got %q", "1042 Crypto", got)
	}
}

func TestProperty_FormatCrypto_AlwaysContainsCrypto(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(0, 1_000_000).Draw(rt, "n")
		result := FormatCrypto(n)
		if !strings.Contains(result, "Crypto") {
			rt.Fatalf("FormatCrypto(%d) = %q does not contain 'Crypto'", n, result)
		}
	})
}
