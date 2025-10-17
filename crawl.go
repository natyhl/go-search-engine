package main

import (
	"net/url"
	"strings"

	"github.com/kljensen/snowball"
)

// CRAWL: given a list of URLs, download each page, extract words and hrefs, clean the hrefs, return the cleaned hrefs
func crawl(seedUrl string, freqmap map[string]map[string]int, idx Index) map[string]map[string]int {

	// make channels of size 10000
	DIn := make(chan , 10000)   // download input
	DOut := make(chan, 10000)
	EOut := make(chan , 10000) // extract output
	CIn := make(chan struct { Base string; Href string }, 10000)


	// Use 
	for {
		select{
			case  DIn <- url:
				go download(url, DOut)
			case  DOut <- :
				go extract(DOut, EOut)
			case  EOut 
				go clean // when you get new URL from clean, pass it to DIn
			default:
				// len of all channels == 0
		}
	}

	visited := make(map[string]bool)
	queue := []string{seedUrl} // queue of URLs to visit

	stopwords, _ := LoadStopwords("stopwords-en.json")
	stopSet := StopwordSet(stopwords)

	for len(queue) > 0 {
		currentURL := queue[0]
		queue = queue[1:] // dequeue

		if visited[currentURL] {
			continue
		}
		visited[currentURL] = true

		// Check robots.txt
		u, err := url.Parse(currentURL)
		if err != nil {
			continue
		}
		rules := rulesForHost(u)

		if isDisallowed(rules, u.Path) {
			continue // disallowed by robots.txt
		}

		// Enforce crawl delay
		waitForDelay(rules)

		// Now we can download:
		// Download
		body, err := download(currentURL)
		if err != nil {
			continue
		}

		// Extract
		words, hrefs := extract(body)

		cleaned := []string{}
		for _, h := range hrefs {
			c := clean(currentURL, h)
			if c != "" {
				cleaned = append(cleaned, c)
			}
		}

		// new slice for Index.AddDocument, collect kept tokens for this page
		var kept []string // will only be used if idx != nil
		if idx != nil {
			kept = make([]string, 0, len(words))
		}

		for _, w := range words {
			// Is word in stop words?
			wl := strings.ToLower(w)
			if _, isStop := stopSet[wl]; isStop {
				continue // skip stopwords
			}

			//stemm word
			stemmed, _ := snowball.Stem(wl, "english", true)
			if stemmed == "" {
				continue
			}

			if freqmap != nil {
				// add to map
				_, ok := freqmap[stemmed]
				if !ok {
					freqmap[stemmed] = make(map[string]int)
					freqmap[stemmed][currentURL] = 1 // initialize inner map if it doesn't exist
				} else {
					freqmap[stemmed][currentURL]++ // increment count
				}
			}

			if idx != nil {
				kept = append(kept, stemmed)
			}
		}

		// Update Index once per page (increments DocCount once, maintains Doc & Tracked)
		if idx != nil && len(kept) > 0 {
			idx.AddDocument(currentURL, kept)
		}

		// Add new URLs to the queue
		for _, s := range cleaned {
			if !visited[s] {
				queue = append(queue, s)
			}
		}
	}
	return freqmap
}
