package main

import (
	"net/url"
	"strings"
)

// CLEAN INPUT TYPE //
type CleanInput struct {
	Base string
	Href string
}

func clean(CIn chan CleanInput, COut chan string) {

	for input := range CIn {
		base := input.Base
		href := input.Href

		if strings.HasPrefix(href, "mailto:") {
			continue
		}

		bu, err := url.Parse(base)
		if err != nil {
			continue
		}

		hu, err := url.Parse(strings.TrimSpace(href))
		if err != nil {
			continue
		}

		if hu.Hostname() != bu.Hostname() && hu.Hostname() != "" {
			continue
		}

		abs := bu.ResolveReference(hu) // Resolve hu against the base bu (relative/absolute)
		abs.Fragment = ""
		COut <- abs.String()
	}
}
