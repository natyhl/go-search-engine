package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

// !! sqlite for test cases

package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/kljensen/snowball"
)

type TestCase struct {
	name    string
	mockURL string
	body    []byte
	err     error
}

// -------------------------------------------------------
// TestDownload
// -------------------------------------------------------
func TestDownload(t *testing.T) {
	wantBody := []byte(`<html>
<body>
<ul>
  <li>
    <a href="/test-data/project01/simple.html">simple.html</a>
  </li>
  <li>
    <a href="/test-data/project01/href.html">href.html</a>
  </li>
  <li>
    <a href="/test-data/project01/style.html">style.html</a>
  </li>
</ul>
</body>
</html>`)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(wantBody)
	}))
	defer ts.Close()

	tests := []TestCase{
		{
			name:    "Download: Test Case 1",
			mockURL: ts.URL,
			body:    wantBody,
			err:     nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := download(tc.mockURL)
			if err != nil {
				t.Fatalf("Download (testcase) error")
			}
			if !reflect.DeepEqual(got, tc.body) {
				t.Errorf("download(%v) = %v; expected %v", tc.mockURL, got, tc.body)
			}
		})
	}
}

// -------------------------------------------------------
// TestExtract: Check extraction of words and hrefs from HTML.
// -------------------------------------------------------
func TestExtract(t *testing.T) {

	type extractCase struct {
		name      string
		body      []byte
		wantWords []string
		wantHrefs []string
	}

	tests := []extractCase{
		{
			name: "Simple body with links",
			body: []byte(`
			<body>
				<p>Hello World!</p>
				<p>Welcome to <a href="https://usf-cs272-f25.github.io/">CS272</a>!</p>
				<a href="/syllabus/">Syllabus</a>
				<a href="mailto:prof@usf.edu">Email</a>
			</body>`),
			wantWords: []string{"Hello", "World", "Welcome", "to", "CS272", "Syllabus", "Email"},
			wantHrefs: []string{
				"https://usf-cs272-f25.github.io/",
				"/syllabus/",
				"mailto:prof@usf.edu",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			words, hrefs := extract(tc.body)
			if !reflect.DeepEqual(words, tc.wantWords) {
				t.Errorf("words got %v, want %v", words, tc.wantWords)
			}
			if !reflect.DeepEqual(hrefs, tc.wantHrefs) {
				t.Errorf("hrefs got %v, want %v", hrefs, tc.wantHrefs)
			}
		})
	}
}

// -------------------------------------------------------
// TestCleanHref
// -------------------------------------------------------
func TestCleanHref(t *testing.T) {
	base := "https://usf-cs272-f25.github.io/"
	tests := []struct {
		name string
		href string
		want string
	}{
		{"Root", "/", "https://usf-cs272-f25.github.io/"},
		{"Help", "/help/", "https://usf-cs272-f25.github.io/help/"},
		{"Syllabus", "/syllabus/", "https://usf-cs272-f25.github.io/syllabus/"},
		{"Absolute", "https://usf-cs272-f25.github.io/test-data/project01/", "https://usf-cs272-f25.github.io/test-data/project01/"},
		{"Full URL", "https://example.com/page.html", "https://example.com/page.html"},
		{"Relative", "page.html", "https://usf-cs272-f25.github.io/page.html"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := clean(base, tc.href)
			if got != tc.want {
				t.Errorf("clean(%q,%q) = %q; want %q", base, tc.href, got, tc.want)
			}
		})
	}
}

// -------------------------------------------------------
// TestCrawl
// -------------------------------------------------------
func TestCrawl(t *testing.T) {

	type crawlCase struct {
		name        string
		rootHTML    string
		aHTML       string
		bHTML       string
		offHTML     string
		wantVisited map[string]bool
	}

	var ts *httptest.Server
	case1 := crawlCase{
		name: "Visits all same-host pages linked from root",
		rootHTML: `<html><body>
			<a href="/a">A</a>
			<a href="/b">B</a>
			<a href="/off">Off</a>
			<a href="mailto:x@y">mail</a>
		</body></html>`,
		aHTML:   `<html><head><title>Alpha</title></head><body>Page A</body></html>`,
		bHTML:   `<html><head><title>Beta</title></head><body>Page B</body></html>`,
		offHTML: `<html><body>Off Host But Same Server</body></html>`,
	}

	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/", "":
			w.Write([]byte(case1.rootHTML))
		case "/a":
			w.Write([]byte(case1.aHTML))
		case "/b":
			w.Write([]byte(case1.bHTML))
		case "/off":
			w.Write([]byte(case1.offHTML))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	case1.wantVisited = map[string]bool{
		ts.URL + "/":    true,
		ts.URL + "/a":   true,
		ts.URL + "/b":   true,
		ts.URL + "/off": true,
	}

	t.Run(case1.name, func(t *testing.T) {
		tracked := make(map[string]map[string]int)
		crawl(ts.URL+"/", tracked, nil)

		visited := map[string]bool{}
		for _, postings := range tracked {
			for u := range postings {
				visited[u] = true
			}
		}
		if !reflect.DeepEqual(visited, case1.wantVisited) {
			t.Errorf("visited got %v, want %v", visited, case1.wantVisited)
		}
	})
}

