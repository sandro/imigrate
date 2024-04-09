package imigrate

import (
	"bufio"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var Logger = log.Default()
var DiscardLogger = log.New(io.Discard, "", log.LstdFlags)

// Executor is the interface to executing SQL
//
// Exec executes a SQL query returning a sql.Result. Use this for mutation
// queries like CREATE, INSERT, UPDATE, DELETE, etc.
//
// GetVersions returns the timestamped versions that have been migrated thus
// far. Timestamps are stored in Unix time, that is seconds since epoch, stored
// in an int64.
type Executor interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	GetVersions(query string, args ...interface{}) ([]int64, error)
}

// Migrator is the interface for running migrations.
//
// Create is used to create a new migration file. The file should be prefixed
// with a unix timestamp and stored in the migrations directory.
//
// Up runs the UP migration for every migration file.
//
// Down runs the DOWN migration for every migration file.
//
// Redo runs the DOWN then UP migration for the most recently created
// migration.
//
// Rollback runs the DOWN migration for the most recenlty created migration.
//
// Status prints out which migrations have been run thus far.
type Migrator interface {
	Create(string)
	Up(int, int64)
	Down(int, int64)
	Redo(int, int64)
	Rollback(int)
	Status()
}

// Migration represents a single migration file
type Migration struct {
	Version  int64
	Time     time.Time
	FileInfo os.FileInfo
	Up       string
	Dn       string
}

// Valid reads and stores the UP and DOWN SQL queries, and returns true if both
// are found.
func (o *Migration) Valid(file http.File, upKey, dnKey *regexp.Regexp) (valid bool) {
	upStart := false
	dnStart := false
	reader := bufio.NewReader(file)
	for {
		l, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				valid = upStart && dnStart
				break
			}
			Logger.Println("read string error", err)
			break
		}
		if !upStart && upKey.MatchString(l) {
			upStart = true
			continue
		}
		if upStart && !dnStart && dnKey.MatchString(l) {
			dnStart = true
			continue
		}
		if upStart && !dnStart {
			o.Up += l
		}
		if dnStart {
			o.Dn += l
		}
	}
	return valid
}

// IMigrator is the default migrator that satisfies the Migrator interface.
type IMigrator struct {
	DB                Executor
	FS                http.FileSystem
	Dirname           string         // The directory where migrations are stored.
	UpKey             *regexp.Regexp // The Regexp to detecth the up migration SQL.
	DnKey             *regexp.Regexp // The Regexp to detecth the down migration SQL.
	TableName         string         // The table where migration info is stored.
	VersionColumn     string         // The version column in the migrations table.
	CreateTableSQL    string         // The SQL to create the migrations table.
	Migrations        []Migration
	FileVersionRegexp *regexp.Regexp // The Regexp to detect a migration file.
	TemplateUp        string         // The SQL to place in the UP section of a generated file.
	TemplateDn        string         // The SQL to place in the DOWN section of a generated file.
	setupDone         bool
}

