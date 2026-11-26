package memory

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
)

const recallRuneLimit = 8192

// tokens supports ordinary words and overlapping Han bigrams without an
// external model. It is lexical matching, not embedding-based semantics.
func tokens(s string) map[string]bool {
	out := map[string]bool{}
	var word []rune
	var prev rune
	flush := func() {
		if len(word) > 0 {
			out[string(word)] = true
			word = nil
		}
	}
	count := 0
	for _, r := range strings.ToLower(s) {
		if count >= recallRuneLimit {
			break
		}
		count++
		if unicode.Is(unicode.Han, r) {
			flush()
			out[string(r)] = true
			if prev != 0 {
				out[string([]rune{prev, r})] = true
			}
			prev = r
			continue
		}
		prev = 0
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			word = append(word, r)
		} else {
			flush()
		}
	}
	flush()
	return out
}
func overlap(a, b map[string]bool) int {
	n := 0
	for k := range a {
		if b[k] {
			n++
		}
	}
	return n
}
func (s *Service) Recall(id string, global bool, query string, limit, maxBytes int) (SearchResult, error) {
	if len(query) > 4096 || len(tokens(query)) == 0 {
		return SearchResult{}, fmt.Errorf("nonempty bounded recall query required")
	}
	if limit == 0 {
		limit = 20
	}
	if maxBytes == 0 {
		maxBytes = 16384
	}
	if limit < 1 || limit > 100 || maxBytes < 256 || maxBytes > 65536 {
		return SearchResult{}, fmt.Errorf("recall bounds exceeded")
	}
	items, err := s.list(id, global)
	if err != nil {
		return SearchResult{}, err
	}
	q := tokens(query)
	type ranked struct {
		e     Entry
		score float64
	}
	candidates := []ranked{}
	for _, e := range items {
		key := e.Key
		importance := 0
		if e.Metadata != nil {
			key += " " + e.Metadata.Category + " " + strings.Join(e.Metadata.Tags, " ")
			importance = e.Metadata.Importance
		}
		a, b := tokens(key), tokens(e.Value)
		keyHits, valueHits := overlap(q, a), overlap(q, b)
		if keyHits+valueHits == 0 {
			continue
		}
		covered := 0
		for k := range q {
			if a[k] || b[k] {
				covered++
			}
		}
		score := float64(covered)*4 + float64(keyHits)*2 + float64(valueHits) + float64(importance)*0.1
		candidates = append(candidates, ranked{e, score})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].e.Key < candidates[j].e.Key
	})
	out := SearchResult{Entries: []Entry{}, Total: len(candidates), Omitted: len(candidates)}
	for _, c := range candidates {
		if len(out.Entries) >= limit {
			break
		}
		out.Entries = append(out.Entries, c.e)
		out.Omitted--
		b, _ := json.Marshal(out)
		if len(b) > maxBytes {
			out.Entries = out.Entries[:len(out.Entries)-1]
			out.Omitted++
		}
	}
	return out, nil
}

type SimilarPair struct {
	FirstKey  string  `json:"first_key"`
	SecondKey string  `json:"second_key"`
	Score     float64 `json:"score"`
}
type SimilarResult struct {
	Pairs          []SimilarPair `json:"pairs"`
	ScannedEntries int           `json:"scanned_entries"`
	TotalEntries   int           `json:"total_entries"`
	TotalPairs     int           `json:"total_pairs_in_scan"`
	Truncated      bool          `json:"truncated"`
}

func (s *Service) Similar(id string, global bool, threshold float64, limit int) (SimilarResult, error) {
	if threshold == 0 {
		threshold = .75
	}
	if limit == 0 {
		limit = 30
	}
	if threshold < .5 || threshold > 1 || math.IsNaN(threshold) || limit < 1 || limit > 100 {
		return SimilarResult{}, fmt.Errorf("similarity bounds exceeded")
	}
	items, err := s.list(id, global)
	if err != nil {
		return SimilarResult{}, err
	}
	out := SimilarResult{Pairs: []SimilarPair{}, TotalEntries: len(items)}
	// Bounded pairwise work; callers can use ordinary recall to inspect more.
	if len(items) > 200 {
		items = items[:200]
	}
	out.ScannedEntries = len(items)
	out.Truncated = out.ScannedEntries < out.TotalEntries
	features := make([]map[string]bool, len(items))
	for i, e := range items {
		features[i] = tokens(e.Value)
	}
	for i := range items {
		if len(features[i]) < 3 {
			continue
		}
		for j := i + 1; j < len(items); j++ {
			if items[i].Value == items[j].Value {
				continue
			}
			if len(features[j]) < 3 {
				continue
			}
			common := overlap(features[i], features[j])
			score := float64(common) / float64(len(features[i])+len(features[j])-common)
			if score < threshold {
				continue
			}
			out.TotalPairs++
			out.Pairs = append(out.Pairs, SimilarPair{items[i].Key, items[j].Key, score})
		}
	}
	sort.Slice(out.Pairs, func(i, j int) bool {
		a, b := out.Pairs[i], out.Pairs[j]
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		if a.FirstKey != b.FirstKey {
			return a.FirstKey < b.FirstKey
		}
		return a.SecondKey < b.SecondKey
	})
	if len(out.Pairs) > limit {
		out.Pairs = out.Pairs[:limit]
		out.Truncated = true
	}
	for len(out.Pairs) > 0 {
		b, _ := json.Marshal(out)
		if len(b) <= 65536 {
			break
		}
		out.Pairs = out.Pairs[:len(out.Pairs)-1]
		out.Truncated = true
	}
	return out, nil
}
