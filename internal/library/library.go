package library

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/judeadeniji/go-ytm/internal/session"

	_ "modernc.org/sqlite"
)

const schemaVersion = 7

// DB is the sqlite-backed local store (session, playlists, download cache).
type DB struct {
	sql  *sql.DB
	path string
	mu   sync.Mutex
}

// DefaultPath returns ~/.local/state/go-ytm/library.db (XDG_STATE_HOME aware).
func DefaultPath() (string, error) {
	state := os.Getenv("XDG_STATE_HOME")
	if state == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		state = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(state, "go-ytm", "library.db"), nil
}

// Open opens (or creates) the library database at the default path.
func Open() (*DB, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return OpenPath(path)
}

// OpenPath opens the library database at path.
func OpenPath(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	// Tighten perms on the DB file (created by sqlite with umask).
	_ = os.Chmod(path, 0o600)
	db := &DB{sql: sqlDB, path: path}
	if err := db.migrate(); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	if err := db.importLegacyJSONSession(); err != nil {
		// Non-fatal: keep going with empty/sqlite state.
		fmt.Fprintf(os.Stderr, "library: legacy session import: %v\n", err)
	}
	return db, nil
}

// Path returns the database file path.
func (db *DB) Path() string {
	if db == nil {
		return ""
	}
	return db.path
}

// Close closes the database.
func (db *DB) Close() error {
	if db == nil {
		return nil
	}
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.sql == nil {
		return nil
	}
	err := db.sql.Close()
	db.sql = nil
	return err
}

