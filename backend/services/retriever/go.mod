module github.com/strata/retriever

go 1.25.10

require (
	github.com/go-chi/chi/v5 v5.1.0
	github.com/rs/zerolog v1.33.0
	github.com/strata/shared v0.0.0
)

require (
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	golang.org/x/sys v0.12.0 // indirect
)

replace github.com/strata/shared => ../shared
