package main

import (
	"bytes"
	"strings"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type ExtractResult struct {
	URL   string
	Words []string
	Hrefs []string
}

// EXTRACT: extract all of the words between
func extract(Dout chan DownloadResult, EOut chan ExtractResult) {
	for result := range Dout {
		if result.Err != nil { // skip if download had an error
			continue
		}
		w, h := extractHelper(result.Body)
		EOut <- ExtractResult{URL: result.URL, Words: w, Hrefs: h}

	}
}

func extractHelper(body []byte) ([]string, []string) {
	var words []string
	var hrefs []string

	// Parse HTML

	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		print("Error parsing HTML")
		return nil, nil
	}

	// Helper to trim
	trimed := func(s string) string {
		return strings.TrimFunc(s, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		})
	}

	// Find <title> and <body>
	var titleWords []string

	var findTitle func(*html.Node)
	findTitle = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.Title {
			// Found <title>
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.TextNode {
					for _, tok := range strings.Fields(c.Data) {
						if w := trimed(tok); w != "" {
							titleWords = append(titleWords, w)
						}
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findTitle(c)
		}
	}
	findTitle(doc)

	// Find <body>
	var findBody func(*html.Node) *html.Node
	findBody = func(n *html.Node) *html.Node {
		if n.Type == html.ElementNode && n.DataAtom == atom.Body {
			return n
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if b := findBody(c); b != nil {
				return b
			}
		}
		return nil
	}
	root := findBody(doc)
	if root == nil {
		root = doc
	}

	// analyze tree and collect words/hrefs ---
	var analyzeTree func(*html.Node)
	analyzeTree = func(n *html.Node) {
		switch n.Type {
		case html.TextNode:
			for _, tok := range strings.Fields(n.Data) {
				if w := trimed(tok); w != "" {
					words = append(words, w)
				}
			}
		case html.ElementNode:
			if n.DataAtom == atom.A {
				for _, a := range n.Attr {
					if a.Key == "href" {
						if v := strings.TrimSpace(a.Val); v != "" {
							hrefs = append(hrefs, v)
						}
						break
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			analyzeTree(c)
		}
	}
	analyzeTree(root)

	// Add title words at the start
	words = append(titleWords, words...)

	return words, hrefs
}
