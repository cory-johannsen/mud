package inventory

import "fmt"

// FormatCrypto returns a human-readable currency string for the given total.
//
// Precondition: total >= 0.
// Postcondition: returned string is "{total} Crypto".
func FormatCrypto(total int) string {
	return fmt.Sprintf("%d Crypto", total)
}