func (db *DB) migrate() error {
	if _, err := db.sql.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY
);`); err != nil {
		return err
	}

	var ver int
	err := db.sql.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&ver)
	if err != nil {
		return err
	}
	if ver >= schemaVersion {
		return nil
	}

	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if ver < 1 {
		stmts := []string{
			`CREATE TABLE IF NOT EXISTS session (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			active_menu TEXT NOT NULL DEFAULT 'Home',
			queue_panel_hidden INTEGER NOT NULL DEFAULT 0,
			search_filter TEXT NOT NULL DEFAULT '',
			last_search_query TEXT NOT NULL DEFAULT '',
			active_carousel INTEGER NOT NULL DEFAULT 0,
			home_card_cursor INTEGER NOT NULL DEFAULT 0,
			track_cursor INTEGER NOT NULL DEFAULT 0,
			list_cursor INTEGER NOT NULL DEFAULT 0,
			queue_cursor INTEGER NOT NULL DEFAULT 0,
			play_pos REAL NOT NULL DEFAULT 0,
			queue_index INTEGER NOT NULL DEFAULT -1,
			show_search INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		);`,
			`CREATE TABLE IF NOT EXISTS queue_track (
			position INTEGER PRIMARY KEY,
			video_id TEXT NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			artist TEXT NOT NULL DEFAULT '',
			thumbnail_url TEXT NOT NULL DEFAULT ''
		);`,
			`CREATE TABLE IF NOT EXISTS nav_stack (
			position INTEGER PRIMARY KEY,
			kind TEXT NOT NULL,
			entity_id TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL DEFAULT ''
		);`,
			`CREATE TABLE IF NOT EXISTS local_playlist (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		);`,
			`CREATE TABLE IF NOT EXISTS local_playlist_track (
			playlist_id INTEGER NOT NULL,
			position INTEGER NOT NULL,
			video_id TEXT NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			artist TEXT NOT NULL DEFAULT '',
			thumbnail_url TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (playlist_id, position),
			FOREIGN KEY (playlist_id) REFERENCES local_playlist(id) ON DELETE CASCADE
		);`,
			`CREATE TABLE IF NOT EXISTS download_cache (
			video_id TEXT PRIMARY KEY,
			path TEXT NOT NULL,
			bytes INTEGER NOT NULL DEFAULT 0,
			cached_at TEXT NOT NULL DEFAULT (datetime('now'))
		);`,
			`INSERT INTO schema_migrations(version) VALUES (1);`,
		}
		for _, s := range stmts {
			if _, err := tx.Exec(s); err != nil {
				return fmt.Errorf("migrate v1: %w", err)
			}
		}
		ver = 1
	}

	if ver < 2 {
		alters := []string{
			`ALTER TABLE queue_track ADD COLUMN artist_id TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE queue_track ADD COLUMN album TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE queue_track ADD COLUMN album_id TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE queue_track ADD COLUMN duration TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE queue_track ADD COLUMN is_explicit INTEGER NOT NULL DEFAULT 0`,
			`INSERT INTO schema_migrations(version) VALUES (2)`,
		}
		for _, s := range alters {
			if _, err := tx.Exec(s); err != nil {
				return fmt.Errorf("migrate v2: %w", err)
			}
		}
		ver = 2
	}

	if ver < 3 {
		alters := []string{
			`ALTER TABLE session ADD COLUMN volume REAL NOT NULL DEFAULT 100`,
			`ALTER TABLE session ADD COLUMN muted INTEGER NOT NULL DEFAULT 0`,
			`INSERT INTO schema_migrations(version) VALUES (3)`,
		}
		for _, s := range alters {
			if _, err := tx.Exec(s); err != nil {
				return fmt.Errorf("migrate v3: %w", err)
			}
		}
		ver = 3
	}

	if ver < 4 {
		alters := []string{
			`ALTER TABLE session ADD COLUMN play_duration REAL NOT NULL DEFAULT 0`,
			`ALTER TABLE session ADD COLUMN was_playing INTEGER NOT NULL DEFAULT 0`,
			`ALTER TABLE session ADD COLUMN now_playing_open INTEGER NOT NULL DEFAULT 0`,
			`INSERT INTO schema_migrations(version) VALUES (4)`,
		}
		for _, s := range alters {
			if _, err := tx.Exec(s); err != nil {
				return fmt.Errorf("migrate v4: %w", err)
			}
		}
		ver = 4
	}

	if ver < 5 {
		alters := []string{
			`ALTER TABLE session ADD COLUMN normalize INTEGER NOT NULL DEFAULT 0`,
			`INSERT INTO schema_migrations(version) VALUES (5)`,
		}
		for _, s := range alters {
			if _, err := tx.Exec(s); err != nil {
				return fmt.Errorf("migrate v5: %w", err)
			}
		}
		ver = 5
	}

	if ver < 6 {
		alters := []string{
			`ALTER TABLE session ADD COLUMN crossfade INTEGER NOT NULL DEFAULT 0`,
			`ALTER TABLE session ADD COLUMN crossfade_sec INTEGER NOT NULL DEFAULT 3`,
			`INSERT INTO schema_migrations(version) VALUES (6)`,
		}
		for _, s := range alters {
			if _, err := tx.Exec(s); err != nil {
				return fmt.Errorf("migrate v6: %w", err)
			}
		}
		ver = 6
	}

	if ver < 7 {
		alters := []string{
			`ALTER TABLE download_cache ADD COLUMN title TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE download_cache ADD COLUMN artist TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE download_cache ADD COLUMN album TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE download_cache ADD COLUMN duration TEXT NOT NULL DEFAULT ''`,
			`CREATE TABLE IF NOT EXISTS lyrics_cache (
				video_id TEXT PRIMARY KEY,
				instrumental INTEGER NOT NULL DEFAULT 0,
				plain_lyrics TEXT NOT NULL DEFAULT '',
				synced_lyrics TEXT NOT NULL DEFAULT '',
				cached_at TEXT NOT NULL DEFAULT (datetime('now'))
			)`,
			`INSERT INTO schema_migrations(version) VALUES (7)`,
		}
		for _, s := range alters {
			if _, err := tx.Exec(s); err != nil {
				return fmt.Errorf("migrate v7: %w", err)
			}
		}
	}

	return tx.Commit()
}

// importLegacyJSONSession migrates ~/.local/state/go-ytm/session.json once.
func (db *DB) importLegacyJSONSession() error {
	jsonPath, err := session.DefaultPath()
	if err != nil {
		return err
	}
	b, err := os.ReadFile(jsonPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	var n int
	if err := db.sql.QueryRow(`SELECT COUNT(*) FROM session`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil // already have sqlite session
	}

	var snap session.Snapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		return err
	}
	if err := db.SaveSession(snap); err != nil {
		return err
	}
	// Keep the JSON as backup but rename so we don't re-import.
	_ = os.Rename(jsonPath, jsonPath+".migrated")
	return nil
}

// LoadSession returns the persisted UI/playback snapshot, or nil if none.
func (db *DB) LoadSession() (*session.Snapshot, error) {
	if db == nil {
		return nil, nil
	}
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.sql == nil {
		return nil, fmt.Errorf("library: database closed")
	}

	var snap session.Snapshot
	var hidden, showSearch int
	var muted, wasPlaying, nowPlaying, normalize, crossfade int
	var crossfadeSec int
	err := db.sql.QueryRow(`
