package toasts

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // pure Go, no cgo: builds for Windows ARM64 without a C toolchain
)

// ClaudeAUMID is the notification handler of the Claude Desktop Store app.
const ClaudeAUMID = "Claude_pzs8sxrjxfjjc!Claude"

// MaxAge: older toasts are ignored.
const MaxAge = 12 * time.Hour

// Stored is a toast with its arrival time.
type Stored struct {
	Toast
	Arrival time.Time
}

// ReadDB copies wpndatabase.db (and its -wal) from srcDir into tmpDir, never
// touching the original, and returns the newest Claude toasts that arrived
// after since. A missing table or column is an error, never an empty list.
func ReadDB(srcDir, tmpDir string, since time.Time) ([]Stored, error) {
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		return nil, err
	}
	for _, n := range []string{"wpndatabase.db", "wpndatabase.db-wal", "wpndatabase.db-shm"} {
		_ = os.Remove(filepath.Join(tmpDir, n))
	}
	if err := copyFile(filepath.Join(srcDir, "wpndatabase.db"), filepath.Join(tmpDir, "wpndatabase.db")); err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(srcDir, "wpndatabase.db-wal")); err == nil {
		if err := copyFile(filepath.Join(srcDir, "wpndatabase.db-wal"), filepath.Join(tmpDir, "wpndatabase.db-wal")); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", filepath.Join(tmpDir, "wpndatabase.db"))
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`select n.Id, n.ArrivalTime, n.Payload from Notification n
		join NotificationHandler h on h.RecordId = n.HandlerId
		where h.PrimaryId = ? and n.Type = 'toast'
		order by n.ArrivalTime desc limit 40`, ClaudeAUMID)
	if err != nil {
		return nil, fmt.Errorf("notification database: %w", err)
	}
	defer rows.Close()
	var out []Stored
	for rows.Next() {
		var id, arrival int64
		var payload []byte
		if err := rows.Scan(&id, &arrival, &payload); err != nil {
			return nil, fmt.Errorf("notification database: %w", err)
		}
		t := FromFiletime(arrival)
		if t.Before(since) {
			continue
		}
		out = append(out, Stored{Toast: Toast{ID: id, Payload: string(payload)}, Arrival: t})
	}
	return out, rows.Err()
}

// FromFiletime converts a Windows FILETIME (100 ns ticks since 1601, UTC).
func FromFiletime(ft int64) time.Time {
	const ticksTo1970 = 116444736000000000
	return time.Unix(0, (ft-ticksTo1970)*100).UTC()
}

// ToFiletime is the inverse of FromFiletime (used by tests).
func ToFiletime(t time.Time) int64 {
	const ticksTo1970 = 116444736000000000
	return t.UnixNano()/100 + ticksTo1970
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
