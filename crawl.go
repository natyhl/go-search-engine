package main

import (
	"net/url"
	"strings"
	"log"
	"time"

	"github.com/kljensen/snowball"
)

// CRAWL: given a list of URLs, download each page, extract words and hrefs, clean the hrefs, return the cleaned hrefs
// starts from seedURL
// returns: the populated freqmap
func crawl(seedUrl string, freqmap map[string]map[string]int, idx Index) map[string]map[string]int {

	// make channels of size 10000
	DIn := make(chan string, 10000)          // download input
	DOut := make(chan DownloadResult, 10000) // download output
	EOut := make(chan ExtractResult, 10000)  // extract output
	CIn := make(chan CleanInput, 10000)
	COut := make(chan string, 10000) // clean output - going to pass to DIn

	// start workers of each type
	downloadWorkers := 20  
	extractWorkers := 10   
	cleanWorkers := 30     
	
	for i := 0; i < downloadWorkers; i++ {
		go download(DIn, DOut)
	}
	
	for i := 0; i < extractWorkers; i++ {
		go extract(DOut, EOut)
	}
	
	for i := 0; i < cleanWorkers; i++ {
		go clean(CIn, COut)
	}

	visited := make(map[string]bool) // we already crawled
	queue := []string{seedUrl} // queue of URLs to visit

	stopwords, _ := LoadStopwords("stopwords-en.json")
	stopSet := StopwordSet(stopwords)

	pendingDownloads := 0 // helpers to stop the for loop when all done
	pendingCleans := 0
	maxQueueSize := 10000  // Limit queue growth

	// timeout mechanism - detect when crawler is stuck
	lastProgress := time.Now()
	progressTicker := time.NewTicker(5 * time.Second) // Log progress every 5 seconds
	defer progressTicker.Stop() // Clean up timer when function exits

	// CRAWL LOOP //
	for len(queue) > 0 || pendingDownloads > 0 || pendingCleans > 0 { // download and cleaning in progress

		// ADDED//
		if len(visited) >= 5000 {
			log.Printf("Reached 5000 pages. Stopping crawl.")
			goto drain
		}

		select { // listens to multiple channels simultaneously and executes whichever case is ready
			
		case <-progressTicker.C:
			// Every 5 seconds, log current state
			log.Printf("Progress: queue=%d, visited=%d, pendingDownloads=%d, pendingCleans=%d", 
				len(queue), len(visited), pendingDownloads, pendingCleans)
			
			// If stuck for more than 30 seconds, break
			if time.Since(lastProgress) > 30*time.Second {
				log.Printf("WARNING: No progress for 30s. Breaking out. pendingDownloads=%d, pendingCleans=%d",
					pendingDownloads, pendingCleans)
				goto drain // Jump to drain section to clean up
			} 

		case cleanedURL := <-COut:
			pendingCleans--
			lastProgress = time.Now() // Update progress timestamp
			if cleanedURL != "" && !visited[cleanedURL] && len(queue) < maxQueueSize { 
				queue = append(queue, cleanedURL)
			}
		case result := <-EOut:
			pendingDownloads-- // download->extract cycle complete
			// process result
			currentURL := result.URL

			// Index the document
			if idx != nil && len(result.Words) > 0 {
				var kept []string
				for _, w := range result.Words {
					wl := strings.ToLower(w)
					if _, isStop := stopSet[wl]; isStop {
						continue
					}
					stemmed, _ := snowball.Stem(wl, "english", true)
					if stemmed == "" {
						continue
					}
					kept = append(kept, stemmed)
				}
				if len(kept) > 0 {
					idx.AddDocument(currentURL, kept)
				}
			}

			// Send links to clean()
			for _, h := range result.Hrefs {
				CIn <- CleanInput{Base: currentURL, Href: h}
				pendingCleans++
			}

			// Process words for freqmap 
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

	drain:
	//Close channels when done, signal workers to stop
	close(DIn)
	close(CIn)

	log.Printf("Main loop done. Draining remaining results. pendingDownloads=%d, pendingCleans=%d", 
		pendingDownloads, pendingCleans)

	// DRAIN REMAINING DATA //
	drainStart := time.Now() 
	drainTimeout := 30 * time.Second // Max time to wait

	// Keep receiving until all pending work is done
	for pendingDownloads > 0 || pendingCleans > 0 {
		if time.Since(drainStart) > drainTimeout {
			log.Printf("WARNING: Drain timeout after %v. Forcing exit. pendingDownloads=%d, pendingCleans=%d",
				drainTimeout, pendingDownloads, pendingCleans)  
			// Force counters to zero to exit loop
			pendingDownloads = 0  
			pendingCleans = 0     
			break
		} 
		select {

		case <-COut:  
			pendingCleans--

		// Receive any remaining extracted results
		case result := <-EOut:
			pendingDownloads--
			log.Printf("Drained extract. pendingDownloads=%d", pendingDownloads)

			// **Still index during draining** //
			if idx != nil && len(result.Words) > 0 {
				var kept []string
				for _, w := range result.Words {
					wl := strings.ToLower(w)
					if _, isStop := stopSet[wl]; isStop {
						continue
					}
					stemmed, _ := snowball.Stem(wl, "english", true)
					if stemmed == "" {
						continue
					}
					kept = append(kept, stemmed)
				}
				if len(kept) > 0 {
					idx.AddDocument(result.URL, kept)
				}
			}
		// If nothing arrives for 100ms
		case <-time.After(100 * time.Millisecond):
			// Stuck for 2 sec --> force exit
			if time.Since(drainStart) > 2*time.Second {
				log.Printf("Forcing drain completion after 2s of timeouts. Zeroing counters.")
				pendingDownloads = 0
				pendingCleans = 0
			}	
		}
	}

	log.Printf("Crawl complete! Visited %d URLs", len(visited))
	return freqmap
}