// -------------------------------------------------------
// TestSearch
// -------------------------------------------------------
func TestSearch(t *testing.T) {
	root := "https://usf-cs272-f25.github.io/test-data/rnj/"
	if _, err := url.Parse(root); err != nil {
		t.Fatalf("bad base url: %v", err)
	}
	tracked := make(map[string]map[string]int)
	crawl(root, tracked, nil)

	stem := func(term string) string {
		s, _ := snowball.Stem(term, "english", true)
		return s
	}

	type single struct {
		term string
		doc  string
		want int
	}
	singles := []single{
		{"Verona", root + "sceneI_30.0.html", 1},
		{"Benvolio", root + "sceneI_30.1.html", 26},
	}
	for _, tc := range singles {
		t.Run("single-"+tc.term, func(t *testing.T) {
			if got := tracked[stem(tc.term)][tc.doc]; got != tc.want {
				t.Errorf("%s in %s: got %d, want %d", tc.term, tc.doc, got, tc.want)
			}
		})
	}

	type pair struct {
		url  string
		want int
	}
	romeo := []pair{
		{strings.ToLower(root + "sceneI_30.0.html"), 2},
		{strings.ToLower(root + "sceneI_30.1.html"), 22},
		{strings.ToLower(root + "sceneI_30.2.html"), 15},
		{strings.ToLower(root + "sceneI_30.3.html"), 2},
		{strings.ToLower(root + "sceneI_30.4.html"), 17},
		{strings.ToLower(root + "sceneI_30.5.html"), 15},
		{strings.ToLower(root + "sceneII_30.0.html"), 3},
		{strings.ToLower(root + "sceneII_30.1.html"), 10},
		{strings.ToLower(root + "sceneII_30.2.html"), 42},
		{strings.ToLower(root + "sceneII_30.3.html"), 13},
		{strings.ToLower(root), 199},
	}

	gotRomeo := tracked[stem("Romeo")]
	gotLower := make(map[string]int, len(gotRomeo))
	for u, c := range gotRomeo {
		gotLower[strings.ToLower(u)] = c
	}
	for _, p := range romeo {
		t.Run("romeo-"+p.url, func(t *testing.T) {
			if got := gotLower[p.url]; got != p.want {
				t.Errorf("romeo count for %s: got %d, want %d", p.url, got, p.want)
			}
		})
	}
	if len(gotLower) != len(romeo) {
		t.Errorf("romeo unexpected url count: got %d, want %d", len(gotLower), len(romeo))
	}
}

// -------------------------------------------------------
// TestStop
// -------------------------------------------------------
func TestStop(t *testing.T) {
	type tc struct {
		word   string
		isStop bool
	}
	tests := []tc{
		{"the", true},
		{"and", true},
		{"to", true},
		{"Computer", false},
		{"Science", false},
		{"sunny", false},
		{"San", false},
		{"Francisco", false},
	}

	list, err := LoadStopwords("stopwords-en.json")
	if err != nil {
		t.Fatalf("LoadStopwords error: %v", err)
	}
	set := StopwordSet(list)

	for _, tt := range tests {
		t.Run(tt.word, func(t *testing.T) {
			_, ok := set[strings.ToLower(tt.word)]
			if ok != tt.isStop {
				t.Errorf("word %q stop? got %v, want %v", tt.word, ok, tt.isStop)
			}
		})
	}
}

// -------------------------------------------------------
// TestTfIdf
// -------------------------------------------------------
func TestTfIdf(t *testing.T) {
	// Serve local ./top10 under /top10/
	ts := httptest.NewServer(http.StripPrefix("/top10/", http.FileServer(http.Dir("./top10"))))
	defer ts.Close() // server stops when the test ends

	seed := ts.URL + "/top10/"

	tracked := make(map[string]map[string]int)
	idx := NewInvertedIndex()

	// Build the index by crawling the served corpus
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

	pickMatch := func(hits Hits, avoid string, want string) (string, bool) { // helper function
		want = strings.ToLower(want)
		for i := 0; i < len(hits) && i < 10; i++ {
			u := strings.ToLower(hits[i].URL)
			if u == strings.ToLower(avoid) {
				continue // skip the root page (/top10/)
			}
			if strings.Contains(u, want) {
				return hits[i].URL, true // found a suitable match
			}
		}
		return "", false
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			hits := idx.TFIDF(tt.query)
			if len(hits) == 0 {
				t.Fatalf("no TF-IDF hits for %q", tt.query)
			}

			matchURL, ok := pickMatch(hits, seed, tt.expectContains)
			if !ok {
				var top []string
				for i := 0; i < len(hits) && i < 5; i++ {
					top = append(top, hits[i].URL)
				}
				t.Fatalf("no suitable hit for %q containing %q; top URLs: %v", tt.query, tt.expectContains, top)
			}

			if hits[0].Score < 0 {
				t.Errorf("unexpected negative TF-IDF score for top hit %q: %f", hits[0].URL, hits[0].Score)
			}

			_ = matchURL
		})
	}
}

