package main

import (
	"database/sql"
	"log"
	"strings"

	"github.com/kljensen/snowball"
	_ "github.com/mattn/go-sqlite3"
)

type SqlIndex struct {
	db       *sql.DB
	DocLen   map[string]int // map of document lengths
	DocCount int            // total number of documents
}

func NewSqlIndex(path string, reset bool) (*SqlIndex, error) {

	db, err := sql.Open("sqlite3", path) //create new database
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		return nil, err
	}

	if reset {
		if _, err := db.Exec(`
			DROP TABLE IF EXISTS frequencies;
			DROP TABLE IF EXISTS terms;
			DROP TABLE IF EXISTS documents;
		`); err != nil {
			return nil, err
		}
	}

	if _, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS documents(
			doc_id     INTEGER PRIMARY KEY,
			url        TEXT UNIQUE NOT NULL,
			doc_length INTEGER DEFAULT 0
		);
		CREATE TABLE IF NOT EXISTS terms(
			term_id INTEGER PRIMARY KEY,
			term    TEXT UNIQUE NOT NULL 
		);
		CREATE TABLE IF NOT EXISTS frequencies(
			freq_id INTEGER PRIMARY KEY,
			term_id INTEGER,
			doc_id  INTEGER,
			tf      INTEGER,
			FOREIGN KEY(term_id) REFERENCES terms(term_id),
			FOREIGN KEY(doc_id) REFERENCES documents(doc_id),
			UNIQUE(term_id, doc_id)
		);
	`); err != nil {
		return nil, err
	}

	return &SqlIndex{
		db:     db,
		DocLen: make(map[string]int),
	}, nil
}

func (s *SqlIndex) AddWord(url, word string) {
	w := strings.ToLower(strings.TrimSpace(word))
	if w == "" {
		return
	}

	stemmed, _ := snowball.Stem(w, "english", true)
	if stemmed == "" {
		return
	}

	var docID int64
	err := s.db.QueryRow(`SELECT doc_id FROM documents WHERE url = ?`, url).Scan(&docID)
	if err == sql.ErrNoRows { // ErrNoRows is returned by Row.Scan when DB.QueryRow doesn't return a row. In such a case, QueryRow returns a placeholder *Row value that defers this error until a Scan
		res, err := s.db.Exec(`INSERT INTO documents(url, doc_length) VALUES(?, 0)`, url)
		if err != nil {
			log.Println("insert documents:", err)
			return
		}
		docID, _ = res.LastInsertId() // retrieve the auto-generated ID of the last row inserted into a table
		s.DocCount++                  // optional cache
	} else if err != nil {
		log.Println("select documents:", err)
		return
	}
	var termID int64
	err = s.db.QueryRow(`SELECT term_id FROM terms WHERE term = ?`, stemmed).Scan(&termID)
	if err == sql.ErrNoRows {
		res, err := s.db.Exec(`INSERT INTO terms(term) VALUES(?)`, stemmed)
		if err != nil {
			log.Println("insert terms:", err)
			return
		}
		termID, _ = res.LastInsertId()
	} else if err != nil {
		log.Println("select terms:", err)
		return
	}
	// updte frequencies table: increment tf by 1 for this term/doc pair
	if _, err := s.db.Exec(`
		INSERT INTO frequencies(term_id, doc_id, tf)
		VALUES(?, ?, 1)
		ON CONFLICT(term_id, doc_id) DO UPDATE SET tf = tf + 1;
	`, termID, docID); err != nil {
		log.Println("upsert frequencies:", err)
		return
	}

	// increment doc_length by 1 for this document
	if _, err := s.db.Exec(`
		UPDATE documents SET doc_length = doc_length + 1 WHERE doc_id = ?;
	`, docID); err != nil {
		log.Println("update doc_length:", err)
		return
	}
}

func (s *SqlIndex) AddDocument(url string, words []string) {
	tx, err := s.db.Begin() // start transaction
	if err != nil {
		log.Println("begin transaction:", err)
		return
	}

	if _, err := tx.Exec(`INSERT OR IGNORE INTO documents(url, doc_length) VALUES(?, 0)`, url); err != nil {
		log.Println("insert documents:", err)
		tx.Rollback() // reverses all data modifications performed since the beginning of the current transaction
		return
	}

	for _, w := range words {
		if err := addWordInTransaction(tx, url, w); err != nil {
			log.Println("add word in transaction:", err)
			tx.Rollback()
			return
		}
	}
	if err := tx.Commit(); err != nil {
		log.Println("commit transaction:", err)
		return
	}
}

func addWordInTransaction(tx *sql.Tx, url, word string) error {
	w := strings.ToLower(strings.TrimSpace(word))
	if w == "" {
		return nil
	}

	stemmed, _ := snowball.Stem(w, "english", true)
	if stemmed == "" {
		return nil
	}

	var docID int64
	err := tx.QueryRow(`SELECT doc_id FROM documents WHERE url = ?`, url).Scan(&docID)
	if err == sql.ErrNoRows {
		res, err := tx.Exec(`INSERT INTO documents(url, doc_length) VALUES(?, 0)`, url)
		if err != nil {
			return err
		}
		docID, _ = res.LastInsertId()
	} else if err != nil {
		return err
	}

	var termID int64
	err = tx.QueryRow(`SELECT term_id FROM terms WHERE term = ?`, stemmed).Scan(&termID)
	if err == sql.ErrNoRows {
		res, err := tx.Exec(`INSERT INTO terms(term) VALUES(?)`, stemmed)
		if err != nil {
			return err
		}
		termID, _ = res.LastInsertId()
	} else if err != nil {
		return err
	}

	if _, err := tx.Exec(`
		INSERT INTO frequencies(term_id, doc_id, tf)
		VALUES(?, ?, 1)
		ON CONFLICT(term_id, doc_id) DO UPDATE SET tf = tf + 1;
	`, termID, docID); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		UPDATE documents SET doc_length = doc_length + 1 WHERE doc_id = ?;
	`, docID); err != nil {
		return err
	}

	return nil
}

