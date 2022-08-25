package locale

import "unicode"

// LanguageTag returns IETF BCP 47 language tag by query or en by default.
func LanguageTag(input string) string {
	for _, r := range input {
		switch {
		case unicode.Is(unicode.Cyrillic, r):
			return "ru"
		default:
			return "en"
		}
	}

	return "en"
}
