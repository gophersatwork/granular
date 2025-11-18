module github.com/gophersatwork/granular/poc/test-caching

go 1.25

require github.com/gophersatwork/granular v0.0.0

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/spf13/afero v1.11.0 // indirect
	golang.org/x/text v0.14.0 // indirect
)

replace github.com/gophersatwork/granular => ../..