SELECT active_menu, queue_panel_hidden, search_filter, last_search_query,
       active_carousel, home_card_cursor, track_cursor, list_cursor, queue_cursor,
       play_pos, queue_index, show_search,
       COALESCE(volume, 100), COALESCE(muted, 0),
       COALESCE(play_duration, 0), COALESCE(was_playing, 0), COALESCE(now_playing_open, 0),
       COALESCE(normalize, 0),
       COALESCE(crossfade, 0), COALESCE(crossfade_sec, 3)
FROM session WHERE id = 1`).Scan(
		&snap.ActiveMenu, &hidden, &snap.SearchFilter, &snap.LastSearchQuery,
		&snap.ActiveCarousel, &snap.HomeCardCursor, &snap.TrackCursor, &snap.ListCursor, &snap.QueueCursor,
		&snap.PlayPos, &snap.QueueIndex, &showSearch,
		&snap.Volume, &muted,
		&snap.PlayDuration, &wasPlaying, &nowPlaying,
		&normalize,
		&crossfade, &crossfadeSec,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	snap.Version = schemaVersion
	snap.QueuePanelHidden = hidden != 0
	snap.ShowSearch = showSearch != 0
	snap.Muted = muted != 0
	snap.WasPlaying = wasPlaying != 0
	snap.NowPlayingOpen = nowPlaying != 0
	snap.Normalize = normalize != 0
	snap.Crossfade = crossfade != 0
	snap.CrossfadeSec = session.ClampCrossfadeSec(crossfadeSec)
	if snap.Volume < 0 {
		snap.Volume = 0
	}
	if snap.Volume > 100 {
		snap.Volume = 100
	}

	rows, err := db.sql.Query(`
SELECT video_id, title, artist, thumbnail_url,
       COALESCE(artist_id,''), COALESCE(album,''), COALESCE(album_id,''),
       COALESCE(duration,''), COALESCE(is_explicit,0)
FROM queue_track ORDER BY position ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t session.Track
		var explicit int
		if err := rows.Scan(
			&t.VideoID, &t.Title, &t.Artist, &t.ThumbnailURL,
			&t.ArtistID, &t.Album, &t.AlbumID, &t.Duration, &explicit,
		); err != nil {
			return nil, err
		}
		t.IsExplicit = explicit != 0
		snap.Queue = append(snap.Queue, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	navRows, err := db.sql.Query(`
SELECT kind, entity_id, title FROM nav_stack ORDER BY position ASC`)
	if err != nil {
		return nil, err
	}
	defer navRows.Close()
	for navRows.Next() {
		var n session.NavItem
		if err := navRows.Scan(&n.Kind, &n.ID, &n.Title); err != nil {
			return nil, err
		}
		snap.Nav = append(snap.Nav, n)
	}
	return &snap, navRows.Err()
}

