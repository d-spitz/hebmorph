# hebmorph

A Hebrew morphological analyzer for Go. Given a word, it returns the ways the
word splits into a known prefix and base, and for each base the possible
morphological readings (part of speech, gender, person, number, tense, construct
state, and pronominal suffix).

It is a modern, UTF-8-native rewrite of the analysis core of
[hspell](http://hspell.ivrix.org.il). The dictionary (≈342k words) is embedded
in the binary, so there is nothing to install or ship alongside it. Each reading
also carries English glosses for its stem, so an analysis says what the word
means as well as how it is built.

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
          },
          "glosses": ["to reign", "to rule"]
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

Responses carry `Access-Control-Allow-Origin: *` so browsers on other origins
can read them — the web portal below is served from GitHub Pages, so every call
it makes is cross-origin.

## English glosses

Every reading carries `glosses`, the English senses of its lemma:

```json
{ "stem": "מלך", "desc": "פ,נ,3,יחיד,עבר", "features": { "part_of_speech": "verb", ... },
  "glosses": ["to reign", "to rule"] }
```

Translation is **per lemma and part of speech**, not per inflected form. The
342k words collapse onto 22,412 lemmas, and tense, person and number do not
change what a word means, so the table only has to hold a sense set per lemma —
291 KB gzipped against the dictionary's 2.5 MB.

Part of speech does matter: 1,153 stems are attested under more than one, and
`מלך` means "to reign" as a verb and "king" as a noun. So each reading is
answered with the glosses of its own part of speech, which is why the three
readings of `מלכה` come back with different English:

```sh
curl -s http://localhost:8080/api/v1/analyze/מלכה | jq '.splits[0].readings[] | {desc, glosses}'
# {"desc":"פ,נ,3,יחיד,עבר","glosses":["to reign","to rule"]}   verb, stem מלך
# {"desc":"ע,נ,יחיד","glosses":["queen"]}                      noun, stem מלכה
# {"desc":"ע,ז,יחיד,כינוי/נ,3,יחיד","glosses":["king"]}        noun, stem מלך
```

### Words with no stem

Most lemmas are stems, but 1,633 words have none. hspell files proper nouns
(`אביגיל`), acronyms (`צה"ל`), and particles (`אולי`, `עכשיו`, `אבל`) under a
catch-all "stem" `שונות` for want of a real one — so they are their own lemmas,
keyed by themselves rather than by a stem.

A word can be both, in the same part of speech, which is why the distinction is
recorded rather than inferred. `אדם` is a common noun through its stem and a
name through the catch-all, and the two readings are glossed apart:

```sh
curl -s http://localhost:8080/api/v1/analyze/אדם | jq '.splits[0].readings[] | {stem, glosses}'
# {"stem":"אדם","glosses":["man","person","human being"]}
# {"stem":"שונות","glosses":["Adam"]}       the proper-noun reading
```

The field is omitted when a lemma has no gloss for the reading's part of
speech. 22,410 of the 22,412 lemmas are translated; a `TestGlossCoverage` guard
fails the build if a regeneration drops below 99%.

