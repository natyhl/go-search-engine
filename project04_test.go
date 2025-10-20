
// -------------------------------------------------------
// TestTfIdf (concurrent crawl, same style as your example)
// -------------------------------------------------------

package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"fmt"
	"os"
)

// -------------------------------------------------------
// TestTfIdfConcurrent
// -------------------------------------------------------
func TestTfIdfConcurrent(t *testing.T) {
	// Serve local ./top10 under /top10/
	ts := httptest.NewServer(http.StripPrefix("/top10/", http.FileServer(http.Dir("./top10"))))
	defer ts.Close()

	seed := ts.URL + "/top10/"

	// Use SQLite index for concurrent test
	idx, err := NewSqlIndex("test_concurrent.db", true)
	if err != nil {
		t.Fatalf("failed to create SQLite index: %v", err)
	}
	defer os.Remove("test_concurrent.db")

	tracked := make(map[string]map[string]int)

	// Build index using concurrent crawler
	_ = crawl(seed, tracked, idx)

	type tc struct {
		query          string
		expectContains string
	}
	tests := []tc{
		{query: "dracula", expectContains: "dracula"},
		{query: "romeo", expectContains: "romeo"},
		{query: "juliet", expectContains: "romeo%20and%20juliet"},
	}

	pickMatch := func(hits Hits, avoid string, want string) (string, bool) {
		want = strings.ToLower(want)
		for i := 0; i < len(hits) && i < 10; i++ {
			u := strings.ToLower(hits[i].URL)
			if u == strings.ToLower(avoid) {
				continue
			}
			if strings.Contains(u, want) {
				return hits[i].URL, true
			}
		}
		return "", false
	}

	// Run searches concurrently to test thread safety
	var wg sync.WaitGroup
	results := make(chan error, len(tests))

	for _, tt := range tests {
		wg.Add(1)
		go func(tc tc) {
			defer wg.Done()

			hits := idx.TFIDF(tc.query)
			if len(hits) == 0 {
				results <- fmt.Errorf("no TF-IDF hits for %q", tc.query)
				return
			}

			matchURL, ok := pickMatch(hits, seed, tc.expectContains)
			if !ok {
				var top []string
				for i := 0; i < len(hits) && i < 5; i++ {
					top = append(top, hits[i].URL)
				}
				results <- fmt.Errorf("no suitable hit for %q containing %q; top URLs: %v", tc.query, tc.expectContains, top)
				return
			}

			if hits[0].Score < 0 {
				results <- fmt.Errorf("unexpected negative TF-IDF score for top hit %q: %f", hits[0].URL, hits[0].Score)
				return
			}

			_ = matchURL
			results <- nil // success
		}(tt)
	}

	// Wait for all goroutines
	wg.Wait()
	close(results)

	// Check results
	for err := range results {
		if err != nil {
			t.Error(err)
		}
	}
}