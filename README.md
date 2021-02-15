# I Migrate

Interface driven migrations for Go.

Imigrate is an interface driven approach to managing database migrations. It was created out of my need for a migration library that can be included into my Go project which uses a custom sql library that doesn't conform to database/sql, and can be deployed in a single binary.

This project aims to fulfill the following needs:

1. Migrations written in pure SQL, stored in files. 
1. CLI runner for up, down, redo, etc. 
1. CLI migration template generator for timestamp-prefixed migrations
1. http.FileSystem support, allowing migrations to be packaged and run in a Go Binary.
1. Database driver agnostic via a simple Executor interface.
1. No config files. (But config in code) 

## Motivation

I didn't want to write this code, I really didn't. Migrations should be commodity code, and there are already dozens of libraries available. Further, after finding shmig, I had written off ever needing a migration tool again, but that was before I needed to ship an actual migration to prod.

These days, prod is different. It's docker container running a single binary. My image doesn't need a migrations folder, nor does it need a shell, both of which shmig require. So what to do? Create a new docker container that only has shmig, SQLite, and a migrations folder? That seems terribly redundant and a waste of space and deployment overhead. Too many moving parts!

Great, so then I decided to use the most popular migration tool that supports embedded sql migrations. But that's when I hit the interface problem. You see, one of my projects uses a non-standard SQLite driver that doesn't conform to database/sql, while another project uses the standard driver. I checked a few libraries and though some looked very promising by offering interfaces for migrations, they eventually relied on sql.Rows or another database struct. And that's when I decided to use my Saturday to create this simple library.

But increased flexibility comes at a price. This tool requires configuration through code. It's interface-driven which means the tool provides the interface but you have to fulfill the contract. Not to worry though, it's a fairly straight forward contract: allow the tool to Exec sql and Select an array of ids and it'll manage your migrations for you! A wholesome give and take.


