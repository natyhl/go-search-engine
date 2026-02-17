# Go Search Engine (Crawler + Inverted Index)

A lightweight search engine built in Go featuring a web crawler, text extraction pipeline, and an inverted index for fast keyword-based retrieval.

This project demonstrates core information retrieval concepts and backend engineering fundamentals: crawling, parsing, indexing, and query processing.

## Features
- Web crawling and robots handling
- HTML cleaning + text extraction
- Tokenization and stopword filtering
- Inverted index construction
- Query lookup and ranked results (extendable to TF-IDF)

## Tech Stack
- Go
- Custom indexing + retrieval logic

## Project Structure
- `crawl.go` / `download.go`: crawling and fetching
- `extract.go` / `clean.go`: extraction + normalization
- `invertedindex.go` / `index.go`: indexing + lookup
- `stopwords*.json` / `stopwords.go`: stopword filtering
