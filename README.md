# DurationLint

**NOTE**: this check is currently a work in progress, and needs some more testing. Issues and suggestions are welcome.

DurationLint disallows usage of untyped literals and constants as time.Duration.

It prevents the accidental uses of small constants as time.Duration values that result in unintended time.Durations of a few nanoseconds. As an example:

```go
package main
import "net/http"
import "time"

func example() {
	// Programmer here mistakenly defined a constant as 5, instead of 5 * time.Seconds
	const timeout = 5
	
	// When using a literal, or an untyped constant like the one above, in a 
	// place where a time.Duration is expected, the value gets automatically
	// promoted, resulting in a 5 nanosecond timeout being used.
	//
	// All the following uses are flagged by the check:

	_ = http.Client { Timeout: timeout }
	var duration time.Duration = timeout
	time.Sleep(timeout)
	time.Sleep(5)
	
	// Multiplication against typed values of time.Duration are allowed, so
	// you can silence the check by multiplying untyped values against
	// existing time units, converting typed values using `time.Duration()`
	// first when needed.
	// 
	// The following uses are not flagged:

	const timeoutInSeconds int = 5
	time.Sleep(time.Duration(timeoutInSeconds) * time.Second)
	time.Sleep(5 * time.Second)
	const correctTimeout = 5  * time.Second
	time.Sleep(correctTimeout)
	
	// Note that when the "forbid-improper-conversions" flag is set to
	// true, raw conversions using `time.Duration()` are not allowed
	// unless immediately multiplied by a time unit. This prevents usage
	// errors from forgetting that `time.Duration()` also converts to
	// nanoseconds.
	//
	// The following uses are flagged by the check with the flag enabled:

	time.Sleep(time.Duration(timeoutInSeconds)) 
	time.Sleep(time.Duration(500)) 
}
```

See `./testdata/src/p1/test.go` and `./testdata/src/forbid_improper_conversion/test.go` for more.

## Known issues

- To ensure that casting to time.Duration is always possible while allowing the `time` package to be aliased,
  the check currently ignores calls to functions named `Duration` regardless of their package.
- Various edge cases are not yet handled, such as parenthetical expressions. Tickets will be created to track these.

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

