package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"hebmorph"
	"hebmorph/api"
)

// analyzer is shared: loading the dictionary costs ~95ms and Analyze is
// safe for concurrent use.
var analyzer = sync.OnceValues(hebmorph.New)

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	a, err := analyzer()
	if err != nil {
		t.Fatal(err)
	}
	s := httptest.NewServer(api.NewHandler(a))
	t.Cleanup(s.Close)
	return s
}

func get(t *testing.T, url string) *http.Response {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status = %d, want 200", url, res.StatusCode)
	}
	return res
}

// TestAnalyzeWord checks that the endpoint returns exactly what the library
// produces, unchanged by the generated transport.
func TestAnalyzeWord(t *testing.T) {
	s := newServer(t)
	res := get(t, s.URL+"/api/v1/analyze/%D7%9E%D7%9C%D7%9B%D7%94") // מלכה

	if got := res.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	var got hebmorph.Analysis
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	a, _ := analyzer()
	if gotJSON, wantJSON := mustJSON(t, got), mustJSON(t, a.Analyze("מלכה")); gotJSON != wantJSON {
		t.Errorf("response =\n%s\nwant\n%s", gotJSON, wantJSON)
	}
}

// TestAccepts covers the characters the dictionary actually contains: Hebrew
// letters plus the apostrophe and quote standing in for geresh and gershayim.
// A scan of all 341,585 entries finds no others.
func TestAccepts(t *testing.T) {
	s := newServer(t)
	for _, word := range []string{
		"מלכה",  // plain
		`צה"ל`,  // acronym, gershayim
		"וכו'",  // abbreviation, geresh
		"ג'ינס", // loanword affricate
		`י"ד`,   // gimatria numeral
	} {
		res := get(t, s.URL+"/api/v1/analyze/"+url.PathEscape(word))
		res.Body.Close()
	}
}

// TestRejects checks that input the dictionary could never match is a
// descriptive 400 rather than a silently empty analysis.
func TestRejects(t *testing.T) {
	s := newServer(t)
	for _, tc := range []struct{ word, wantInMessage string }{
		{"מַלְכָּה", "מלכה"},    // niqqud: message suggests the undotted form
		{"סֵ֑פֶר", "ספר"},       // cantillation, likewise
		{"שלום עולם", "U+0020"}, // space
		{"hello", "U+0068"},     // latin
		{"שלום123", "U+0031"},   // digits
		{"שלום!", "U+0021"},     // punctuation
		{"שלום״", "U+05F4"},     // typographic gershayim, not the ASCII one
	} {
		res, err := http.Get(s.URL + "/api/v1/analyze/" + url.PathEscape(tc.word))
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != http.StatusBadRequest {
			res.Body.Close()
			t.Errorf("%q: status = %d, want 400", tc.word, res.StatusCode)
			continue
		}
		var got api.Error
		err = json.NewDecoder(res.Body).Decode(&got)
		res.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got.Message, tc.wantInMessage) {
			t.Errorf("%q: message = %q, want it to mention %q", tc.word, got.Message, tc.wantInMessage)
		}
	}
}

func TestServesSpec(t *testing.T) {
	s := newServer(t)
	get(t, s.URL+"/api/v1/openapi.yaml")
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
