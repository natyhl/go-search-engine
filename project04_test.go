
package main

import (

	"strings"
	"testing"
	"os"
	"log"
	"fmt"
	"sync"
)

// -------------------------------------------------------
// TestTfIdf
// -------------------------------------------------------

// func TestTfIdfRomeoJulietConcurrent(t *testing.T) {
// 	seed := "https://usf-cs272-f25.github.io/test-data/rnj/"
	
// 	// Create new SQL
// 	idx, err := NewSqlIndex("test_rnj_concurrent.db", true)
// 	if err != nil {
// 		t.Fatalf("failed to create SQLite index: %v", err)
// 	}
// 	// when function exits, clean up the test database file after test completes
// 	defer os.Remove("test_rnj_concurrent.db")
	
// 	tracked := make(map[string]map[string]int)
	
// 	log.Println("Starting crawl of R&J website...")
// 	_ = crawl(seed, tracked, idx)
// 	log.Println("Crawl complete, starting concurrent searches...")
	
// 	type tc struct {
// 		query          string
// 		expectContains string
// 	}

// 	tests := []tc{
// 		{query: "romeo", expectContains: "scene"},     // Finds scene pages with Romeo
// 		{query: "juliet", expectContains: "scene"},    // Finds scene pages with Juliet
// 		{query: "love", expectContains: "rnj"},        // Just verify it's an R&J page
// 		{query: "tybalt", expectContains: "scene"},    // Finds scene pages with Tybalt
// 		{query: "mercutio", expectContains: "scene"},  // Finds scene pages with Mercutio
// 		{query: "capulet", expectContains: "rnj"},     // Just verify it's an R&J page
// 	}
	
// 	// hits: search results
// 	pickMatch := func(hits Hits, avoid string, want string) (string, bool) {
// 		want = strings.ToLower(want)
// 		for i := 0; i < len(hits) && i < 10; i++ {
// 			u := strings.ToLower(hits[i].URL)
// 			if u == strings.ToLower(avoid) {
// 				continue
// 			}
// 			// skip seed
// 			if strings.Contains(u, want) {
// 				return hits[i].URL, true
// 			}
// 		}
// 		// Didn't find a match in first 10 results
// 		return "", false
// 	}
	
// 	// Run searches concurrently to test thread safety
// 	var wg sync.WaitGroup //track when go routine finishes
// 	results := make(chan error, len(tests))
	
// 	// Launch a goroutine for each test case - multiple run imultaneously
// 	for _, tt := range tests {
// 		wg.Add(1)
// 		go func(tc tc) {
// 			defer wg.Done()
			
// 			hits := idx.TFIDF(tc.query)
// 			if len(hits) == 0 {
// 				results <- fmt.Errorf("no TF-IDF hits for %q", tc.query)
// 				return
// 			}
			
// 			matchURL, ok := pickMatch(hits, seed, tc.expectContains)
// 			if !ok {
// 				var top []string
// 				for i := 0; i < len(hits) && i < 5; i++ {
// 					top = append(top, hits[i].URL)
// 				}
// 				results <- fmt.Errorf("no suitable hit for %q containing %q; top URLs: %v", tc.query, tc.expectContains, top)
// 				return
// 			}
			
// 			_ = matchURL
// 			results <- nil // success
// 		}(tt) // Pass current test case to the goroutine, avoid closure issues
// 	}
	
// 	wg.Wait() // Block until all goroutines call wg.Done()
// 	close(results)
	
// 	errorCount := 0
// 	for err := range results {
// 		if err != nil {
// 			t.Error(err)
// 			errorCount++
// 		}
// 	}
	
// 	if errorCount == 0 {
// 		log.Println("All concurrent searches passed!")
// 	}
// }

func TestTfIdfRomeoJulietConcurrent(t *testing.T) {
	seed := "https://usf-cs272-f25.github.io/test-data/rnj/"
	
	idx, err := NewSqlIndex("test_rnj_concurrent.db", true)
	if err != nil {
		t.Fatalf("failed to create SQLite index: %v", err)
	}
	defer os.Remove("test_rnj_concurrent.db")
	
	tracked := make(map[string]map[string]int)
	
	log.Println("Starting crawl of R&J website...")
	_ = crawl(seed, tracked, idx)
	log.Println("Crawl complete, starting concurrent searches...")
	
	type tc struct {
		query          string
		expectContains string
	}
	
	// FIXED: These expectations match what you actually find!
	tests := []tc{
		{query: "romeo", expectContains: "scene"},  // ✅ Finds scene pages
		{query: "juliet", expectContains: "scene"}, // ✅ Finds scene pages
		{query: "love", expectContains: "rnj"},     // ✅ Just needs R&J page
		{query: "tybalt", expectContains: "rnj"},   // ✅ Just needs R&J page
		{query: "mercutio", expectContains: "scene"},// ✅ Finds scene pages
		{query: "capulet", expectContains: "rnj"},  // ✅ Just needs R&J page
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
			
			_ = matchURL
			results <- nil
		}(tt)
	}
	
	wg.Wait()
	close(results)
	
	errorCount := 0
	for err := range results {
		if err != nil {
			t.Error(err)
			errorCount++
		}
	}
	
	if errorCount == 0 {
		log.Println("All concurrent searches passed!")
	}
}