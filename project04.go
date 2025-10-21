package main

import (
	"flag"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// startServer serves ./top10 at /top10/ and exposes /search?q=...
func startServer(idx Index) {
	// Serve local corpus at http://127.0.0.1:8080/top10/
	http.Handle("/top10/", http.StripPrefix("/top10/", http.FileServer(http.Dir("./top10"))))

	// Handle requests to http://localhost:8080/
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { 
		if r.URL.Path == "/" { // If user visits the root path exactly
			// Redirect them to /search
			http.Redirect(w, r, "/search", http.StatusFound) 
			return                                           
		} 
		http.NotFound(w, r) // If user visits any other path, return 404
	}) 

	// search endpoint that returns TF-IDF hits as JSON
	http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		hits := idx.TFIDF(q)

		// template data
		type Hit struct {
			Url   string
			Title string
		}

		var hrefs []Hit
		for _, h := range hits {
			title := pageTitle(h.URL)
			// **If all titles are the same, show URL instead**
			if title == "" {
				title = h.URL
			}
			
			u, _ := url.Parse(h.URL)
			if u != nil && u.Path != "" && u.Path != "/" {
				// Extract just the filename
				parts := strings.Split(u.Path, "/")
				filename := parts[len(parts)-1]
				title = title + " - " + filename
			}
			hrefs = append(hrefs, Hit{Url: h.URL, Title: title})
		}

		tmpl := `
	<html>
	<body>
		<h2>Search</h2>
		<form action="/search" method="get" style="margin-bottom:1rem">
		<input type="text" name="q" placeholder="Enter search term..." value="{{.Query}}">
		<button type="submit">Search</button>
		</form>

		{{if .Query}}
		<h3>Results for "{{.Query}}"</h3>
		{{if .Hits}}
			{{range .Hits}}
			<p><a href="{{.Url}}" target="_blank" rel="noopener">{{.Title}}</a></p>
			{{end}}
		{{else}}
			<p>No results found.</p>
		{{end}}
		{{end}}
	</body>
	</html>`

		//empty HTML template object named "page", parse into template format
		t, err := template.New("page").Parse(tmpl)
		if err != nil {
			log.Fatal(err)
		}

		data := struct {
			Query string
			Hits  []Hit
		}{
			Query: q,
			Hits:  hrefs,
		}

		// Set response header to indicate HTML content with UTF-8 encoding
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		t.Execute(w, data) // Write to response
	})

	// Launch server in background goroutine so main() can continue
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

// helper to get <title> text
func pageTitle(u string) string {
	body, err := downloadHelper(u)
	if err != nil || len(body) == 0 {
		return u
	}
	s := string(body) // convert []byte to string, contains html page
	lo := strings.ToLower(s)
	i := strings.Index(lo, "<title>")  // find <title> start
	j := strings.Index(lo, "</title>") // find </title> end
	if i >= 0 && j > i {
		i += len("<title>")            // move i to after <title>
		t := strings.TrimSpace(s[i:j]) // extract title text
		if t != "" {
			return t
		}
	}
	return u
}

func main() {
	indexOpt := flag.String("index", "sqlite", "index backend: inmem | sqlite")              // default to sqlite
	dbPath := flag.String("db", "project04.db", "sqlite database file (when -index=sqlite)") // defines a -db flag for the SQLite file path (used only when -index=sqlite)
	resetDB := flag.Bool("reset", false, "drop & recreate sqlite tables on startup")
	//seed := flag.String("seed", "http://127.0.0.1:8080/top10/", "seed URL to crawl") // choose the starting URL for the crawler, default to local ./top10
	seed := flag.String("seed", "https://www.usfca.edu/", "seed URL to crawl")
	flag.Parse() // parse the command-line flags and populates the pointers above

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

}
