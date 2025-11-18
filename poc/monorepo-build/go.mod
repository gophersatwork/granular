module monorepo-build

go 1.25

require github.com/gophersatwork/granular v0.0.0

replace github.com/gophersatwork/granular => ../..

replace monorepo-build/shared/models => ./shared/models

replace monorepo-build/shared/utils => ./shared/utils

replace monorepo-build/services/api => ./services/api

replace monorepo-build/services/worker => ./services/worker

replace monorepo-build/services/admin => ./services/admin
