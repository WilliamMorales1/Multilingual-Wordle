package wordlist

import (
	jsonv1 "encoding/json"
	json "encoding/json/v2"
	"testing"
)

// sampleLine mirrors the shape of one kaikki.org wiktextract JSONL record.
const sampleLine = `{"word":"caf\u00e9","lang":"French","pos":"noun",` +
	`"senses":[{"glosses":["coffee"],"tags":["masculine"]},{"glosses":["cafe"],"form_of":[{"word":"cafe"}]}],` +
	`"etymology_text":"from Turkish kahve","head_templates":[{"args":{"canj":"VND"},"expansion":"Cangjie input \u5973\u5f13\u6728 (VND)"}],` +
	`"sounds":[{"zh_pron":"ka1","tags":["Mandarin"]}],"redirects":["cafe"]}`

// TestEntryDecodeMatchesV1 pins encoding/json/v2 decoding of a dump record to
// the encoding/json behaviour the parser was originally written against.
func TestEntryDecodeMatchesV1(t *testing.T) {
	var v1, v2 KaikkiEntry
	if err := jsonv1.Unmarshal([]byte(sampleLine), &v1); err != nil {
		t.Fatalf("encoding/json: %v", err)
	}
	if err := json.Unmarshal([]byte(sampleLine), &v2, permissiveJSON); err != nil {
		t.Fatalf("encoding/json/v2: %v", err)
	}
	checks := []struct {
		name string
		a, b any
	}{
		{"word", v1.Word, v2.Word},
		{"etymology", v1.Etymology, v2.Etymology},
		{"firstGloss", firstGloss(v1), firstGloss(v2)},
		{"formOfWord", formOfWord(v1), formOfWord(v2)},
		{"cangjieCode", cangjieCodeFromEntry(v1), cangjieCodeFromEntry(v2)},
		{"romanize", romanizeEntry(v1, "Mandarin"), romanizeEntry(v2, "Mandarin")},
		{"excluded", isExcludedEntry(v1), isExcludedEntry(v2)},
	}
	for _, c := range checks {
		if c.a != c.b {
			t.Errorf("%s: v1 = %v, v2 = %v", c.name, c.a, c.b)
		}
	}
}

// TestPermissiveJSONAcceptsDirtyInput guards the options in permissiveJSON:
// v2's defaults reject invalid UTF-8 and duplicate members, which would make
// the parser silently skip records that encoding/json used to accept.
func TestPermissiveJSONAcceptsDirtyInput(t *testing.T) {
	dirty := []byte("{\"word\":\"a\xffb\",\"word\":\"a\xffb\",\"lang\":\"French\"}")
	var entry KaikkiEntry
	if err := json.Unmarshal(dirty, &entry, permissiveJSON); err != nil {
		t.Fatalf("permissiveJSON rejected dirty record: %v", err)
	}
	if entry.Lang != "French" {
		t.Errorf("lang = %q, want %q", entry.Lang, "French")
	}
	if err := json.Unmarshal(dirty, new(KaikkiEntry)); err == nil {
		t.Log("note: v2 defaults now accept this input; permissiveJSON may be unnecessary")
	}
}

func TestPickAutoLength(t *testing.T) {
	cases := []struct {
		name    string
		weights map[int]int
		want    int
	}{
		{"single length", map[int]int{5: 10}, 5},
		{"median, not mean", map[int]int{4: 6, 5: 1, 11: 3}, 4}, // mean 6.1
		{"lands on the heavier side", map[int]int{4: 3, 5: 7}, 5},
		{
			// A long tail can't move the median the way it moves a mean.
			"long outliers are ignored",
			map[int]int{5: 190, 12: 10},
			5,
		},
		{
			// Weight, not word count, decides: the common 4-letter words
			// outvote twice as many ordinary 9-letter ones.
			"common words count more",
			map[int]int{4: 5 * commonWordWeight, 9: 10},
			4,
		},
		{"clamped to minimum", map[int]int{1: 10}, minAutoLength},
		{"clamped to maximum", map[int]int{30: 10}, maxAutoLength},
		{"nothing sampled", map[int]int{}, minAutoLength},
		{"no weight at all", map[int]int{5: 0}, minAutoLength},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pickAutoLength(c.weights); got != c.want {
				t.Errorf("pickAutoLength(%v) = %d, want %d", c.weights, got, c.want)
			}
		})
	}
}

func TestMedianWeight(t *testing.T) {
	sense := func(tags ...string) map[string]any {
		s := map[string]any{}
		if len(tags) > 0 {
			raw := make([]any, len(tags))
			for i, tag := range tags {
				raw[i] = tag
			}
			s["tags"] = raw
		}
		return s
	}
	listed := func(n int) []struct{} { return make([]struct{}, n) }

	cases := []struct {
		name  string
		entry KaikkiEntry
		want  int
	}{
		{"bare entry counts once", KaikkiEntry{Senses: []map[string]any{sense()}}, 1},
		{
			"a widely used word counts for many",
			KaikkiEntry{Senses: []map[string]any{sense()}, Translations: listed(40), Derived: listed(12), Synonyms: listed(3)},
			56,
		},
		{
			"cross-references come from every list",
			KaikkiEntry{Senses: []map[string]any{sense()}, Related: listed(2), Descendants: listed(1), Antonyms: listed(1), Hyponyms: listed(1), Hypernyms: listed(1), CoordinateTerms: listed(1)},
			8,
		},
		{
			"senses multiply the count",
			KaikkiEntry{Senses: []map[string]any{sense(), sense(), sense()}, Synonyms: listed(1)},
			6,
		},
		{
			"common tag multiplies the count",
			KaikkiEntry{Senses: []map[string]any{sense("common")}, Translations: listed(4)},
			5 * commonWordWeight,
		},
		{
			"wholly obsolete word doesn't count at all",
			KaikkiEntry{Senses: []map[string]any{sense("obsolete"), sense("archaic")}, Derived: listed(9)},
			0,
		},
		{
			"one live sense is enough to count",
			KaikkiEntry{Senses: []map[string]any{sense("obsolete"), sense("slang")}, Synonyms: listed(1)},
			4,
		},
		{"no senses", KaikkiEntry{}, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := medianWeight(c.entry); got != c.want {
				t.Errorf("medianWeight(%q) = %d, want %d", c.name, got, c.want)
			}
		})
	}
}

func TestIsInflectedForm(t *testing.T) {
	sense := func(tags ...string) map[string]any {
		s := map[string]any{}
		if len(tags) > 0 {
			raw := make([]any, len(tags))
			for i, tag := range tags {
				raw[i] = tag
			}
			s["tags"] = raw
		}
		return s
	}
	cases := []struct {
		name   string
		senses []map[string]any
		want   bool
	}{
		{"a word of its own", []map[string]any{sense()}, false},
		{"tagged a form of something", []map[string]any{sense("form-of", "past")}, true},
		{"tagged an alternative spelling", []map[string]any{sense("alt-of", "misspelling")}, true},
		{"an untagged form_of reference", []map[string]any{{"form_of": []any{map[string]any{"word": "find"}}}}, true},
		{"one inflected sense among real ones", []map[string]any{sense("past"), sense()}, false},
		{"no senses at all", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isInflectedForm(KaikkiEntry{Senses: c.senses}); got != c.want {
				t.Errorf("isInflectedForm(%v) = %v, want %v", c.senses, got, c.want)
			}
		})
	}
}
