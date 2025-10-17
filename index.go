// index.go implements the Index interface

package main

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/kljensen/snowball"
)

type Posting struct {
	URL    string
	Count  int // term count in this doc
	DocLen int // total tokens in this doc
}

// TFIDFData is what the shared TF-IDF calculator requires
type TFIDFData interface {
	Postings(stem string) []Posting // raw postings for a stem
	TotalDocs() int                 // number of docs with tokens
}

type SearchResult struct {
	URL   string  // document URL
	TF    float64 // term frequency for this
	IDF   float64 // inverse document frequency for the term
	Score float64 // TF-IDF score = TF * IDF
}

type Hits []SearchResult // a slice of results that implements sort.Interface

func (h Hits) Len() int      { return len(h) }
func (h Hits) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h Hits) Less(i, j int) bool {
	if h[i].Score == h[j].Score {
		return h[i].URL > h[j].URL
	}
	return h[i].Score > h[j].Score
}

func (h Hits) Sort() { sort.Sort(h) }

type Index interface {
	AddWord(url, word string)
	AddDocument(url string, words []string)

	Search(term string) Hits
	TFIDF(term string) Hits
}

func NewIndex(option, path string, reset bool) (Index, error) {
	switch option {
	case "inmem":
		return NewInvertedIndex(), nil
	case "sqlite":
		return NewSqlIndex(path, reset)
	default:
		fmt.Println("Unknown index option:", option)
		return nil, nil

	}
}

// TF = termCount(url, term) / totalTermsInDoc(url)
// IDF = ln( N / df(term) ), where N = #docs, df = number of docs containing term
func computeTFIDF(src TFIDFData, query string) Hits {
	term := strings.ToLower(strings.TrimSpace(query)) //matching is case-insensitive
	if term == "" {
		return nil
	}

	stemmed, _ := snowball.Stem(term, "english", true)
	if stemmed == "" {
		return nil
	}

	postings := src.Postings(stemmed)
	if len(postings) == 0 { // If no document contains this term, return no hits
		return nil
	}

	n := src.TotalDocs() // the corpus size (number of documents)
	if n == 0 {
		return nil
	}

	df := len(postings) // document frequency: how many documents contain the term - # docs containing term
	// IDF = log(N / (df + 1))
	idf := math.Log(float64(n) / float64(df+1))

	var hits Hits
	for _, p := range postings { // Iterates over each document URL u that contains the term - cnt is term count in that document
		if p.DocLen <= 0 {
			continue
		}
		tf := float64(p.Count) / float64(p.DocLen) // TF = (term count in doc) / (total tokens - words - in doc)

		score := tf * idf // Compute the TF-IDF score for this document
		hits = append(hits, SearchResult{
			URL:   p.URL,
			TF:    tf,
			IDF:   idf,
			Score: score,
		}) // Add a ScoredResult for this document to the hits slice
	}
	hits.Sort() // Sort the results by score (highest first), breaking ties by URL
	return hits
}
