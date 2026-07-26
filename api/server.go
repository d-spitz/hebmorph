// Package api serves hebmorph over HTTP. The routing, request binding and
// response types in api.gen.go are generated from openapi.yaml by
// oapi-codegen; everything here is the handful of lines that spec cannot
// write for itself.
package api

//go:generate go tool oapi-codegen -config cfg.yaml openapi.yaml

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"hebmorph"
)

// basePath is the servers[0].url of openapi.yaml; the generated router mounts
// the spec's paths under it.
const basePath = "/api/v1"

//go:embed openapi.yaml
var spec []byte

// server implements the generated StrictServerInterface.
type server struct{ a *hebmorph.Analyzer }

func (s server) AnalyzeWord(_ context.Context, req AnalyzeWordRequestObject) (AnalyzeWordResponseObject, error) {
	if msg, bad := rejectReason(req.Word); bad {
		return AnalyzeWord400JSONResponse{Message: msg}, nil
	}
	return AnalyzeWord200JSONResponse(*s.a.Analyze(req.Word)), nil
}

// badRune matches the first character a word may not contain. Lookup is an
// exact string match, and every one of the 341,585 dictionary entries is built
// from just these characters: the Hebrew letters alef..tav (finals included),
// plus the ASCII apostrophe and quote that stand in for geresh and gershayim
// in acronyms (צה"ל), abbreviations (וכו'), loanword affricates (ג'ינס) and
// gimatria numerals (י"ד). Anything else — niqqud, Latin, digits, whitespace,
// punctuation — could never match, so it is refused rather than answered with
// an empty analysis.
var badRune = regexp.MustCompile(`[^'"\x{05D0}-\x{05EA}]`)

// rejectReason explains why word cannot be analyzed, if it cannot.
func rejectReason(word string) (string, bool) {
	loc := badRune.FindStringIndex(word)
	if loc == nil {
		return "", false
	}
	r, _ := utf8.DecodeRuneInString(word[loc[0]:])

	// Niqqud is the common mistake and the one we can repair, so it gets its
	// own message. The mark is never echoed on its own: a combining character
	// has no standalone glyph and renders as mojibake.
	if unicode.Is(unicode.Mn, r) {
		return fmt.Sprintf(
			"word contains niqqud or cantillation (%U); the dictionary holds only "+
				"undotted ktiv male forms, so send %q instead",
			r, stripMarks(word)), true
	}
	return fmt.Sprintf(
		`word contains %q (%U); expected Hebrew letters only, `+
			`with ' and " permitted for geresh and gershayim`,
		r, r), true
}

// stripMarks removes every nonspacing mark from word. It undoes niqqud, but
// not defective ktiv haser spelling, so the result is a suggestion rather
// than a guaranteed-analyzable word.
func stripMarks(word string) string {
	return strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return r
	}, word)
}

// NewHandler returns an http.Handler serving the API under /api/v1, along with
// the OpenAPI document it was generated from at /api/v1/openapi.yaml.
func NewHandler(a *hebmorph.Analyzer) http.Handler {
	mux := http.NewServeMux()
	HandlerWithOptions(NewStrictHandler(server{a}, nil), StdHTTPServerOptions{
		BaseURL:    basePath,
		BaseRouter: mux,
	})
	mux.HandleFunc("GET "+basePath+"/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.Write(spec)
	})
	return mux
}
