package main

import (
	"net/url"
	"strings"
)

// CLEAN: helper to clean a single href
func clean(CIn chan, COut chan) string {

	if strings.HasPrefix(href, "mailto:") {
		return ""
	}

	bu, err := url.Parse(base)
	if err != nil {
		return ""
	}
	hu, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return ""
	}

	if hu.Hostname() != bu.Hostname() && hu.Hostname() != "" {
		return ""
	}

	abs := bu.ResolveReference(hu) // Resolve hu against the base bu (relative/absolute)
	abs.Fragment = ""
	return abs.String()
}