// -------------------------------------------------------
// TestDisallow
// -------------------------------------------------------
func TestDisallow(t *testing.T) {
	type tc struct {
		name       string
		robotsTxt  string   // content of robots.txt
		indexLinks []string // links on the seed page
	}

	tests := []tc{ //from spec
		{
			name: "Disallow chap21 via wildcard",
			robotsTxt: "# Lab05 robots.txt\n\n" +
				"User-agent: *\n" +
				"Disallow: *chap21.html\n" +
				"Crawl-delay: 1\n",
			indexLinks: []string{
				"./chap10.html", // allowed
				"./chap21.html", // disallowed
			},
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {

			robots = make(map[string]*hostRules)

			mux := http.NewServeMux() // Create a multiplexer to register handlers for our in-memory test server

			// robots.txt endpoint returns the exact content from the spec
			mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, c.robotsTxt)
			})

			// seed page with links
			mux.HandleFunc("/book/index.html", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, "<html><body>index ")
				for _, l := range c.indexLinks {
					fmt.Fprintf(w, `<a href="%s">alpha</a>`, l)
				}
				fmt.Fprint(w, "</body></html>")
			})

			// allowed page
			mux.HandleFunc("/book/chap10.html", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, "<html><body>alpha chap10</body></html>")
			})

			// disallowed page (crawler should skip this)
			mux.HandleFunc("/book/chap21.html", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, "<html><body>alpha chap21</body></html>")
			})

			ts := httptest.NewServer(mux)
			defer ts.Close()

			seed := ts.URL + "/book/index.html"

			// run crawl with legacy freq map
			freq := make(map[string]map[string]int)
			_ = crawl(seed, freq, nil)

			// reconstruct visited set from freq map
			visited := map[string]bool{}
			for _, postings := range freq {
				for u := range postings {
					visited[u] = true
				}
			}

			// expected: seed + allowed page only
			want := map[string]bool{
				seed:                         true,
				ts.URL + "/book/chap10.html": true,
			}

			// must NOT include chap21.html
			if visited[ts.URL+"/book/chap21.html"] {
				t.Fatalf("expected chap21.html to be DISALLOWED by robots.txt, but it was crawled")
			}

			if !reflect.DeepEqual(visited, want) {
				t.Errorf("visited got %v, want %v", visited, want)
			}
		})
	}
}

// -------------------------------------------------------
// TestCrawlDelay
// -------------------------------------------------------
func TestCrawlDelay(t *testing.T) {
	type tc struct {
		name     string
		delaySec int
		nChaps   int           // number of chapter links on the seed page
		minTime  time.Duration // minimum total crawl time expected
	}

	tests := []tc{
		{
			name:     "Crawl-delay 1s, ~10s minimum",
			delaySec: 1,
			nChaps:   12,
			minTime:  10 * time.Second,
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			// fresh robots cache each run
			robots = make(map[string]*hostRules)

			mux := http.NewServeMux()

			mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, "User-agent: *\nCrawl-delay: %d\n", c.delaySec)
			})

			base := "/top10/Dracula%20%7C%20Project%20Gutenberg"
			mux.HandleFunc(base+"/index.html", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, "<html><body>")
				for i := 1; i <= c.nChaps; i++ {
					fmt.Fprintf(w, `<a href="./chap%02d.html">link</a>`, i)
				}
				fmt.Fprint(w, "</body></html>")
			})

			for i := 1; i <= c.nChaps; i++ { // Registers each chapter path with a simple HTML body
				path := base + fmt.Sprintf("/chap%02d.html", i)
				mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
					// simple HTML body
					fmt.Fprint(w, "<html><body>chapter</body></html>")
				})
			}

			ts := httptest.NewServer(mux)
			defer ts.Close()

			seed := ts.URL + base + "/index.html"

			start := time.Now()
			freq := make(map[string]map[string]int)
			_ = crawl(seed, freq, nil)
			elapsed := time.Since(start) // total crawl time

			if elapsed < c.minTime {
				t.Fatalf("crawler ran too fast: got %v, want at least %v", elapsed, c.minTime)
			}
		})
	}
}
