package character

import "math/rand"

// StandardGenders is the list of built-in gender values offered during character creation.
var StandardGenders = []string{"male", "female", "non-binary", "indeterminate"}

// RandomStandardGender returns a random value from StandardGenders.
func RandomStandardGender() string {
	return StandardGenders[rand.Intn(len(StandardGenders))]
}
