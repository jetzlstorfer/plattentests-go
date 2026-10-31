package creator

import "testing"

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
			expected: "Song Title With Dashes & Symbols",
		},
		{
			name:     "preserves ampersand in artist name",
			input:    "Simon & Garfunkel Track",
			expected: "Simon & Garfunkel Track",
		},
		{
			name:     "preserves ampersands in Spotify search title",
			input:    "Muff Potter Freaks & Geeks & Spinner",
			expected: "Muff Potter Freaks & Geeks & Spinner",
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
		{
			name:     "removes inline feat.",
			input:    "Song Title feat. Other Artist",
			expected: "Song Title",
		},
		{
			name:     "removes inline ft.",
			input:    "Song Title ft. Other Artist",
			expected: "Song Title",
		},
		{
			name:     "removes inline featuring",
			input:    "Song Title featuring Other Artist",
			expected: "Song Title",
		},
		{
			name:     "removes bracketed feat. case-insensitively",
			input:    "Song Title [Feat. Other Artist]",
			expected: "Song Title",
		},
		{
			name:     "removes bracketed featuring",
			input:    "Song Title (featuring Other Artist)",
			expected: "Song Title",
		},
		{
			name:     "keeps words containing with",
			input:    "Dancing With Myself (Without You)",
			expected: "Dancing With Myself (Without You)",
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

func TestSearchTermVariants(t *testing.T) {
	tests := []struct {
		name       string
		searchTerm string
		expected   []string
	}{
		{
			name:       "ampersands also search as and",
			searchTerm: "Muff Potter Freaks & Geeks & Spinner year:2026",
			expected: []string{
				"Muff Potter Freaks & Geeks & Spinner year:2026",
				"Muff Potter Freaks and Geeks and Spinner year:2026",
			},
		},
		{
			name:       "and also searches as ampersand",
			searchTerm: "Artist Freaks and Geeks",
			expected: []string{
				"Artist Freaks and Geeks",
				"Artist Freaks & Geeks",
			},
		},
		{
			name:       "keeps queries without either form unchanged",
			searchTerm: "Artist Plain Song",
			expected:   []string{"Artist Plain Song"},
		},
		{
			name:       "does not replace and inside words",
			searchTerm: "The Sandcastles",
			expected:   []string{"The Sandcastles"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := searchTermVariants(tt.searchTerm)
			if len(result) != len(tt.expected) {
				t.Fatalf("searchTermVariants(%q) = %q, want %q", tt.searchTerm, result, tt.expected)
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("searchTermVariants(%q)[%d] = %q, want %q", tt.searchTerm, i, result[i], tt.expected[i])
				}
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
			expected: "song title with dashes and symbols",
		},
		{
			name:     "ampersand normalizes to and",
			input:    "Ghosts & Errors",
			expected: "ghosts and errors",
		},
		{
			name:     "and matches ampersand form",
			input:    "Ghosts and Errors",
			expected: "ghosts and errors",
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
