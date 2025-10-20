package main

import (
	"bufio"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

type hostRules struct {
	disallow []string      // each Disallow line compiled into a regex --> pattern
	delay    time.Duration // how long to wait
	lastF    time.Time     // last time we downloaded from this host
	mu       sync.Mutex    // prevent race conditions, only one goroutine can access a shared resource at a time
	initialized bool      // CHANGED: set true once robots.txt parsed
}

var robots = make(map[string]*hostRules)    // per-host rules: key = host, value = pointer to host’s rules
var robotsMu sync.Mutex // CHANGED: protect the global robots map
const defaultDelay = 100 * time.Millisecond //given by spec

func rulesForHost(u *url.URL) *hostRules { // build rules for a host if not already done
	host := u.Host                 // extract host from URL

	// robotsMu.Lock()                // CHANGED: serialize access/creation
	// if r, ok := robots[host]; ok { //if we already have rules return them
	// 	robotsMu.Unlock()          // CHANGED
	// 	return r
	// }

	// r := &hostRules{delay: defaultDelay} // create a *hostRules with default delay
	// robots[host] = r               // CHANGED: write under the lock
	// robotsMu.Unlock()              // CHANGED: done touching the map

	robotsMu.Lock()
	r, ok := robots[host]
	if !ok {
		// Insert a placeholder BEFORE network I/O so others see a non-nil pointer.
		r = &hostRules{delay: defaultDelay}
		robots[host] = r // CHANGED: single write to the map under lock
	}
	robotsMu.Unlock()

	// Initialize r exactly once; serialize with r.mu so readers won’t race while we fill fields.
	r.mu.Lock()
	if r.initialized {
		r.mu.Unlock()
		return r
	}

	resp, _ := http.Get(u.Scheme + "://" + host + "/robots.txt") // try to download robots.txt
	if resp == nil {
		//robots[host] = r
		r.initialized = true // CHANGED
		r.mu.Unlock()
		return r
	}
	defer resp.Body.Close() // always close the body of an HTTP request

	scanner := bufio.NewScanner(resp.Body) // line scanner over the robots.txt
	curUA := ""                            // current User-agent we’re processing
	for scanner.Scan() {                   // for each line
		line := scanner.Text()

		if i := strings.Index(line, "#"); i >= 0 { // skip comments and blank lines
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		col := strings.Index(line, ":") // Split Key: value
		if col < 0 {                    // skip lines without a colon
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:col]))
		val := strings.TrimSpace(line[col+1:])

		switch key {
		case "user-agent":
			curUA = strings.ToLower(val)

		case "disallow":
			// only apply rules under "User-agent: *"
			if curUA == "*" && val != "" {
				r.disallow = append(r.disallow, val)
			}
		case "crawl-delay":
			// only apply rules under "User-agent: *"
			if curUA == "*" && val != "" {
				if d, err := time.ParseDuration(val + "s"); err == nil {
					r.delay = d
				}
			}
		}
	}
	//robots[host] = r
	r.initialized = true // CHANGED
	r.mu.Unlock()
	return r
}

func isDisallowed(r *hostRules, path string) bool { // does path match any Disallow pattern?
	for _, pat := range r.disallow {
		re := regexp.QuoteMeta(pat)                                  // literal characters like . and * are treated as regular characters, not regex operators
		re = strings.ReplaceAll(re, `\*`, ".*")                      // turn * into .*
		if matched, _ := regexp.MatchString("^"+re, path); matched { // Tests if the path matches the pattern. The ^ ensures matching from the start of the path
			return true
		}
	}
	return false
}

func waitForDelay(r *hostRules) {
	r.mu.Lock()         // lock the hostRules while we check and update lastF
	defer r.mu.Unlock() // unlock when done
	wait := r.delay - time.Since(r.lastF)
	if wait > 0 {
		time.Sleep(wait)
	}
	r.lastF = time.Now()
}
