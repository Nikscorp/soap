package tvmeta

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLanguageTag(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expectedTag string
	}{
		{
			name:        "englishTag",
			input:       "blabla",
			expectedTag: enLangTag,
		},
		{
			name:        "russianTag",
			input:       "блабла",
			expectedTag: ruLangTag,
		},
		{
			name:        "defaultTag",
			input:       "😱😱😱",
			expectedTag: defaultLangTag,
		},
		{
			name:        "composedTagRu",
			input:       "блабла" + "bla",
			expectedTag: ruLangTag,
		},
		{
			name:        "composedTagEn",
			input:       "blabla" + "бла",
			expectedTag: enLangTag,
		},
		{
			name:        "composedTagDefault",
			input:       "😱😱😱😱" + "бла",
			expectedTag: defaultLangTag,
		},
		{
			name:        "empty",
			input:       "",
			expectedTag: defaultLangTag,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expectedTag, languageTag(tc.input))
		})
	}
}
