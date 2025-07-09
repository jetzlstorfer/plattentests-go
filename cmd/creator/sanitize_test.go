package creator

import (
	"testing"
)

func TestSanitizeTrackname(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "removes feat.",
			input:    "Song Title (feat. Other Artist)",
			expected: "Song Title",
		},
		{
			name:     "removes with",
			input:    "Song Title (with Other Artist)",
			expected: "Song Title",
		},
		{
			name:     "removes Bonus",
			input:    "Song Title (Bonus)",
			expected: "Song Title",
		},
		{
			name:     "handles accented characters",
			input:    "Café Münchën",
			expected: "Cafe Munchen",
		},
		{
			name:     "handles special punctuation",
			input:    "Song: Title - With Dashes & Symbols!",
			expected: "Song Title With Dashes Symbols",
		},
		{
			name:     "handles unicode characters",
			input:    "Naïve Résumé",
			expected: "Naive Resume",
		},
		{
			name:     "handles mixed case and special chars",
			input:    "Artist Name - Song Title (feat. Other) & More",
			expected: "Artist Name Song Title",
		},
		{
			name:     "handles quotes and brackets",
			input:    "\"Song Title\" [Remix]",
			expected: "Song Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeTrackname(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeTrackname(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeForComparison(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "handles accented characters",
			input:    "Café Münchën",
			expected: "cafe munchen",
		},
		{
			name:     "handles special punctuation",
			input:    "Song: Title - With Dashes & Symbols!",
			expected: "song title with dashes symbols",
		},
		{
			name:     "handles unicode characters",
			input:    "Naïve Résumé",
			expected: "naive resume",
		},
		{
			name:     "handles mixed case",
			input:    "Artist NAME",
			expected: "artist name",
		},
		{
			name:     "handles quotes and parentheses",
			input:    "\"Song Title\" (Live)",
			expected: "song title live",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeForComparison(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeForComparison(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}