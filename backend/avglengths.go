package main

// avgWordLengths holds precomputed average grapheme length per language,
// computed once from the full kaikki.org dumps so the app never has to
// download and scan a whole dictionary just to suggest a default word length.
var avgWordLengths = map[string]float64{}
