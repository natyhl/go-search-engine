package main

import (
	"strings"

	"github.com/kljensen/snowball"
)

type InvIndex struct {
	Tracked  map[string]map[string]int // stemmed word, url - count
	DocLen   map[string]int            // for each url, the total number of terms in that document
	DocCount int                       // how many documents are indexed
}

// constructor for Index: Creates and returns a pointer to a new, empty Index,
func NewInvertedIndex() *InvIndex {
	return &InvIndex{
		Tracked:  make(map[string]map[string]int),
		DocLen:   make(map[string]int),
		DocCount: 0,
	}
}

// add a single word occurrence for a specific url
func (idx *InvIndex) AddWord(url, word string) {
	w := strings.ToLower(strings.TrimSpace(word))
	if w == "" {
		return // skip empty words
	}

	stemmed, _ := snowball.Stem(w, "english", true)
	if stemmed == "" {
		return
	}

	_, ok := idx.Tracked[stemmed] // Ensures there’s a map for the term in Tracked
	if !ok {
		idx.Tracked[stemmed] = make(map[string]int)
	}
	idx.Tracked[stemmed][url]++ // Increment the count for that term in that document
	idx.DocLen[url]++           // Increments to compute TF
}

// add an entire document’s words at once
func (idx *InvIndex) AddDocument(url string, words []string) {
	// Only count the document once
	_, seen := idx.DocLen[url]
	if !seen { // If this URL hasn’t been seen before, increment DocCount
		idx.DocCount++
	}
	for _, w := range words {
		idx.AddWord(url, w)
	}
}

func (idx *InvIndex) Search(word string) Hits {
	stemmed, err := snowball.Stem(strings.ToLower(word), "english", true)
	if err == nil {
		postings := idx.Tracked[stemmed] //Look up the postings list for this term
		if postings == nil {
			return nil
		}
		var hits Hits

		for url, cnt := range postings { //Iterate over each document containing the term: url is the doc
			if dl := idx.DocLen[url]; dl > 0 {
				tf := float64(cnt) / float64(dl)
				hits = append(hits, SearchResult{URL: url, TF: tf, Score: tf})

			}
		}
		hits.Sort()
		return hits
	}
	return nil
}

// Postings returns raw counts + doc lengths for a stem.
func (idx *InvIndex) Postings(stem string) []Posting {
	m := idx.Tracked[stem]
	if m == nil {
		return nil
	}
	out := make([]Posting, 0, len(m))
	for url, cnt := range m {
		out = append(out, Posting{URL: url, Count: cnt, DocLen: idx.DocLen[url]})
	}
	return out
}

// TotalDocs returns the number of indexed docs with tokens.
func (idx *InvIndex) TotalDocs() int {
	if idx.DocCount > 0 {
		return idx.DocCount
	}
	return len(idx.DocLen)
}

// Now the TFIDF method just delegates to the shared function.
func (idx *InvIndex) TFIDF(term string) Hits {
	return computeTFIDF(idx, term)
}
