module jfs-router

go 1.22.2

replace golang.org/x/sys => github.com/golang/sys v0.28.0

require (
	github.com/go-sql-driver/mysql v1.7.1
	github.com/hanwen/go-fuse/v2 v2.11.0
	github.com/redis/go-redis/v9 v9.7.3
)

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	golang.org/x/sys v0.28.0 // indirect
)
