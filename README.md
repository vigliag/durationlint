# DurationLint

**NOTE**: this check is a work in progress, issues and suggestions are welcome 

DurationLint disallows usage of untyped literals and constants as time.Duration.

It helps prevent mistakes like:

```go
package main
import "net/http"

func mistakes() {
	// Programmer meant 5 * time.Seconds, but constant is automatically promoted
	// to time.Duration on use, resulting in a 5 nanoseconds value.
	// Explicitly cast to time.Duration() to silence the check.
	const timeout = 5
	
	// valid uses in go, flagged by this check
	_ = http.Client { Timeout: timeout }
	var duration time.Duration = timeout
	time.Sleep(timeout)
}
```

see `./testdata/src/p1/test.go` for more

## Usage

```bash
go install github.com/vigliag/durationlint/cmd/durationlint@latest
cd yourcode
durationlint ./...
```

## Thanks

This tool was really easy to write, thanks to the excellent go/analysis package and the following amazing guides:

- https://disaev.me/p/writing-useful-go-analysis-linter/
- https://arslan.io/2019/06/13/using-go-analysis-to-write-a-custom-linter/

Previous efforts that I could find (prior to the introduction of go/analysis, as far as I can tell) are at:

- https://github.com/golang/lint/issues/130
- https://github.com/dominikh/go-staticcheck/issues/1

Also related:

- https://github.com/charithe/durationcheck