// SaveSession replaces the persisted session, queue, and nav stack.
func (db *DB) SaveSession(snap session.Snapshot) error {
	if db == nil {
		return nil
	}
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.sql == nil {
		return fmt.Errorf("library: database closed")
	}

	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	hidden, showSearch, muted, wasPlaying, nowPlaying, normalize, crossfade := 0, 0, 0, 0, 0, 0, 0
	if snap.QueuePanelHidden {
		hidden = 1
	}
	if snap.ShowSearch {
		showSearch = 1
	}
	if snap.Muted {
		muted = 1
	}
	if snap.WasPlaying {
		wasPlaying = 1
	}
	if snap.NowPlayingOpen {
		nowPlaying = 1
	}
	if snap.Normalize {
		normalize = 1
	}
	if snap.Crossfade {
		crossfade = 1
	}
	crossfadeSec := session.ClampCrossfadeSec(snap.CrossfadeSec)
	vol := snap.Volume
	if vol < 0 {
		vol = 0
	}
	if vol > 100 {
		vol = 100
	}

	if _, err := tx.Exec(`
INSERT INTO session (
  id, active_menu, queue_panel_hidden, search_filter, last_search_query,
  active_carousel, home_card_cursor, track_cursor, list_cursor, queue_cursor,
  play_pos, queue_index, show_search, volume, muted,
  play_duration, was_playing, now_playing_open, normalize,
  crossfade, crossfade_sec, updated_at
) VALUES (
  1, ?, ?, ?, ?,
  ?, ?, ?, ?, ?,
  ?, ?, ?, ?, ?,
  ?, ?, ?, ?,
  ?, ?, datetime('now')
)
ON CONFLICT(id) DO UPDATE SET
  active_menu=excluded.active_menu,
  queue_panel_hidden=excluded.queue_panel_hidden,
  search_filter=excluded.search_filter,
  last_search_query=excluded.last_search_query,
  active_carousel=excluded.active_carousel,
  home_card_cursor=excluded.home_card_cursor,
  track_cursor=excluded.track_cursor,
  list_cursor=excluded.list_cursor,
  queue_cursor=excluded.queue_cursor,
  play_pos=excluded.play_pos,
  queue_index=excluded.queue_index,
  show_search=excluded.show_search,
  volume=excluded.volume,
  muted=excluded.muted,
  play_duration=excluded.play_duration,
  was_playing=excluded.was_playing,
  now_playing_open=excluded.now_playing_open,
  normalize=excluded.normalize,
  crossfade=excluded.crossfade,
  crossfade_sec=excluded.crossfade_sec,
  updated_at=datetime('now')
`, snap.ActiveMenu, hidden, snap.SearchFilter, snap.LastSearchQuery,
		snap.ActiveCarousel, snap.HomeCardCursor, snap.TrackCursor, snap.ListCursor, snap.QueueCursor,
		snap.PlayPos, snap.QueueIndex, showSearch, vol, muted,
		snap.PlayDuration, wasPlaying, nowPlaying, normalize,
		crossfade, crossfadeSec); err != nil {
		return err
	}

	if _, err := tx.Exec(`DELETE FROM queue_track`); err != nil {
		return err
	}
	for i, t := range snap.Queue {
		explicit := 0
		if t.IsExplicit {
			explicit = 1
		}
		if _, err := tx.Exec(`
INSERT INTO queue_track(position, video_id, title, artist, thumbnail_url,
  artist_id, album, album_id, duration, is_explicit)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			i, t.VideoID, t.Title, t.Artist, t.ThumbnailURL,
			t.ArtistID, t.Album, t.AlbumID, t.Duration, explicit); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(`DELETE FROM nav_stack`); err != nil {
		return err
	}
	for i, n := range snap.Nav {
		if _, err := tx.Exec(`
INSERT INTO nav_stack(position, kind, entity_id, title)
VALUES (?, ?, ?, ?)`, i, n.Kind, n.ID, n.Title); err != nil {
			return err
		}
	}

	return tx.Commit()
}
