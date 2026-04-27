module monorepo-build

go 1.26

require (
	github.com/gophersatwork/granular v0.0.0
	github.com/spf13/afero v1.11.0
)

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/klauspost/compress v1.18.3 // indirect
	golang.org/x/text v0.14.0 // indirect
)

replace github.com/gophersatwork/granular => ../..

replace monorepo-build/shared/models => ./shared/models

replace monorepo-build/shared/utils => ./shared/utils

replace monorepo-build/services/api => ./services/api

replace monorepo-build/services/worker => ./services/worker

replace monorepo-build/services/admin => ./services/admin
