package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite database connection
type DB struct {
	conn *sql.DB
}

// sqliteFmt is the datetime format SQLite can parse with strftime
const sqliteFmt = "2006-01-02 15:04:05"

// CallQualityRow represents a row in the call_quality table
type CallQualityRow struct {
	ID              int64   `json:"id"`
	Channel         string  `json:"channel"`
	UniqueID        string  `json:"uniqueid"`
	Caller          string  `json:"caller"`
	Callee          string  `json:"callee"`
	LinkedChannel   string  `json:"linked_channel"`
	Codec           string  `json:"codec"`
	DurationSeconds int     `json:"duration_seconds"`
	RxMES           float64 `json:"rxmes"`
	TxMES           float64 `json:"txmes"`
	RxJitter        float64 `json:"rxjitter"`
	TxJitter        float64 `json:"txjitter"`
	RxPLoss         int     `json:"rxploss"`
	TxPLoss         int     `json:"txploss"`
	RTT             float64 `json:"rtt"`
	MaxRTT          float64 `json:"maxrtt"`
	MinRTT          float64 `json:"minrtt"`
	TxCount         int     `json:"txcount"`
	RxCount         int     `json:"rxcount"`
	CreatedAt       string  `json:"created_at"`
	EndedAt         string  `json:"ended_at"`
}

// SecurityEventRow represents a row in the security_events table
type SecurityEventRow struct {
	ID            int64  `json:"id"`
	EventType     string `json:"event_type"`
	AccountID     string `json:"account_id"`
	RemoteAddress string `json:"remote_address"`
	Service       string `json:"service"`
	Timestamp     string `json:"timestamp"`
}

// EndpointChangeRow represents a row in the endpoint_changes table
type EndpointChangeRow struct {
	ID        int64  `json:"id"`
	Endpoint  string `json:"endpoint"`
	OldState  string `json:"old_state"`
	NewState  string `json:"new_state"`
	Timestamp string `json:"timestamp"`
}

// HourlyStat represents aggregated hourly call stats
type HourlyStat struct {
	Hour       int     `json:"hour"`
	TotalCalls int     `json:"total_calls"`
	AvgMOS     float64 `json:"avg_mos"`
	BadCalls   int     `json:"bad_calls"`
}

// DailyStat represents aggregated daily call stats
type DailyStat struct {
	Date       string  `json:"date"`
	TotalCalls int     `json:"total_calls"`
	AvgMOS     float64 `json:"avg_mos"`
	BadCalls   int     `json:"bad_calls"`
}

// CallStats represents aggregated call quality stats
type CallStats struct {
	TotalCalls  int     `json:"total_calls"`
	AvgMOS      float64 `json:"avg_mos"`
	CallsBelow3 int     `json:"calls_below_3"`
}

// Page represents a paginated result
type Page[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
	Page  int `json:"page"`
	Pages int `json:"pages"`
}

const schema = `
CREATE TABLE IF NOT EXISTS call_quality (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  channel TEXT NOT NULL,
  uniqueid TEXT NOT NULL,
  caller TEXT,
  callee TEXT,
  linked_channel TEXT,
  codec TEXT,
  duration_seconds INTEGER,
  rxmes REAL, txmes REAL,
  rxjitter REAL, txjitter REAL,
  rxploss INTEGER, txploss INTEGER,
  rtt REAL, maxrtt REAL, minrtt REAL,
  txcount INTEGER, rxcount INTEGER,
  created_at DATETIME NOT NULL,
  ended_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS security_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  event_type TEXT NOT NULL,
  account_id TEXT,
  remote_address TEXT,
  service TEXT,
  timestamp DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS endpoint_changes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  endpoint TEXT NOT NULL,
  old_state TEXT,
  new_state TEXT NOT NULL,
  timestamp DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_cq_ended ON call_quality(ended_at);
CREATE INDEX IF NOT EXISTS idx_se_timestamp ON security_events(timestamp);
CREATE INDEX IF NOT EXISTS idx_ec_timestamp ON endpoint_changes(timestamp);
CREATE INDEX IF NOT EXISTS idx_ec_endpoint ON endpoint_changes(endpoint);
`

