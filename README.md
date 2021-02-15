# I Migrate

Interface-driven migrations for Go.

I Migrate is an interface-driven approach to managing database migrations. It was created out of a need for a migration tool that can be used in a Go project that doesn't conform to database/sql, and can execute migrations embedded in a single binary.

This project aims to fulfill the following needs:

1. Migrations written in pure SQL, stored in flat files. 
1. CLI runner for up, down, redo, rollback, etc. 
1. CLI template generator for timestamp-prefixed migrations.
1. http.FileSystem support, allowing migration files to be embedded in a Go Binary.
1. Database driver agnostic via a sql Exec interface.
1. No config files. (But config in code) 

[Read the docs](https://pkg.go.dev/github.com/sandro/imigrate)

## Motivation

I didn't want to write this code, I really didn't. Migrations should be commodity code, and there are already dozens of libraries available. Further, after finding [shmig](https://github.com/mbucc/shmig), I had written off needing a migration tool ever again, but that was before I needed to ship an actual migration to prod.

These days, prod is different. It's docker container running a single binary. And my image doesn't need a migrations folder, nor does it need a shell, or a SQLite client, all of which shmig require. So what to do? Should I create a new docker container that only has shmig, SQLite, and a migrations folder? That seems terribly redundant and a waste of space and deployment overhead. Too many moving parts!

Great, so instead I decided to use the most popular migration tool with embedded sql migration support. But that's when I hit the interface problem. You see, one of my projects uses a non-standard SQLite driver that doesn't conform to database/sql, while another project uses the standard driver. I checked a few libraries and though some looked very promising ([gloat](https://github.com/gsamokovarov/gloat)) by offering interfaces for migrations, they eventually relied on sql.Rows or another database struct. And that's when I decided to use my Saturday to create this simple library.

But with increased flexibility comes an increased cost. This tool requires configuration through code. Remember, it's interface-driven which means the tool provides an interface while you provide the logic. Not to worry though, it's a fairly straight forward interface: allow the tool to `Exec` sql and `GetVersions` (return an array of ids) and it'll manage your migrations for you! A wholesome give and take.

## Usage

Unfortunately, some assembly required.

I Migrate has a command line interface, but you have to provide the glue to make it work. I know it's bummer when code doesn't just work out of the box, but if that's what you needed, you wouldn't be here. On the upside, you can name the migration binary whatever you want, or skip it all together.

```go
// MyDB conforms to the Executor interface by defining Exec and GetVersions
type MyDB struct {
  *sql.DB
}

func (o MyDB) GetVersions(query string, args ...interface{}) (versions []int64, err error) {
  rows, err := o.Query(query, args...)
  if err != nil {
    return
  }
  defer rows.Close()
  for rows.Next() {
    var version int64
    if err = rows.Scan(&version); err != nil {
      return
    }
    versions = append(versions, version)
  }
  err = rows.Err()
  return
}

db, err := sql.Open("sqlite3", "db.sqlite3")
if err != nil {
  log.Panic(err)
}
defer db.Close()

myDB := MyDB{DB: db}
fs := http.Dir("")
migrator := migrations.NewMigrator(myDB, fs)
migrator.CLI(migrator)
```

Example CLI usage for a tool name "migrate"

```sh
migrate create create_users_table

migrate up
migrate up --steps 1

migrate down
migrate down --steps 1

migrate redo
migrate redo --steps 2

migrate rollback
migrate rollback --steps 3
```
