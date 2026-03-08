package model

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := InitDB(t.TempDir())
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

func TestInitDBCreatesSchema(t *testing.T) {
	db := openTestDB(t)

	for _, table := range []string{"events", "settings"} {
		var name string
		if err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name); err != nil {
			t.Fatalf("schema missing table %s: %v", table, err)
		}
		if name != table {
			t.Fatalf("table name = %q, want %q", name, table)
		}
	}

	dbPath := filepath.Join(t.TempDir(), "clawpanel.db")
	if dbPath == "" {
		t.Fatal("unexpected empty db path sentinel")
	}
}

func TestAddEventAndGetEventsFiltersAndPagination(t *testing.T) {
	db := openTestDB(t)

	events := []*Event{
		{Time: 100, Source: "qq", Type: "info", Summary: "alpha", Detail: "first"},
		{Time: 300, Source: "qq", Type: "warn", Summary: "beta", Detail: "second detail"},
		{Time: 200, Source: "wechat", Type: "info", Summary: "gamma", Detail: "third"},
	}
	for _, event := range events {
		if _, err := AddEvent(db, event); err != nil {
			t.Fatalf("AddEvent() error = %v", err)
		}
	}

	filtered, total, err := GetEvents(db, 10, 0, "qq", "")
	if err != nil {
		t.Fatalf("GetEvents() error = %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	if len(filtered) != 2 || filtered[0].Summary != "beta" || filtered[1].Summary != "alpha" {
		t.Fatalf("unexpected filtered events: %+v", filtered)
	}

	searched, total, err := GetEvents(db, 10, 0, "", "second")
	if err != nil {
		t.Fatalf("GetEvents(search) error = %v", err)
	}
	if total != 1 || len(searched) != 1 || searched[0].Summary != "beta" {
		t.Fatalf("unexpected search result: total=%d events=%+v", total, searched)
	}

	paged, total, err := GetEvents(db, 1, 1, "", "")
	if err != nil {
		t.Fatalf("GetEvents(page) error = %v", err)
	}
	if total != 3 {
		t.Fatalf("page total = %d, want 3", total)
	}
	if len(paged) != 1 || paged[0].Summary != "gamma" {
		t.Fatalf("unexpected paged result: %+v", paged)
	}
}

func TestAddEventDefaultsTimeAndSettingsRoundTrip(t *testing.T) {
	db := openTestDB(t)

	event := &Event{Source: "system", Type: "info", Summary: "boot"}
	id, err := AddEvent(db, event)
	if err != nil {
		t.Fatalf("AddEvent() error = %v", err)
	}
	if id <= 0 {
		t.Fatalf("event id = %d, want > 0", id)
	}
	if event.Time == 0 {
		t.Fatal("AddEvent() should populate missing timestamp")
	}

	if err := SetSetting(db, "theme", "dark"); err != nil {
		t.Fatalf("SetSetting(insert) error = %v", err)
	}
	if err := SetSetting(db, "theme", "light"); err != nil {
		t.Fatalf("SetSetting(update) error = %v", err)
	}
	value, err := GetSetting(db, "theme")
	if err != nil {
		t.Fatalf("GetSetting() error = %v", err)
	}
	if value != "light" {
		t.Fatalf("setting value = %q, want light", value)
	}

	if _, err := GetSetting(db, "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetSetting(missing) error = %v, want sql.ErrNoRows", err)
	}
}

func TestClearEventsRemovesAllRows(t *testing.T) {
	db := openTestDB(t)

	if _, err := AddEvent(db, &Event{Time: 1, Source: "system", Type: "info", Summary: "one"}); err != nil {
		t.Fatalf("AddEvent() error = %v", err)
	}
	if err := ClearEvents(db); err != nil {
		t.Fatalf("ClearEvents() error = %v", err)
	}

	events, total, err := GetEvents(db, 10, 0, "", "")
	if err != nil {
		t.Fatalf("GetEvents() error = %v", err)
	}
	if total != 0 || len(events) != 0 {
		t.Fatalf("expected no events after clear, total=%d events=%+v", total, events)
	}
}