func (s *SqlIndex) Search(term string) Hits {
	stemmed, _ := snowball.Stem(strings.ToLower(term), "english", true)
	if stemmed == "" {
		return nil
	}

	rows, err := s.db.Query(`
		SELECT d.url, f.tf, d.doc_length
		FROM terms t
		JOIN frequencies f ON f.term_id = t.term_id
		JOIN documents  d ON d.doc_id  = f.doc_id
		WHERE t.term = ?;
	`, stemmed)
	if err != nil {
		log.Println("search query:", err)
		return nil
	}
	defer rows.Close()

	var hits Hits
	for rows.Next() {
		var url string
		var tfCount, docLen int
		if err := rows.Scan(&url, &tfCount, &docLen); err != nil {
			log.Println("search scan:", err)
			return nil
		}
		if docLen > 0 {
			tf := float64(tfCount) / float64(docLen)
			hits = append(hits, SearchResult{URL: url, TF: tf, Score: tf})
		}
	}
	hits.Sort()
	return hits
}

func (s *SqlIndex) Postings(stem string) []Posting { // returns raw counts + doc lengths for a stem
	var termID int64 // term_id for this stem
	if err := s.db.QueryRow(`SELECT term_id FROM terms WHERE term = ?`, stem).Scan(&termID); err != nil {
		return nil // no such term or db error
	}

	rows, err := s.db.Query(`SELECT doc_id, tf FROM frequencies WHERE term_id = ?`, termID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []Posting
	for rows.Next() {
		var docID int64
		var tf int
		if err := rows.Scan(&docID, &tf); err != nil {
			return nil
		}

		var url string
		var dl int // doc_length
		if err := s.db.QueryRow(`SELECT url, doc_length FROM documents WHERE doc_id = ?`, docID).Scan(&url, &dl); err != nil {
			continue
		}

		out = append(out, Posting{URL: url, Count: tf, DocLen: dl})
	}
	return out
}

func (s *SqlIndex) TotalDocs() int {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM documents WHERE doc_length > 0`).Scan(&n); err != nil {
		return 0
	}
	return n
}

func (s *SqlIndex) TFIDF(term string) Hits {
	return computeTFIDF(s, term)
}
