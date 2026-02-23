// Package tools documents external tool dependencies used for code generation
// and development workflows in the Broth framework.
//
// Since this project uses Go 1.24+, tool dependencies are managed via the
// `tool` directive in go.mod (see the bottom of go.mod). Tools are added with:
//
//	go get -tool github.com/stephenafamo/bob/gen/bobgen-psql@v0.42.0
//
// And run with:
//
//	go tool bobgen-psql -c config/bobgen.yaml
//
// Or via go generate:
//
//	go generate ./db/...
//
// For more information, see: https://go.dev/blog/tool-directive
package tools
