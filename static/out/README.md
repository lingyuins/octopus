This placeholder ensures `static/out/` exists in git so that
`//go:embed out out/**` in `static.go` compiles without a build step.

The release pipeline replaces the contents with the real frontend build output.
