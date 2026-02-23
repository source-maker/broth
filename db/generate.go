// Package db contains database infrastructure including connection management,
// transaction support, and code generation directives.
//
// To generate type-safe models from your PostgreSQL schema, ensure your database
// is running and the PSQL_DSN environment variable is set, then run:
//
//	go generate ./db/...
package db

//go:generate go tool bobgen-psql -c ../config/bobgen.yaml
