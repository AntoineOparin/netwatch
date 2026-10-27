// Package devices keeps the list of every device ever seen on the LAN, keyed
// by MAC address: a device's IP can change (DHCP), its MAC stays put.
package devices

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/AntoineOparin/netwatch/internal/collector/discovery"
)

const schema = `
CREATE TABLE IF NOT EXISTS devices (
	mac           TEXT PRIMARY KEY,
	ip            TEXT NOT NULL,
	vendor        TEXT NOT NULL DEFAULT '',
	private_mac   INTEGER NOT NULL DEFAULT 0,
	hostname      TEXT NOT NULL DEFAULT '',
	nickname      TEXT NOT NULL DEFAULT '', -- set by the user
	is_self       INTEGER NOT NULL DEFAULT 0,
	is_router     INTEGER NOT NULL DEFAULT 0,
	first_seen_ms INTEGER NOT NULL,
	last_seen_ms  INTEGER NOT NULL,
	last_reply_ms INTEGER -- last answered ping; NULL if it never has
);
`

type Device struct {
	MAC       string
	IP        string
	Vendor    string
	Private   bool
	Hostname  string
	Nickname  string
	Self      bool
	Router    bool
	FirstSeen time.Time
	LastSeen  time.Time
	LastReply time.Time // zero if it never answered a ping
}

var ErrNotFound = errors.New("no such device")

type Store struct {
	db *sql.DB
}

func OpenStore(db *sql.DB) (*Store, error) {
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("create devices table: %w", err)
	}
	return &Store{db: db}, nil
}

// Seen records a sweep observation. It keeps the user's nickname and the
// first-seen time, and doesn't erase a known hostname when the router's DNS
// briefly has no answer.
func (s *Store) Seen(o discovery.Observation) error {
	at := o.At.UnixMilli()
	var reply any
	if o.Replied {
		reply = at
	}
	_, err := s.db.Exec(`
		INSERT INTO devices (mac, ip, vendor, private_mac, hostname, is_self, is_router, first_seen_ms, last_seen_ms, last_reply_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (mac) DO UPDATE SET
			ip = excluded.ip,
			vendor = excluded.vendor,
			private_mac = excluded.private_mac,
			hostname = CASE WHEN excluded.hostname != '' THEN excluded.hostname ELSE hostname END,
			is_self = excluded.is_self,
			is_router = excluded.is_router,
			last_seen_ms = excluded.last_seen_ms,
			last_reply_ms = COALESCE(excluded.last_reply_ms, last_reply_ms)`,
		o.MAC, o.IP, o.Vendor, o.Private, o.Hostname, o.Self, o.Router, at, at, reply,
	)
	if err != nil {
		return fmt.Errorf("save device %s: %w", o.MAC, err)
	}
	return nil
}

func (s *Store) List() ([]Device, error) {
	rows, err := s.db.Query(`SELECT mac, ip, vendor, private_mac, hostname, nickname, is_self, is_router,
		first_seen_ms, last_seen_ms, last_reply_ms FROM devices ORDER BY last_seen_ms DESC`)
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}
	defer rows.Close()

	var out []Device
	for rows.Next() {
		var d Device
		var first, last int64
		var reply sql.NullInt64
		if err := rows.Scan(&d.MAC, &d.IP, &d.Vendor, &d.Private, &d.Hostname, &d.Nickname, &d.Self, &d.Router, &first, &last, &reply); err != nil {
			return nil, err
		}
		d.FirstSeen, d.LastSeen = time.UnixMilli(first), time.UnixMilli(last)
		if reply.Valid {
			d.LastReply = time.UnixMilli(reply.Int64)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// SetNickname names a device; an empty nickname clears it.
func (s *Store) SetNickname(mac, nickname string) error {
	res, err := s.db.Exec(`UPDATE devices SET nickname = ? WHERE mac = ?`, nickname, mac)
	if err != nil {
		return fmt.Errorf("set nickname: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
