package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
)

// startServer serves ./top10 at /top10/ and exposes /search?q=...
func startServer(idx Index) {
	// add field for searching directly on website
	// Serve local corpus at http://127.0.0.1:8080/top10/
	http.Handle("/top10/", http.StripPrefix("/top10/", http.FileServer(http.Dir("./top10"))))

	// search endpoint that returns TF-IDF hits as JSON
	http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		hits := idx.TFIDF(q)

		fmt.Fprintln(w, "search term:", q)
		_ = json.NewEncoder(w).Encode(hits)
	})

	go func() {
		addr := ":8080"
		log.Println("Listening on", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Fatal(err)
		}
	}()

	// add clickable links to /top10/ in the search results
	//display search results in the browser, write to local host - make URL instead(href)
}

func main() {
	indexOpt := flag.String("index", "inmem", "index backend: inmem | sqlite")               // default to in-memory index
	dbPath := flag.String("db", "project04.db", "sqlite database file (when -index=sqlite)") // defines a -db flag for the SQLite file path (used only when -index=sqlite)
	resetDB := flag.Bool("reset", false, "drop & recreate sqlite tables on startup")
	seed := flag.String("seed", "http://127.0.0.1:8080/top10/", "seed URL to crawl") // choose the starting URL for the crawler, default to local ./top10
	flag.Parse()                                                                     // parse the command-line flags and populates the pointers above

	idx, err := NewIndex(*indexOpt, *dbPath, *resetDB)
	if err != nil {
		log.Fatal(err)
	}

	// start the web server
	startServer(idx)

	tracked := make(map[string]map[string]int)
	time.Sleep(150 * time.Millisecond) // wait for server to start

	// Start the crawler in its own goroutine,
	log.Println("Starting crawler on", *seed)
	crawl(*seed, tracked, idx) // crawl the corpus starting from the seed URL
	log.Println("Crawling complete")

	// // keep process alive
	// select {}

	//!! use only sqlite instead
	// crawl USF website

}
