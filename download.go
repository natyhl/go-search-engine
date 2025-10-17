package main

import (
	"io"
	"net/http"
)

// DOWNLOAD //
func download(DIn chan, DOut chan) {
	for url := range DIn {
		body, err := downloadHelper(url)
		if err != nil {
			print("Error downloading URL:", url, err)
			continue
		}
		DOut <- body
	}
	close(DOut) // why do we need this?
}

func downloadHelper(url string) ([]byte, error) {
	// Download HTML
	resp, err := http.Get(url) // Source: https://go.dev/play/p/WF3Ctw7SVnf
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()

	// print(string(body))
	return body, err
}
