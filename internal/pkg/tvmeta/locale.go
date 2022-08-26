package tvmeta

import (
	"sort"
	"unicode"
)

const (
	defaultLangTag = "en"
	enLangTag      = "en"
	ruLangTag      = "ru"
)

type tag struct {
	tag string
	cnt int
}

// languageTag returns IETF BCP 47 language tag by query or en by default.
func languageTag(input string) string {
	cntMap := make(map[string]int)

	for _, r := range input {
		switch {
		case unicode.Is(unicode.Cyrillic, r):
			cntMap[ruLangTag]++
		case unicode.Is(unicode.Latin, r):
			cntMap[enLangTag]++
		default:
			cntMap[defaultLangTag]++
		}
	}

	cntValues := make([]*tag, 0, len(cntMap))
	for k, v := range cntMap {
		cntValues = append(cntValues, &tag{k, v})
	}

	sort.Slice(cntValues, func(i, j int) bool {
		return cntValues[i].cnt > cntValues[j].cnt
	})

	if len(cntValues) == 0 {
		return defaultLangTag
	}

	return cntValues[0].tag
}
