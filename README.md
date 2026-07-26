# hebmorph

A Hebrew morphological analyzer for Go. Given a word, it returns the ways the
word splits into a known prefix and base, and for each base the possible
morphological readings (part of speech, gender, person, number, tense, construct
state, and pronominal suffix).

It is a modern, UTF-8-native rewrite of the analysis core of
[hspell](http://hspell.ivrix.org.il). The dictionary (≈342k words) is embedded
in the binary, so there is nothing to install or ship alongside it.

## Library

```go
import "hebmorph"

a, err := hebmorph.New()
if err != nil {
    log.Fatal(err)
}
analysis := a.Analyze("מלכה")
// analysis.Exists == true
// analysis.Splits[0].Readings[0].Features.PartOfSpeech == "verb"
```

`Analyzer` is safe for concurrent use; `Analyze` only reads.

## CLI

```sh
go build -o hebmorph ./cmd/hebmorph

echo 'מלכה' | ./hebmorph          # one compact JSON object
echo 'מלכה' | ./hebmorph -pretty  # indented
./hebmorph < words.txt > out.jsonl
```

Example (`-pretty`, trimmed):

```json
{
  "word": "מלכה",
  "exists": true,
  "splits": [
    {
      "prefix": "", "base": "מלכה", "whole_word": true,
      "readings": [
        {
          "stem": "מלך",
          "desc": "פ,נ,3,יחיד,עבר",
          "features": {
            "part_of_speech": "verb", "gender": "feminine",
            "person": "3", "number": "singular", "tense": "past"
          }
        }
      ]
    },
    { "prefix": "מל", "base": "כה", "whole_word": false, "readings": [ ... ] }
  ]
}
```

## HTTP API

```sh
go build -o hebmorphd ./cmd/hebmorphd
./hebmorphd -addr :8080

curl -s http://localhost:8080/api/v1/analyze/מלכה
```

One endpoint, `GET /api/v1/analyze/{word}`, returning the same JSON object the
library and CLI produce. A word the dictionary does not recognize is not an
error — it comes back with an empty `splits` list.

Callers must normalize input themselves: lookup is an exact string match against
undotted ktiv male forms, so niqqud must be stripped and ktiv haser converted to
ktiv male first. Rather than answer un-normalized input with a silently empty
analysis, the endpoint validates against `^['"א-ת]+$` and explains the problem:

```sh
curl -s http://localhost:8080/api/v1/analyze/מַלְכָּה
# 400 {"message":"word contains niqqud or cantillation (U+05B7); the dictionary
#      holds only undotted ktiv male forms, so send \"מלכה\" instead"}
```

That character set is not a guess — every one of the 341,585 dictionary entries
is built from exactly 29 runes: the Hebrew letters alef..tav (finals included)
plus the ASCII `'` and `"` that spell geresh and gershayim. So acronyms (`צה"ל`),
abbreviations (`וכו'`), loanwords (`ג'ינס`) and gimatria numerals (`י"ד`) all pass,
while niqqud, Latin, digits, whitespace and other punctuation are refused.

Ktiv haser is not detectable this way, so `ספר` for `סֵפֶר` is accepted and analyzed
as written.

The API is described by [`api/openapi.yaml`](api/openapi.yaml), which the server
also serves at `/api/v1/openapi.yaml`. Routing, path-parameter binding and the
response types live in `api/api.gen.go`, generated from that spec by
[oapi-codegen](https://github.com/oapi-codegen/oapi-codegen):

```sh
go generate ./api
# == go tool oapi-codegen -config api/cfg.yaml api/openapi.yaml
```

The schemas carry `x-go-type`, so the generated models are aliases for the
existing `hebmorph` types (`type Analysis = hebmorph.Analysis`) rather than
parallel copies — the spec documents the payload without introducing a mapping
layer to keep in sync. The only hand-written code is the handler in
[`api/server.go`](api/server.go), which calls `Analyze` and returns the result.

## Design

- **UTF-8 throughout.** The dictionary is stored in UTF-8 and the analysis walks
  Unicode runes directly — no ISO-8859-8 conversion anywhere at runtime, unlike
  the original C implementation.
- **In-memory maps, no external store.** The whole dictionary fits in a few tens
  of MB and lookups are `map` hits (~ns). SQLite/FST would add a dependency and
  memory-mapping complexity for no measurable benefit at this size, so they were
  deliberately not used.
- **Embedded data.** `data/hebrew.dict.gz` (~2.5 MB) is embedded via `go:embed`;
  the binary is self-contained.

Source files:

| File               | Responsibility                                         |
|--------------------|--------------------------------------------------------|
| `analyzer.go`      | public API (`Analyzer`, `Analyze`), prefix trie        |
| `splits.go`        | prefix/base splitting + validity (waw-doubling rule)   |
| `features.go`      | dmask → `Features`, native Hebrew description, specifier|
| `gimatria.go`      | canonical Hebrew-numeral recognition                   |
| `data.go`          | embedded dictionary decode + lookup                    |
| `prefixes_data.go` | generated legal-prefix table                           |
| `api/openapi.yaml` | HTTP API spec; `api/api.gen.go` is generated from it   |
| `api/server.go`    | the handler behind the generated routing               |

## Regenerating the dictionary

The embedded blob is generated from hspell's original data files (kept under
`internal/gen/source/` for provenance):

```sh
go generate ./...
# == go run ./internal/gen -src internal/gen/source -out data/hebrew.dict.gz
```

## Optional: SQLite export

For deploying the dictionary as a database, an exporter builds a modern SQLite
file from the same blob (the `hebmorph` package itself has no SQLite
dependency):

```sh
go run ./internal/gen/sqlite | sqlite3 dist/hebrew.db
```

The result uses `STRICT` tables, a `WITHOUT ROWID` readings table, foreign keys,
`user_version`, and a convenience view:

```sql
CREATE TABLE words (
  id   INTEGER PRIMARY KEY,
  word TEXT    NOT NULL,
  spec INTEGER NOT NULL                       -- prefix specifier
) STRICT;

CREATE TABLE readings (
  word_id INTEGER NOT NULL REFERENCES words(id),
  seq     INTEGER NOT NULL,
  stem_id INTEGER NOT NULL REFERENCES words(id),
  dmask   INTEGER NOT NULL,                    -- morphology bitmask
  PRIMARY KEY (word_id, seq)
) STRICT, WITHOUT ROWID;

CREATE VIEW readings_view AS                   -- readings with resolved text
  SELECT r.word_id, w.word AS word, r.seq, s.word AS stem, r.dmask
  FROM readings r JOIN words w ON w.id = r.word_id JOIN words s ON s.id = r.stem_id;
```

```sh
sqlite3 dist/hebrew.db "SELECT word, stem, dmask FROM readings_view WHERE word='מלכה';"
```

Note the `readings` table holds every reading of a word; an application applies
the prefix-specifier filter (see `prefixSpecifier`) itself, exactly as the Go
analyzer does. `dmask` can be decoded with the same bit layout as the package's
`Features`.

## Tests & benchmarks

```sh
go test -race ./...
go test -run=^$ -bench=. -benchmem
```

Measured: `New` (load whole dictionary) ~95 ms; `Analyze` ~0.8 µs/word
(~1.3M words/s). Analyzing the full 342k-word dictionary takes ~0.7 s end to end.

## Provenance & correctness

The analyzer is a port of Hspell 1.4. Its JSON output was verified to be
**byte-for-byte identical** to the verified reference port across all 341,585
dictionary words, and Hspell's own C output was reproduced exactly in that
reference (see the sibling `hspell-1.4` project).

## License & credits

hebmorph is derived from **Hspell** — copyright © 2000–2017 Nadav Har'El and Dan
Kenigsberg, http://hspell.ivrix.org.il — and embeds Hspell's Hebrew dictionary
data (re-encoded to UTF-8). Hspell, including its dictionary files, is licensed
under the **GNU Affero General Public License, version 3**, so hebmorph is too.

See [`LICENSE`](LICENSE) for the full AGPLv3 text and [`NOTICE`](NOTICE) for the
attribution details. There is no warranty of any kind.
