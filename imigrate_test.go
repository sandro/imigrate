package imigrate

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/bvinc/go-sqlite-lite/sqlite3"
)

type DB struct {
	*sqlite3.Conn
}

func check(err error) {
	if err != nil {
		log.Panic(err)
	}
}

func NewDB(uri string) *DB {
	conn, err := sqlite3.Open(":memory:")
	check(err)
	return &DB{Conn: conn}
}

type DBResult struct {
	lastInsertID int64
	rowsAffected int64
	err          error
}

func (o DBResult) LastInsertId() (int64, error) {
	return o.lastInsertID, o.err
}

func (o DBResult) RowsAffected() (int64, error) {
	return o.rowsAffected, o.err
}

func (o DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	err := o.Conn.Exec(query, args...)
	res := DBResult{
		lastInsertID: o.LastInsertRowID(),
		rowsAffected: int64(o.Changes()),
		err:          err,
	}
	return res, err
}
func (o DB) Get(dst []interface{}, query string, args ...interface{}) (err error) {
	stmt, err := o.Conn.Prepare(query, args...)
	if err != nil {
		return
	}
	defer stmt.Close()
	var hasRow bool
	for {
		hasRow, err = stmt.Step()
		if !hasRow {
			break
		}
		if err != nil {
			return
		}
		err = stmt.Scan(dst...)
		if err != nil {
			return
		}
	}
	return
}
func (o DB) GetVersions(query string, args ...interface{}) (versions []int64, err error) {
	stmt, err := o.Conn.Prepare(query, args...)
	if err != nil {
		return
	}
	defer stmt.Close()
	var hasRow bool
	for {
		hasRow, err = stmt.Step()
		if !hasRow {
			break
		}
		if err != nil {
			return
		}
		var version int64
		err = stmt.Scan(&version)
		if err != nil {
			return
		}
		versions = append(versions, version)
	}
	return
}

type FakeFSFileInfo struct {
	name    string
	size    int64
	modtime time.Time
}

func (o FakeFSFileInfo) Name() string {
	return o.name
}
func (o FakeFSFileInfo) Size() int64 {
	return o.size
}
func (o FakeFSFileInfo) Mode() os.FileMode {
	return os.ModePerm
}
func (o FakeFSFileInfo) ModTime() time.Time {
	return o.modtime
}
func (o FakeFSFileInfo) IsDir() bool {
	return false
}
func (o FakeFSFileInfo) Sys() interface{} {
	return nil
}

// func (o FakeFSFile) Read(p []byte) (n int, err error) {
// }
// func (o FakeFSFile) Seek(offset int64, whence int) (int64, error) {
// 	return 0, nil
// }

type FakeFSFile struct {
	*strings.Reader
	Files    []*FakeFSFile
	FileInfo os.FileInfo
}

func NewFakeFSFile(name, content string) *FakeFSFile {
	return &FakeFSFile{
		Reader:   strings.NewReader(content),
		FileInfo: FakeFSFileInfo{name: name},
	}
}
func (o FakeFSFile) Close() error {
	return nil
}

func (o FakeFSFile) Readdir(count int) ([]os.FileInfo, error) {
	var finfos []os.FileInfo
	for _, f := range o.Files {
		finfos = append(finfos, f.FileInfo)
	}
	return finfos, nil
}

func (o FakeFSFile) Stat() (os.FileInfo, error) {
	return o.FileInfo, nil
}

type FakeFS struct {
	migrationDirectory string
	root               *FakeFSFile
}

func NewFakeFS(migrationDirectory string, rootFiles []*FakeFSFile) *FakeFS {
	root := NewFakeFSFile(migrationDirectory, "root")
	root.Files = rootFiles
	return &FakeFS{
		migrationDirectory: migrationDirectory,
		root:               root,
	}
}

func (o FakeFS) Open(name string) (http.File, error) {
	if name == o.migrationDirectory {
		return o.root, nil
	}
	for _, f := range o.root.Files {
		if path.Join(o.migrationDirectory, f.FileInfo.Name()) == name {
			f.Seek(0, 0)
			return f, nil
		}
	}
	return nil, fmt.Errorf(fmt.Sprintf("file %s not found", name))
}

