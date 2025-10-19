package main

import (
	"net/url"
	"strings"

	"github.com/kljensen/snowball"
)

// CRAWL: given a list of URLs, download each page, extract words and hrefs, clean the hrefs, return the cleaned hrefs
func crawl(seedUrl string, freqmap map[string]map[string]int, idx Index) map[string]map[string]int {

	// make channels of size 10000
	DIn := make(chan string, 10000)          // download input
	DOut := make(chan DownloadResult, 10000) // download output
	EOut := make(chan ExtractResult, 10000)  // extract output
	CIn := make(chan CleanInput, 10000)
	COut := make(chan string, 10000) // clean output - going to pass to DIn

	numWorkers := 3
	for i := 0; i < numWorkers; i++ { // start 3 workers of each type
		go download(DIn, DOut)
		go extract(DOut, EOut)
		go clean(CIn, COut)
		go indexer(EOut, idx) //**describe why
	}

	visited := make(map[string]bool)
	queue := []string{seedUrl} // queue of URLs to visit

	stopwords, _ := LoadStopwords("stopwords-en.json")
	stopSet := StopwordSet(stopwords)

	pendingDownloads := 0 // helpers to stop the for loop when all done
	pendingCleans := 0

	for len(queue) > 0 || pendingDownloads > 0 || pendingCleans > 0 { // download and cleaning in progress
		select { // listens to multiple channels simultaneously and executes whichever case is ready
		case cleanedURL := <-COut:
			pendingCleans--
			if !visited[cleanedURL] {
				queue = append(queue, cleanedURL)
			}
		case result := <-EOut:
			pendingDownloads-- // download->extract cycle complete
			// process result
			currentURL := result.URL

			for _, h := range result.Hrefs {
				CIn <- CleanInput{Base: currentURL, Href: h}
				pendingCleans++
			}

			// Process words for freqmap only (indexer handles DB)
			if freqmap != nil {
				for _, w := range result.Words {
					wl := strings.ToLower(w)
					if _, isStop := stopSet[wl]; isStop {
						continue // skip stopwords
					}

					stemmed, _ := snowball.Stem(wl, "english", true)
					if stemmed == "" {
						continue
					}

					// add to map
					_, ok := freqmap[stemmed]
					if !ok {
						freqmap[stemmed] = make(map[string]int)
						freqmap[stemmed][currentURL] = 1 // initialize inner map if it doesn't exist
					} else {
						freqmap[stemmed][currentURL]++ // increment count
					}
				}
			}

		default:
			// no channels ready, process next URL in queue
			if len(queue) == 0 {
				continue
			}

			// process queue if not empty
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

			// Send URL to download channel
			DIn <- currentURL
			pendingDownloads++

		}
	}

	//Close channels when done, signal workers to stop
	close(DIn)
	close(CIn)

	for pendingDownloads > 0 || pendingCleans > 0 {
		select {
		case <-COut:
			pendingCleans--
		case <-EOut:
			pendingDownloads--
		}
	}

	return freqmap
}

func indexer(EOut chan ExtractResult, idx Index) { // put cleaned, stemmed words into the index
	stopword, _ := LoadStopwords("stopwords-en.json")
	stopSet := StopwordSet(stopword)

	for result := range EOut { // keep reading until channel is closed
		var kept []string

		for _, w := range result.Words {
			wl := strings.ToLower(w)
			if _, isStop := stopSet[wl]; isStop {
				continue // skip stopwords
			}

			stemmed, _ := snowball.Stem(wl, "english", true)
			if stemmed == "" {
				continue
			}

			kept = append(kept, stemmed)
		}

		if idx != nil && len(kept) > 0 {
			idx.AddDocument(result.URL, kept)
		}
	}
}