// Open creates or opens the SQLite database at the given path
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("db open: %w", err)
	}

	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
	} {
		if _, err := conn.Exec(pragma); err != nil {
			conn.Close()
			return nil, fmt.Errorf("db pragma %q: %w", pragma, err)
		}
	}

	if _, err := conn.Exec(schema); err != nil {
		conn.Close()
		return nil, fmt.Errorf("db migrate: %w", err)
	}

	log.Printf("db: opened %s", path)
	return &DB{conn: conn}, nil
}

// Close closes the database connection
func (d *DB) Close() error {
	return d.conn.Close()
}

// Size returns the database file size in bytes
func (d *DB) Size() (int64, error) {
	var pageCount, pageSize int64
	if err := d.conn.QueryRow("PRAGMA page_count").Scan(&pageCount); err != nil {
		return 0, err
	}
	if err := d.conn.QueryRow("PRAGMA page_size").Scan(&pageSize); err != nil {
		return 0, err
	}
	return pageCount * pageSize, nil
}

// fmtTime formats time.Time for SQLite storage (always UTC)
func fmtTime(t time.Time) string {
	return t.UTC().Format(sqliteFmt)
}

// InsertCallQuality inserts a completed call's quality data
func (d *DB) InsertCallQuality(ch, uniqueid, caller, callee, linked, codec string,
	duration int, rxmes, txmes, rxjitter, txjitter float64, rxploss, txploss int,
	rtt, maxrtt, minrtt float64, txcount, rxcount int, createdAt, endedAt time.Time) error {
	_, err := d.conn.Exec(`INSERT INTO call_quality
		(channel, uniqueid, caller, callee, linked_channel, codec, duration_seconds,
		 rxmes, txmes, rxjitter, txjitter, rxploss, txploss,
		 rtt, maxrtt, minrtt, txcount, rxcount, created_at, ended_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		ch, uniqueid, caller, callee, linked, codec, duration,
		rxmes, txmes, rxjitter, txjitter, rxploss, txploss,
		rtt, maxrtt, minrtt, txcount, rxcount,
		fmtTime(createdAt), fmtTime(endedAt))
	return err
}

// InsertSecurityEvent inserts a security event
func (d *DB) InsertSecurityEvent(eventType, accountID, remoteAddr, service string, ts time.Time) error {
	_, err := d.conn.Exec(`INSERT INTO security_events
		(event_type, account_id, remote_address, service, timestamp)
		VALUES (?,?,?,?,?)`,
		eventType, accountID, remoteAddr, service, fmtTime(ts))
	return err
}

// InsertEndpointChange inserts an endpoint state change
func (d *DB) InsertEndpointChange(endpoint, oldState, newState string, ts time.Time) error {
	_, err := d.conn.Exec(`INSERT INTO endpoint_changes
		(endpoint, old_state, new_state, timestamp)
		VALUES (?,?,?,?)`,
		endpoint, oldState, newState, fmtTime(ts))
	return err
}

// Purge deletes records older than the given number of days
func (d *DB) Purge(days int) (int64, error) {
	cutoff := fmtTime(time.Now().AddDate(0, 0, -days))
	var total int64

	for _, table := range []struct {
		name string
		col  string
	}{
		{"call_quality", "ended_at"},
		{"security_events", "timestamp"},
		{"endpoint_changes", "timestamp"},
	} {
		res, err := d.conn.Exec(
			fmt.Sprintf("DELETE FROM %s WHERE %s < ?", table.name, table.col),
			cutoff)
		if err != nil {
			return total, fmt.Errorf("purge %s: %w", table.name, err)
		}
		n, _ := res.RowsAffected()
		total += n
	}
	return total, nil
}

// QueryCalls returns paginated call quality history with optional filters
func (d *DB) QueryCalls(from, to time.Time, minMOS float64, page, perPage int) (*Page[CallQualityRow], error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 50
	}

	fromS, toS := fmtTime(from), fmtTime(to)
	where := "WHERE ended_at BETWEEN ? AND ?"
	args := []any{fromS, toS}
	if minMOS > 0 {
		where += " AND (rxmes >= ? OR txmes >= ?)"
		args = append(args, minMOS, minMOS)
	}

	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	err := d.conn.QueryRow("SELECT COUNT(*) FROM call_quality "+where, countArgs...).Scan(&total)
	if err != nil {
		return nil, err
	}

	pages := (total + perPage - 1) / perPage
	if pages < 1 {
		pages = 1
	}

	offset := (page - 1) * perPage
	args = append(args, perPage, offset)

	rows, err := d.conn.Query(
		"SELECT id, channel, uniqueid, caller, callee, linked_channel, codec, duration_seconds, "+
			"rxmes, txmes, rxjitter, txjitter, rxploss, txploss, rtt, maxrtt, minrtt, txcount, rxcount, "+
			"created_at, ended_at FROM call_quality "+where+" ORDER BY ended_at DESC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]CallQualityRow, 0)
	for rows.Next() {
		var r CallQualityRow
		if err := rows.Scan(&r.ID, &r.Channel, &r.UniqueID, &r.Caller, &r.Callee,
			&r.LinkedChannel, &r.Codec, &r.DurationSeconds,
			&r.RxMES, &r.TxMES, &r.RxJitter, &r.TxJitter, &r.RxPLoss, &r.TxPLoss,
			&r.RTT, &r.MaxRTT, &r.MinRTT, &r.TxCount, &r.RxCount,
			&r.CreatedAt, &r.EndedAt); err != nil {
			return nil, err
		}
		items = append(items, r)
	}

	return &Page[CallQualityRow]{Items: items, Total: total, Page: page, Pages: pages}, nil
}

// QueryCallStats returns aggregated call stats for a time range
func (d *DB) QueryCallStats(from, to time.Time) (*CallStats, error) {
	var stats CallStats
	err := d.conn.QueryRow(`
		SELECT COUNT(*),
			COALESCE(AVG((rxmes + txmes) / 2.0), 0),
			COALESCE(SUM(CASE WHEN (rxmes + txmes) / 2.0 < 3.0 THEN 1 ELSE 0 END), 0)
		FROM call_quality WHERE ended_at BETWEEN ? AND ?`,
		fmtTime(from), fmtTime(to)).Scan(&stats.TotalCalls, &stats.AvgMOS, &stats.CallsBelow3)
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

// QueryCallsHourly returns per-hour aggregated call stats for a time range
func (d *DB) QueryCallsHourly(from, to time.Time) ([]HourlyStat, error) {
	rows, err := d.conn.Query(`
		SELECT
			CAST(strftime('%H', ended_at) AS INTEGER) AS hour,
			COUNT(*) AS total_calls,
			COALESCE(AVG((rxmes + txmes) / 2.0), 0) AS avg_mos,
			COALESCE(SUM(CASE WHEN (rxmes + txmes) / 2.0 < 3.0 THEN 1 ELSE 0 END), 0) AS bad_calls
		FROM call_quality
		WHERE ended_at BETWEEN ? AND ?
		GROUP BY hour
		ORDER BY hour`, fmtTime(from), fmtTime(to))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dataMap := make(map[int]HourlyStat)
	for rows.Next() {
		var s HourlyStat
		if err := rows.Scan(&s.Hour, &s.TotalCalls, &s.AvgMOS, &s.BadCalls); err != nil {
			return nil, err
		}
		dataMap[s.Hour] = s
	}

	result := make([]HourlyStat, 24)
	for h := 0; h < 24; h++ {
		if s, ok := dataMap[h]; ok {
			result[h] = s
		} else {
			result[h] = HourlyStat{Hour: h}
		}
	}
	return result, nil
}

// QueryCallsDaily returns per-day aggregated call stats for a time range
func (d *DB) QueryCallsDaily(from, to time.Time) ([]DailyStat, error) {
	rows, err := d.conn.Query(`
		SELECT
			date(ended_at) AS day,
			COUNT(*) AS total_calls,
			COALESCE(AVG((rxmes + txmes) / 2.0), 0) AS avg_mos,
			COALESCE(SUM(CASE WHEN (rxmes + txmes) / 2.0 < 3.0 THEN 1 ELSE 0 END), 0) AS bad_calls
		FROM call_quality
		WHERE ended_at BETWEEN ? AND ?
		GROUP BY day
		ORDER BY day`, fmtTime(from), fmtTime(to))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []DailyStat
	for rows.Next() {
		var s DailyStat
		if err := rows.Scan(&s.Date, &s.TotalCalls, &s.AvgMOS, &s.BadCalls); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, nil
}

// QuerySecurityEvents returns paginated security event history with optional filters
func (d *DB) QuerySecurityEvents(from, to time.Time, eventType string, page, perPage int) (*Page[SecurityEventRow], error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 50
	}

	fromS, toS := fmtTime(from), fmtTime(to)
	where := "WHERE timestamp BETWEEN ? AND ?"
	args := []any{fromS, toS}
	if eventType != "" {
		where += " AND event_type = ?"
		args = append(args, eventType)
	}

	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	err := d.conn.QueryRow("SELECT COUNT(*) FROM security_events "+where, countArgs...).Scan(&total)
	if err != nil {
		return nil, err
	}

	pages := (total + perPage - 1) / perPage
	if pages < 1 {
		pages = 1
	}

	offset := (page - 1) * perPage
	args = append(args, perPage, offset)

	rows, err := d.conn.Query(
		"SELECT id, event_type, account_id, remote_address, service, timestamp "+
			"FROM security_events "+where+" ORDER BY timestamp DESC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]SecurityEventRow, 0)
	for rows.Next() {
		var r SecurityEventRow
		if err := rows.Scan(&r.ID, &r.EventType, &r.AccountID, &r.RemoteAddress, &r.Service, &r.Timestamp); err != nil {
			return nil, err
		}
		items = append(items, r)
	}

	return &Page[SecurityEventRow]{Items: items, Total: total, Page: page, Pages: pages}, nil
}

// QueryEndpointChanges returns paginated endpoint change history with optional filters
func (d *DB) QueryEndpointChanges(from, to time.Time, endpoint string, page, perPage int) (*Page[EndpointChangeRow], error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 50
	}

	fromS, toS := fmtTime(from), fmtTime(to)
	where := "WHERE timestamp BETWEEN ? AND ?"
	args := []any{fromS, toS}
	if endpoint != "" {
		where += " AND endpoint = ?"
		args = append(args, endpoint)
	}

	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	err := d.conn.QueryRow("SELECT COUNT(*) FROM endpoint_changes "+where, countArgs...).Scan(&total)
	if err != nil {
		return nil, err
	}

	pages := (total + perPage - 1) / perPage
	if pages < 1 {
		pages = 1
	}

	offset := (page - 1) * perPage
	args = append(args, perPage, offset)

	rows, err := d.conn.Query(
		"SELECT id, endpoint, old_state, new_state, timestamp "+
			"FROM endpoint_changes "+where+" ORDER BY timestamp DESC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]EndpointChangeRow, 0)
	for rows.Next() {
		var r EndpointChangeRow
		if err := rows.Scan(&r.ID, &r.Endpoint, &r.OldState, &r.NewState, &r.Timestamp); err != nil {
			return nil, err
		}
		items = append(items, r)
	}

	return &Page[EndpointChangeRow]{Items: items, Total: total, Page: page, Pages: pages}, nil
}

// GetSetting returns the value for a settings key, or empty string if not found
func (d *DB) GetSetting(key string) (string, error) {
	var value string
	err := d.conn.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

// SetSetting upserts a single settings key-value pair
func (d *DB) SetSetting(key, value string) error {
	_, err := d.conn.Exec(
		"INSERT OR REPLACE INTO settings (key, value, updated_at) VALUES (?, ?, ?)",
		key, value, fmtTime(time.Now()))
	return err
}

// GetAllSettings returns all settings as a key-value map
func (d *DB) GetAllSettings() (map[string]string, error) {
	rows, err := d.conn.Query("SELECT key, value FROM settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		settings[k] = v
	}
	return settings, nil
}

// SetSettings upserts multiple settings in a single transaction
func (d *DB) SetSettings(settings map[string]string) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("INSERT OR REPLACE INTO settings (key, value, updated_at) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := fmtTime(time.Now())
	for k, v := range settings {
		if _, err := stmt.Exec(k, v, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}
