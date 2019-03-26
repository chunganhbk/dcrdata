module github.com/decred/dcrdata/middleware

go 1.12

require (
	github.com/decred/dcrd/chaincfg v1.3.0
	github.com/decred/dcrd/chaincfg/chainhash v1.0.1
	github.com/decred/dcrd/dcrjson/v2 v2.0.0
	github.com/decred/dcrd/dcrutil v1.2.1-0.20190118223730-3a5281156b73
	github.com/decred/dcrdata/api/types v1.0.4
	github.com/decred/dcrdata/db/dbtypes v1.0.0 // indirect
	github.com/decred/slog v1.0.0
	github.com/go-chi/chi v4.0.3-0.20190316151245-d08916613452+incompatible
	github.com/go-chi/docgen v1.0.5
)