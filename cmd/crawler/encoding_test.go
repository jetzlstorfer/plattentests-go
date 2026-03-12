package crawler

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestNewDocumentFromPlattentestsResponse_DecodesISO88591(t *testing.T) {
	brokenLikeInput := "<html><body><p>Frickelbande aus Bayern, die sich in ihren Schaltkreisen verliert, gr\xf6\xdftes Album, Hardcore \xfcber Indierock, organische Kl\xe4nge, Karrierebeginns \x96 schon eher</p></body></html>"

	res := &http.Response{
		Header: http.Header{"Content-Type": []string{"text/html; charset=iso-8859-1"}},
		Body:   io.NopCloser(strings.NewReader(brokenLikeInput)),
	}

	doc, err := newDocumentFromPlattentestsResponse(res)
	if err != nil {
		t.Fatalf("newDocumentFromPlattentestsResponse returned error: %v", err)
	}

	text := doc.Text()

	checks := []string{"gr\u00f6\u00dftes", "\u00fcber", "Kl\u00e4nge", "Karrierebeginns \u2013 schon eher"}
	for _, want := range checks {
		if !strings.Contains(text, want) {
			t.Fatalf("expected decoded text to contain %q, got %q", want, text)
		}
	}

	if strings.Contains(text, "\ufffd") {
		t.Fatalf("expected decoded text without replacement characters, got %q", text)
	}

	if strings.ContainsRune(text, '\u0096') {
		t.Fatalf("expected decoded text without C1 control U+0096, got %q", text)
	}
}