The glosses are machine-generated and machine-verified, not lexicographer-
written — see [Provenance & correctness](#provenance--correctness). They are
meant as a reading aid, not as a dictionary of record.

## Web portal

[`web/`](web/) is a static reader: paste Hebrew text, then tap any word for its
analysis. Texts live in `localStorage`, so there is no database and no account.

Three files, no dependencies, no build step: `index.html`, `styles.css`, and
about 200 lines of `app.js`. Open `web/index.html` directly, or let
[the workflow](.github/workflows/pages.yml) publish it to GitHub Pages on push
to `main` (Settings → Pages → Source → "GitHub Actions").

The portal normalizes before querying — stripping niqqud and folding typographic
geresh/gershayim to ASCII — so pointed text does not simply 400.

## Design

- **UTF-8 throughout.** The dictionary is stored in UTF-8 and the analysis walks
  Unicode runes directly — no ISO-8859-8 conversion anywhere at runtime, unlike
  the original C implementation.
- **In-memory maps, no external store.** The whole dictionary fits in a few tens
  of MB and lookups are `map` hits (~ns). SQLite/FST would add a dependency and
  memory-mapping complexity for no measurable benefit at this size, so they were
  deliberately not used.
- **Embedded data.** `data/hebrew.dict.gz` (~2.5 MB) and
  `data/translations.json.gz` (~291 KB) are embedded via `go:embed`; the binary
  is self-contained.
- **Glosses keyed by lemma index.** The translation table reuses the word index
  a reading already carries in `stemIndex`, so attaching English to a reading is
  one map hit and no string work.
- **Packed bytes only where they pay.** The dictionary blob is a transcription
  of hspell's own compact on-disk encoding. The gloss table, a fourteenth its
  size, is plain gzipped JSON: packing it by hand saves 4% of bytes and 15 ms of
  one-time decoding, which does not pay for a codec to keep in sync on both
  sides of the build — and `zcat | jq` reads what actually shipped.

Source files:

| File               | Responsibility                                         |
|--------------------|--------------------------------------------------------|
| `analyzer.go`      | public API (`Analyzer`, `Analyze`), prefix trie        |
| `splits.go`        | prefix/base splitting + validity (waw-doubling rule)   |
| `features.go`      | dmask → `Features`, native Hebrew description, specifier|
| `gimatria.go`      | canonical Hebrew-numeral recognition                   |
| `data.go`          | embedded dictionary decode + lookup                    |
| `translations.go`  | embedded gloss table decode + per-lemma, per-POS lookup |
| `prefixes_data.go` | generated legal-prefix table                           |
| `api/openapi.yaml` | HTTP API spec; `api/api.gen.go` is generated from it   |
| `api/server.go`    | the handler behind the generated routing               |
| `web/`             | static reader portal (vanilla JS + localStorage)       |

## Regenerating the embedded data

Both files are built by `go generate`, in order — the gloss table joins on
Hebrew text, so the dictionary has to exist first:

```sh
go generate ./...
# == go run ./internal/gen -src internal/gen/source -out data/hebrew.dict.gz
# && go run ./internal/gen/translations \
#      -stems internal/gen/translate/translations.verified.jsonl \
#      -misc  internal/gen/translate/misc.verified.jsonl \
#      -dict data/hebrew.dict.gz -out data/translations.json.gz
```

The dictionary comes from hspell's original data files, the glosses from the two
verified translation sets — one per stem, one for the catch-all words that have
none. All sources are kept under `internal/gen/` for provenance. The translation
step reports coverage and drops any gloss whose part of speech the lemma has no
reading in: nothing could carry it.

## Optional: SQLite export

For deploying the dictionary as a database, an exporter builds a modern SQLite
file from the same two blobs (the `hebmorph` package itself has no SQLite
dependency):

```sh
go run ./internal/gen/sqlite | sqlite3 dist/hebrew.db
```

The result uses `STRICT` tables, `WITHOUT ROWID` where the natural key is the
whole row, foreign keys, `user_version` (2 — 1 was the dictionary without
glosses), and two convenience views:

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

CREATE TABLE glosses (
  lemma_id INTEGER NOT NULL REFERENCES words(id),
  pos      INTEGER NOT NULL,                   -- dmask & 3: 1 noun, 2 verb, 3 adjective
  bucket   INTEGER NOT NULL,                   -- 1 when the lemma is the word itself
  seq      INTEGER NOT NULL,                   -- sense order, most representative first
  english  TEXT    NOT NULL,
  PRIMARY KEY (lemma_id, pos, bucket, seq)
) STRICT, WITHOUT ROWID;

CREATE VIEW readings_view AS                   -- readings with resolved text,
  SELECT r.word_id, w.word AS word, r.seq,     -- plus the lemma each is glossed by
         r.stem_id, s.word AS stem, r.dmask,
         CASE WHEN r.stem_id = 300672 THEN r.word_id ELSE r.stem_id END AS lemma_id,
         r.stem_id = 300672 AS bucket          -- 300672 is שונות, hspell's catch-all
  FROM readings r JOIN words w ON w.id = r.word_id JOIN words s ON s.id = r.stem_id;

CREATE VIEW glosses_view AS                    -- glosses with resolved lemma text
  SELECT g.lemma_id, l.word AS lemma, g.pos, g.bucket, g.seq, g.english
  FROM glosses g JOIN words l ON l.id = g.lemma_id;
```

```sh
sqlite3 dist/hebrew.db "SELECT word, stem, dmask FROM readings_view WHERE word='מלכה';"

# a reading and its English, the same match the Go analyzer makes. readings_view
# already resolves which lemma a reading is glossed by, so the join does not
# have to repeat the catch-all rule.
sqlite3 -header -column dist/hebrew.db "
  SELECT r.word, r.stem, g.english FROM readings_view r
    JOIN glosses g ON g.lemma_id = r.lemma_id AND g.bucket = r.bucket
                  AND g.pos = r.dmask & 3
    WHERE r.word = 'אדם';"
# word  stem   english
# אדם   אדם    man
# אדם   אדם    person
# אדם   אדם    human being
# אדם   שונות  Adam
```

Note the `readings` table holds every reading of a word; an application applies
the prefix-specifier filter (see `prefixSpecifier`) itself, exactly as the Go
analyzer does. `dmask` can be decoded with the same bit layout as the package's
`Features`, and its low two bits are the `pos` that joins to `glosses`.

## Tests & benchmarks

```sh
go test -race ./...
go test -run=^$ -bench=. -benchmem
```

Measured: `New` (load dictionary + gloss table) ~110 ms; `Analyze` ~0.9 µs/word
(~1.3M words/s). Analyzing the full 342k-word dictionary takes ~0.7 s end to end.

## Provenance & correctness

The analyzer is a port of Hspell 1.4. Its JSON output was verified to be
**byte-for-byte identical** to the verified reference port across all 341,585
dictionary words, and Hspell's own C output was reproduced exactly in that
reference (see the sibling `hspell-1.4` project).

The **glosses have a weaker guarantee** and are held separately from the
morphology for exactly that reason: nothing about the analysis depends on them.
The 20,917 stems were drafted by [DictaLM-3.0-Nemotron-12B-Instruct](https://huggingface.co/dicta-il/DictaLM-3.0-Nemotron-12B-Instruct)
— the Hebrew-specialized model from [Dicta](https://dicta.org.il), the Israel
Center for Text Analysis — run locally, each stem given its attested parts of
speech and one inflected form as grounding. Every draft gloss was then reviewed
by **Google Gemini** in a second pass, which corrected the senses it judged
wrong. The 1,633 words with no stem — proper nouns, acronyms and particles —
were done separately by **Gemini 3.5 Flash Lite**, since a name or a particle
needs a conventional English form rather than a translated sense.

`internal/gen/translate/translations.verified.jsonl` and `misc.verified.jsonl`
are those two reviewed sets, and the only inputs the embedded table is built
from.

That is machine drafting checked by machine review, not lexicography. It is good
enough to read with and wrong often enough that it should not be cited. Errors
of the kind the review pass exists to catch are real: `אימת` was first glossed
"to frighten" (the unrelated root א־י־ם) rather than "to verify" (א־מ־ת, *emet*,
truth). Corrections belong in the verified set, followed by `go generate ./...`.

## License & credits

hebmorph is derived from **Hspell** — copyright © 2000–2017 Nadav Har'El and Dan
Kenigsberg, http://hspell.ivrix.org.il — and embeds Hspell's Hebrew dictionary
data (re-encoded to UTF-8). Hspell, including its dictionary files, is licensed
under the **GNU Affero General Public License, version 3**, so hebmorph is too.

See [`LICENSE`](LICENSE) for the full AGPLv3 text and [`NOTICE`](NOTICE) for the
attribution details. There is no warranty of any kind.