// NewIMigrator returns a default migrator with the SQLite dialect.
func NewIMigrator(db Executor, fs http.FileSystem) *IMigrator {
	m := &IMigrator{
		DB:                db,
		FS:                fs,
		Dirname:           "migrations",
		UpKey:             regexp.MustCompile(`^\s*--.*UP`),
		DnKey:             regexp.MustCompile(`^\s*--.*DOWN`),
		TableName:         "shmig_version",
		VersionColumn:     "version",
		FileVersionRegexp: regexp.MustCompile(`^\d+`),
		TemplateUp: `
PRAGMA foreign_keys = ON;

BEGIN;
COMMIT;
`,
		TemplateDn: `
PRAGMA foreign_keys = OFF;

BEGIN;
COMMIT;`,
	}
	m.CreateTableSQL = fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
	%s integer primary key,
	migrated_at timestamp not null default (datetime(current_timestamp))
);
`, m.TableName, m.VersionColumn)
	return m
}

func (o IMigrator) createTable() {
	_, err := o.DB.Exec(o.CreateTableSQL)
	if err != nil {
		Logger.Panicln(err)
	}
}

func (o *IMigrator) getCompletedVersions() []int64 {
	versions, err := o.DB.GetVersions(fmt.Sprintf("select %s from %s order by %s", o.VersionColumn, o.TableName, o.VersionColumn))
	if err != nil {
		Logger.Panicln(err)
	}
	return versions
}

func (o *IMigrator) setup() {
	if o.setupDone {
		return
	}
	o.createTable()
	root, err := o.FS.Open(o.Dirname)
	if err != nil {
		Logger.Panicln("couldn't open", o.Dirname, err)
	}
	defer root.Close()
	finfos, err := root.Readdir(-1)
	if err != nil {
		Logger.Panicln("err during readdir", o.Dirname, err)
	}
	for _, info := range finfos {
		n := o.FileVersionRegexp.FindString(info.Name())
		nn, err := strconv.ParseInt(n, 10, 64)
		if err != nil {
			continue
		}
		f, err := o.FS.Open(path.Join(o.Dirname, info.Name()))
		if err != nil {
			Logger.Println("couldn't open file", o.Dirname, info.Name(), err)
			continue
		}
		migration := Migration{
			Version:  nn,
			Time:     time.Unix(nn, 0),
			FileInfo: info,
		}
		if migration.Valid(f, o.UpKey, o.DnKey) {
			o.Migrations = append(o.Migrations, migration)
		}
		f.Close()
		o.setupDone = true
	}
}

func (o IMigrator) migrated(m Migration) bool {
	for _, v := range o.getCompletedVersions() {
		if v == m.Version {
			return true
		}
	}
	return false
}

func getLastId(res sql.Result) int64 {
	id, err := res.LastInsertId()
	if err != nil {
		Logger.Panicln(err)
	}
	return id
}

// Up runs all migrations that have not been run.  If steps is greater than -1,
// it will run that many migrations in ascending order.  If version is greater
// than 0, it will migrate up that specific version.
func (o *IMigrator) Up(steps int, version int64) {
	o.setup()
	if version != 0 {
		o.upVersion(version)
		return
	}
	o.sortAscending()
	completed := 0
	for _, m := range o.Migrations {
		if completed == steps {
			break
		}
		if !o.migrated(m) {
			o.execUp(m)
			completed++
		}
	}
}

func (o IMigrator) execUp(m Migration) {
	res, err := o.DB.Exec(strings.TrimSpace(m.Up))
	if err != nil {
		Logger.Panicln("Migration err", m.Version, err)
	}
	Logger.Printf("Up completed %d %d\n", m.Version, getLastId(res))
	res, err = o.DB.Exec(fmt.Sprintf("INSERT INTO %s (%s) VALUES(?)", o.TableName, o.VersionColumn), m.Version)
	if err != nil {
		Logger.Panicln("could not complete UP migration", err)
	}
	Logger.Println("Migration table updated", getLastId(res))
}

func (o IMigrator) upVersion(version int64) {
	for _, m := range o.Migrations {
		if m.Version == version && !o.migrated(m) {
			o.execUp(m)
			break
		}
	}
}

// Down runs all migrations in descending order.
// If steps is greater than -1, it will step down that many migrations.
// If version is greater than 0, it will only migrate down that specific
// version.
func (o *IMigrator) Down(steps int, version int64) {
	o.setup()
	if version != 0 {
		o.downVersion(version)
		return
	}
	o.sortDescending()
	completed := 0
	for _, m := range o.Migrations {
		if completed == steps {
			break
		}
		if o.migrated(m) {
			o.execDown(m)
			completed++
		}
	}
}

func (o IMigrator) execDown(m Migration) {
	res, err := o.DB.Exec(m.Dn)
	if err != nil {
		Logger.Panicln("Migration err", m.Version, err)
	}
	Logger.Printf("Down completed %d %d\n", m.Version, getLastId(res))
	res, err = o.DB.Exec(fmt.Sprintf("DELETE FROM %s WHERE %s = ?", o.TableName, o.VersionColumn), m.Version)
	if err != nil {
		Logger.Panicln("could not complete DOWN migration", err)
	}
	Logger.Println("Migration table updated", getLastId(res))
}

func (o IMigrator) downVersion(version int64) {
	for _, m := range o.Migrations {
		if m.Version == version && o.migrated(m) {
			o.execDown(m)
			break
		}
	}
}

// Redo runs Down, then Up
func (o *IMigrator) Redo(steps int, version int64) {
	o.Down(steps, version)
	o.Up(steps, version)
}

// Rollback runs the down SQL for the most recent migration.
// If steps is greater than 1, it will run that many migrations down.
func (o *IMigrator) Rollback(steps int) {
	o.Down(steps, 0)
}

// Status prints out which migrations have been run and which are pending.
func (o *IMigrator) Status() {
	Logger.Println("STATUS")
	o.setup()
	for _, v := range o.getCompletedVersions() {
		Logger.Println("Migration Completed", v)
	}
	o.pending()
}

func (o *IMigrator) sortAscending() {
	sort.Slice(o.Migrations, func(i, j int) bool { return o.Migrations[i].Version < o.Migrations[j].Version })
}
func (o *IMigrator) sortDescending() {
	sort.Slice(o.Migrations, func(i, j int) bool { return o.Migrations[i].Version > o.Migrations[j].Version })
}

func (o IMigrator) pending() {
	o.sortAscending()
	for _, m := range o.Migrations {
		if !o.migrated(m) {
			Logger.Println("Pending", m.Version)
		}
	}
}

// Create generates a new migration file in the Dirname directory.  The file is
// prefixed with the current time as a unix timestamp, followed by the provided
// name.  It will insert the provided TemplateUp and TemplateDn strings into
// the appropriate sections of the migration file.
func (o IMigrator) Create(name string) {
	err := os.MkdirAll(o.Dirname, os.ModeDir)
	if err != nil {
		Logger.Panicln(err)
	}
	now := time.Now()
	fname := fmt.Sprintf("%d-%s.sql", now.Unix(), name)
	path := filepath.Join(o.Dirname, fname)
	Logger.Println("Created", path)
	f, err := os.Create(path)
	if err != nil {
		Logger.Panicln(err)
	}
	defer f.Close()
	template := fmt.Sprintf(`
-- Migration:  %s
-- Created at: %s
-- ==== UP ====

%s

-- ==== DOWN ====

%s
`, name,
		now.Format("2006-01-02 15:04:05"),
		strings.TrimSpace(o.TemplateUp),
		strings.TrimSpace(o.TemplateDn),
	)
	f.WriteString(strings.TrimSpace(template))
}
