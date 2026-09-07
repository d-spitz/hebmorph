# hebmorph

A Hebrew morphological analyzer for Go. Given a word, it returns the ways the
word splits into a known prefix and base, and for each base the possible
morphological readings (part of speech, gender, person, number, tense, construct
state, and pronominal suffix).

It is a modern, UTF-8-native rewrite of the analysis core of
[hspell](http://hspell.ivrix.org.il). The dictionary (≈342k words) is embedded
in the binary, so there is nothing to install or ship alongside it. Each reading
also names the distinct word it means, with English glosses and a Modern Hebrew
frequency rank, so an analysis says what the word means as well as how it is
built — likeliest reading first.

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

## Lemmas: what a reading means

Every reading names a `lemma` — the distinct word it means — and carries that
lemma's English:

```json
{ "stem": "מלך", "desc": "פ,נ,3,יחיד,עבר", "lemma": "מלך",
  "features": { "part_of_speech": "verb", ... },
  "glosses": ["to reign", "to rule"], "rank": 661 }
```

**A lemma is one word, not one spelling.** Hebrew writes no vowels, so
unrelated words collide. `את` is three of them — the direct-object marker, the
preposition "with", and the pronoun "you" — and each reading gets its own:

```sh
curl -s http://localhost:8080/api/v1/analyze/את | jq -c '.splits[0].readings[] | {lemma, part_of_speech: .features.part_of_speech, glosses}'
# {"lemma":"את","part_of_speech":"particle","glosses":["direct object marker"]}
# {"lemma":"את","part_of_speech":"pronoun","glosses":["you (feminine singular)"]}
# {"lemma":"את","part_of_speech":"noun","glosses":["plowshare","mattock"]}
```

Conversely one lemma answers for many words: tense, person and number do not
change meaning, so the 342k words collapse onto 23,729 lemmas.

The lemma is usually the stem, but the two are different ideas and the API
reports both. Names, acronyms and particles have no stem — hspell files them
all under a catch-all `שונות` — so they are their own lemma, which is why `אדם`
reads "man, person" through its stem and "Adam" through its proper-noun
reading, both under the lemma `אדם`:

```sh
curl -s http://localhost:8080/api/v1/analyze/אדם | jq -c '.splits[0].readings[] | {stem, lemma, glosses}'
# {"stem":"אדם","lemma":"אדם","glosses":["man","person","human being"]}
# {"stem":"שונות","lemma":"אדם","glosses":["Adam"]}
```

Which lemma a reading means is decided when the data is built and stored per
reading, so nothing at runtime has to work it out — the answer is an array
index. It cannot be a rule, because hspell's own data does not always separate
the words; see [Provenance & correctness](#provenance--correctness).

### Parts of speech

hspell records only `noun`, `verb` and `adjective`, which leaves every
particle, pronoun and preposition in the language blank. The lemma table types
those too, adding `particle`, `preposition`, `pronoun`, `conjunction`,
`adverb` and `interjection` — so `של` reports `preposition`, `את` can report
`pronoun`, and `אוי` reports `interjection`.

Every lemma is typed: 23,340 take their part of speech from hspell and 389
from the two audits described under [Provenance](#provenance--correctness).

| POS | lemmas |     | POS | lemmas |
|---|---:|---|---|---:|
| noun | 14,348 | | adverb | 146 |
| verb | 4,726 | | particle | 83 |
| adjective | 4,266 | | pronoun | 69 |
| preposition | 53 | | interjection | 20 |
| conjunction | 18 | | | |

Glosses are machine-generated and machine-verified, not lexicographer-written
— see [Provenance & correctness](#provenance--correctness). They are meant as a
reading aid, not a dictionary of record.

## Reading order

A word is usually ambiguous, and hspell orders its readings by criteria of its
own that say nothing about usage: it offers `אביהו` as the name *Avihu* before
*his father*, and `טלפוניה` as *telephony* before *her telephones*. Readers want
the opposite.

So every reading carries a `rank` — its lemma's position when the lemmas are
ordered by how often they occur in a corpus of Modern Hebrew, 1 being the most
frequent — and readings come back in that order, likeliest first:

```sh
curl -s http://localhost:8080/api/v1/analyze/אביהו | jq -c '.splits[0].readings[] | {stem, lemma, rank, glosses}'
# {"stem":"אב","lemma":"אב","rank":913,"glosses":["father"]}         "his father"
# {"stem":"שונות","lemma":"אביהו","rank":19648,"glosses":["Avihu"]}  the name
```

Frequency is counted **per lemma**, so this is a suggested ordering, not a
claim about the occurrence in front of you. It separates readings that resolve
to different lemmas, which is most of the win. It says nothing about two
readings of one lemma — `מלך` as a verb and as a noun tie, and keep hspell's
own order between them — and nothing about how an inflected form is usually
read: `מלכה` is *queen* more often than *her king*, but `מלך` is the commoner
lemma and so leads.

The table ranks every one of the dictionary's lemmas, and a
`TestFrequencyCoverage` guard fails the build if a regeneration drops below
99%. A lemma it missed would rank 0, which sorts last rather than first.

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
- **Embedded data, two files.** `data/dictionary.json.gz` (~2.5 MB) is hspell's
  words and morphology; `data/lemmas.json.gz` (~740 KB) is everything they
  *mean*. Both are embedded via `go:embed`, so the binary is self-contained.
  The split is the same one the code makes: morphology is hspell's and is
  transcribed, meaning is this project's and is curated.
- **Plain gzipped JSON, no hand-rolled codec.** The dictionary was once packed
  into hspell's own uvarint encoding. Gzip closes all but 10% of the gap, which
  did not pay for an encoder and two decoders to keep in sync across the build
  — and `zcat | jq` reads what actually shipped. The cost is one-time decode at
  `New()`.
- **Lemmas resolved at build time.** Which lemma a reading means is worked out
  by `internal/gen/lemmas` and stored per reading, so the runtime does no
  resolution at all: it reads the answer out of an array. That has to be a
  stored table rather than a rule, because hspell's data does not always
  separate words that share a stem — see Provenance below.
- **One concept, not three.** Glosses, frequency rank and part of speech all
  hang off the lemma, so there is one table to key, one id space, and nothing
  to keep in sync between them.

Source files:

| File               | Responsibility                                         |
|--------------------|--------------------------------------------------------|
| `analyzer.go`      | public API (`Analyzer`, `Analyze`), prefix trie        |
| `splits.go`        | prefix/base splitting + validity (waw-doubling rule)   |
| `features.go`      | dmask → `Features`, native Hebrew description, specifier|
| `gimatria.go`      | canonical Hebrew-numeral recognition                   |
| `data.go`          | embedded dictionary decode + lookup                    |
| `lemmas.go`        | embedded sense table decode + per-reading lemma lookup |
| `prefixes_data.go` | generated legal-prefix table                           |
| `api/openapi.yaml` | HTTP API spec; `api/api.gen.go` is generated from it   |
| `api/server.go`    | the handler behind the generated routing               |
| `web/`             | static reader portal (vanilla JS + localStorage)       |

## Regenerating the embedded data

Both files are built by `go generate`, in order — the sense table joins on
Hebrew text, so the dictionary has to exist first:

```sh
go generate ./...
# == go run ./internal/gen -src internal/gen/source \
#      -add internal/gen/additions.json -out data/dictionary.json.gz
# && go run ./internal/gen/lemmas \
#      -dict data/dictionary.json.gz -out data/lemmas.json.gz
```

The dictionary is hspell's original data transcribed to UTF-8, plus the
spellings in `internal/gen/additions.json`. The sense table joins four curated
sources — per-stem glosses, glosses for the words hspell gives no stem, the
audit of its function-word blocks, and hand-written lemmas — and ranks
everything against a Modern Hebrew corpus word count. All sources live under
`internal/gen/` for provenance, and both steps report coverage.

Inputs to `internal/gen/lemmas`:

| Source                                     | What it contributes                    |
|--------------------------------------------|----------------------------------------|
| `translate/translations.verified.jsonl`    | English per stem and part of speech    |
| `translate/misc.verified.jsonl`            | English for words with no stem         |
| `translate/milot_audited.jsonl`            | hspell's function-word blocks, split into the real words they contain |
| `curated_lemmas.json`                      | hand-written lemmas (the pronouns)     |
| `additions.json`                           | which lemma each added spelling means  |
| `translate/he_freq_raw.txt`                | corpus word counts, for `rank`         |

## Optional: SQLite export

For deploying the dictionary as a database, an exporter builds a modern SQLite
file from the same two files (the `hebmorph` package itself has no SQLite
dependency):

```sh
go run ./internal/gen/sqlite | sqlite3 dist/hebrew.db
```

The result uses `STRICT` tables, `WITHOUT ROWID` where the natural key is the
whole row, foreign keys, `user_version` (4 — 3 kept glosses and frequencies in
separate tables, 2 had no frequencies, 1 was the dictionary alone), and two
convenience views:

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

CREATE TABLE lemmas (                          -- one row per distinct word;
  id      INTEGER PRIMARY KEY,                 -- word_id is NOT unique, since
  word_id INTEGER NOT NULL REFERENCES words(id), -- unrelated words share a spelling
  pos     TEXT    NOT NULL,
  rank    INTEGER NOT NULL                     -- 1 is most frequent; 0 unranked
) STRICT;

CREATE TABLE glosses (
  lemma_id INTEGER NOT NULL REFERENCES lemmas(id),
  seq      INTEGER NOT NULL,                   -- sense order, most representative first
  english  TEXT    NOT NULL,
  PRIMARY KEY (lemma_id, seq)
) STRICT, WITHOUT ROWID;

CREATE TABLE reading_lemmas (                  -- what each reading means, resolved
  word_id  INTEGER NOT NULL,                   -- when the data was built: no rule
  seq      INTEGER NOT NULL,                   -- to reapply, no catch-all case
  lemma_id INTEGER NOT NULL REFERENCES lemmas(id),
  PRIMARY KEY (word_id, seq),
  FOREIGN KEY (word_id, seq) REFERENCES readings(word_id, seq)
) STRICT, WITHOUT ROWID;

CREATE VIEW readings_view AS                   -- readings with resolved text, plus
  SELECT r.word_id, w.word AS word, r.seq,     -- the lemma each one means
         r.stem_id, s.word AS stem, r.dmask,
         rl.lemma_id, lw.word AS lemma, l.pos, l.rank
  FROM readings r JOIN words w ON w.id = r.word_id JOIN words s ON s.id = r.stem_id
       LEFT JOIN reading_lemmas rl ON rl.word_id = r.word_id AND rl.seq = r.seq
       LEFT JOIN lemmas l ON l.id = rl.lemma_id
       LEFT JOIN words lw ON lw.id = l.word_id;

CREATE VIEW glosses_view AS                    -- glosses with resolved lemma text
  SELECT g.lemma_id, lw.word AS lemma, l.pos, l.rank, g.seq, g.english
  FROM glosses g JOIN lemmas l ON l.id = g.lemma_id
       JOIN words lw ON lw.id = l.word_id;
```

```sh
sqlite3 dist/hebrew.db "SELECT word, stem, dmask FROM readings_view WHERE word='מלכה';"

# a reading and its English. readings_view already carries the lemma, so the
# join is the same match the Go analyzer makes and repeats no rule.
sqlite3 -header -column dist/hebrew.db "
  SELECT r.word, r.stem, r.lemma, r.pos, g.english
    FROM readings_view r JOIN glosses g ON g.lemma_id = r.lemma_id
    WHERE r.word = 'את' ORDER BY r.seq, g.seq;"
# word  stem   lemma  pos       english
# את    את     את     particle  direct object marker
# את    שונות  את     pronoun   you (feminine singular)
# את    את     את     noun      plowshare          -- twice: the plain noun
# את    את     את     noun      mattock            -- reading and the construct

# readings in the order the Go analyzer returns them. rank 0 means unranked,
# which sorts last there, so order by "rank = 0, rank".
sqlite3 -header -column dist/hebrew.db "
  SELECT word, stem, lemma, rank FROM readings_view
    WHERE word = 'אביהו' ORDER BY rank = 0, rank;"
# word   stem   lemma  rank
# אביהו  אב     אב     913
# אביהו  שונות  אביהו  19648
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
are those two reviewed sets.

### Where hspell's data needed correcting

hspell is a **spellchecker**. It only ever had to answer "is this a word?", so
it never needed to tell senses apart, and its morphological output is an
optional feature (`--enable-linginfo`) built from data shaped for spelling. Two
consequences matter here, and both are corrected by audits kept alongside the
glosses.

**Stems are positional in the hand-written files.** `binarize-desc.pl` assigns
the first word of a block as the stem of every word in it. For the
machine-generated noun and verb lists a block really is one word's inflections.
For the hand-maintained function words a block is an editorial grouping, and
one of them merges two unrelated words: `את` the object marker (`אותי`, `אותו`)
and `את` the preposition "with" (`אתי`, `אתו`). All 17 forms would share one
lemma, and so one gloss. `milot_audited.jsonl` is an audit of all 40 blocks —
39 are genuine single words; that one is not.

Its catch-all `שונות` is the same accident: it is simply the first line of
`extrawords.hif`, inherited by the 2,000 lines below it. It reads as "this word
has no stem", which is what the lemma table turns it back into.

**Nothing outside noun/verb/adjective is typed.** `untyped_audited.jsonl` types
the 521 function words that arrived with no part of speech, `לא` and `אני`
among them.

**A few words are missing or spelled against usage.** hspell's own
subject-pronoun list (`extrawords.hif:530`) has eight of the ten pronouns and
omits `אתם`/`אתן`; `curated_lemmas.json` supplies them. And hspell rejects the
ktiv male spelling of "with" on prescriptive grounds — its `spellinghints` says
to write `אתו`, not `איתו` — where ordinary Modern Hebrew does the opposite, so
`additions.json` adds those 18 spellings.

These corrections are deliberate divergences from hspell, confined to the sense
layer and to `additions.json`; the morphology is still transcribed unchanged.

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
