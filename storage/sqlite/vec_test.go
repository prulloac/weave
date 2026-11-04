package sqlite

import (
	"database/sql"
	"encoding/binary"
	"math"
	"testing"

	_ "modernc.org/sqlite"
	_ "modernc.org/sqlite/vec"
)

func TestVec0BlobProtocol(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE VIRTUAL TABLE vec_probe USING vec0(
		bundle_root TEXT PARTITION KEY,
		doc_key TEXT PRIMARY KEY,
		doc_id TEXT,
		content_hash TEXT,
		embedding float[3] distance_metric=cosine)`); err != nil {
		t.Fatal(err)
	}
	blob := func(values ...float32) []byte {
		data := make([]byte, 4*len(values))
		for i, v := range values {
			binary.LittleEndian.PutUint32(data[i*4:], math.Float32bits(v))
		}
		return data
	}
	if _, err := db.Exec(`INSERT INTO vec_probe(doc_key, bundle_root, doc_id, content_hash, embedding)
		VALUES ('root/a', 'root', 'a', 'h', vec_f32(?))`, blob(1, 0, 0)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO vec_probe(doc_key, bundle_root, doc_id, content_hash, embedding)
		VALUES ('root/b', 'root', 'b', 'h', vec_f32(?))`, blob(0, 1, 0)); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query(`SELECT doc_id, distance FROM vec_probe
		WHERE embedding MATCH vec_f32(?) AND bundle_root = ? AND k = ?`, blob(1, 0, 0), "root", 2)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id string
		var distance float64
		if err := rows.Scan(&id, &distance); err != nil {
			t.Fatal(err)
		}
		count++
	}
	if count != 2 {
		t.Fatalf("count = %d", count)
	}
}
