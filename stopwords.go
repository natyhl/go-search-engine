package main

import (
	"os"
	"strings"
)

// loads a stopword list from a JSON file
func LoadStopwords(path string) ([]string, error) {

	data, err := os.ReadFile(path)

	if err != nil {
		return nil, err
	}

	// Remove square brackets at start and end
	s := strings.TrimSpace(string(data))
	if strings.HasPrefix(s, "[") {
		s = s[1:]
	}
	if strings.HasSuffix(s, "]") {
		s = s[:len(s)-1]
	}

	items := strings.Split(s, ",")
	var words []string
	for _, item := range items {
		w := strings.Trim(item, "\"' \n\r\t")
		if w != "" {
			words = append(words, w)
		}
	}
	return words, nil
}

// Helper to build a set for fast lookup
func StopwordSet(stopwords []string) map[string]struct{} {
	set := make(map[string]struct{}, len(stopwords))
	for _, w := range stopwords {
		set[w] = struct{}{}
	}
	return set
}
