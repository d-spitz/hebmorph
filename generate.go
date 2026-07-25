package hebmorph

// Regenerate the embedded UTF-8 dictionary from hspell's original ISO-8859-8
// data files (kept under internal/gen/source for provenance and reproducibility).
//
//go:generate go run ./internal/gen -src internal/gen/source -out data/hebrew.dict.gz