var migrations = map[string]*FakeFSFile{
	"mig1": NewFakeFSFile("1111110001-mig1", `
-- ==== UP ====
create table foo (id integer primary key);
-- ==== DOWN ====
drop table foo;
`),
	"mig2": NewFakeFSFile("1111110002-mig2", `
-- ==== UP ====
create table bar (id integer primary key);
-- ==== DOWN ====
drop table bar;
`),
	"mig3": NewFakeFSFile("1111110003-mig3", `
-- ==== UP ====
drop table bar;
create table baz (id integer primary key);
-- ==== DOWN ====
create table bar (id integer primary key);
drop table baz;
`),
	"mig4": NewFakeFSFile("1111110004-mig4", `
-- ==== UP ====
create table bux (id integer primary key);
-- ==== DOWN ====
drop table bux;
`),
}

func TestIMigrateUpDown(t *testing.T) {
	db := NewDB(":memory:")
	defer db.Close()
	fs := NewFakeFS("migrations", []*FakeFSFile{migrations["mig1"]})
	mig := NewIMigrator(db, fs)

	// UP
	mig.Up(-1, 0)
	var tableName string

	check(db.Get([]interface{}{&tableName}, "select name from sqlite_master where name='foo'"))
	if tableName != "foo" {
		t.Fatalf("expected %s to equal foo", tableName)
	}

	tableName = ""
	check(db.Get([]interface{}{&tableName}, "select name from sqlite_master where name=?", mig.TableName))
	if tableName != mig.TableName {
		t.Fatalf("expected %s to equal %s", tableName, mig.TableName)
	}

	// DOWN
	tableName = ""
	mig.Down(-1, 0)
	check(db.Get([]interface{}{&tableName}, "select name from sqlite_master where name='foo'"))
	if tableName != "" {
		t.Fatalf("expected %s to equal \"\"", tableName)
	}

}

func TestIMigrateUp(t *testing.T) {
	db := NewDB(":memory:")
	defer db.Close()
	fs := NewFakeFS("migrations", []*FakeFSFile{migrations["mig1"], migrations["mig2"], migrations["mig3"], migrations["mig4"]})
	mig := NewIMigrator(db, fs)

	mig.Up(-1, 0)
	tableNames := []string{
		"foo", "baz", "bux", mig.TableName,
	}
	for _, t := range tableNames {
		var sqlTbl string
		check(db.Get([]interface{}{&sqlTbl}, "select name from sqlite_master where name=?", t))
		if sqlTbl != t {
			log.Fatalf("expected %s to exist, got %s", t, sqlTbl)
		}
	}
	versions := []int{1111110001, 1111110002, 1111110003, 1111110004}
	for _, v := range versions {
		var versionMigrated int
		sql := fmt.Sprintf("select %s from %s where %s=?", mig.VersionColumn, mig.TableName, mig.VersionColumn)
		check(db.Get([]interface{}{&versionMigrated}, sql, v))
		if v != versionMigrated {
			t.Fatalf("Expected version %d to exist, but got %d", v, versionMigrated)
		}
	}

	mig.Down(-1, 0)
	var count int
	check(db.Get([]interface{}{&count}, "select count(*) from sqlite_master where type='table'"))
	if count != 1 {
		log.Fatalf("expected one table to exist, not %d", count)
	}
	count = 1111
	check(db.Get([]interface{}{&count}, fmt.Sprintf("select count(*) from %s", mig.TableName)))
	if count != 0 {
		log.Fatalf("Expected 0 migrations to exist after Down, got %d", count)
	}
	var migrationTable string
	check(db.Get([]interface{}{&migrationTable}, "select name from sqlite_master where type='table'"))

	if migrationTable != mig.TableName {
		t.Fatalf("Expected table %s to exist, got %s", mig.TableName, migrationTable)
	}

	mig.Up(-1, 1111110004)
	check(db.Get([]interface{}{&migrationTable}, "select name from sqlite_master where name=?", "bux"))
	if migrationTable != "bux" {
		log.Fatalf("expected table bux to exist on Up with version, but got %s", migrationTable)
	}

	mig.Down(-1, 0)
	mig.Up(2, 0)
	tableNames = []string{
		"foo", "bar", mig.TableName,
	}
	for _, t := range tableNames {
		var sqlTbl string
		check(db.Get([]interface{}{&sqlTbl}, "select name from sqlite_master where name=?", t))
		if sqlTbl != t {
			log.Fatalf("expected %s to exist, got %s", t, sqlTbl)
		}
	}

}
func TestIMigrateDown(t *testing.T) {
}
func TestIMigrateRedo(t *testing.T) {
}
func TestIMigrateRollback(t *testing.T) {
}
func TestIMigrateCreate(t *testing.T) {
}
func TestIMigrateStatus(t *testing.T) {
}
