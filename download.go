package main

import (
	"io"
	"net/http"
)

// DOWNLOAD RESULT TYPE //
type DownloadResult struct {
	URL  string
	Body []byte
	Err  error
}

// DOWNLOAD //
func download(DIn chan string, DOut chan DownloadResult) {
	for url := range DIn {
		body, err := downloadHelper(url)
		if err != nil {
			print("Error downloading URL:", url, err)
			continue
		}
		DOut <- DownloadResult{URL: url, Body: body, Err: err}
	}
}

func downloadHelper(url string) ([]byte, error) {
	// Download HTML
	resp, err := http.Get(url) // Source: https://go.dev/play/p/WF3Ctw7SVnf
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()

	return body, err
}
