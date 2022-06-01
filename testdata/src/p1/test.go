package p1

import (
	"net/http"
	"time"
	timeAliased "time"
)

const untypedConst = 30
const typedConst time.Duration = time.Second
const ignored1, typedConst2 = 10, time.Duration(2)
const ignored2, untypedConst2 = 10, 10

const typedConstImplicit = time.Second
const derivedConst = typedConst
const derivedUntypedConst = untypedConst
const intTypedConst int = 10

type TestStruct struct {
	DurationField time.Duration
}

func useTestStruct(testStruct TestStruct) {
}

func TestFunction() {
	var a time.Duration

	// non suspicious
	a = 10 * time.Second
	a = time.Second
	a = time.Duration(10)
	a = timeAliased.Duration(10)
	a = typedConst
	a = typedConstImplicit
	a = derivedConst
	a = 0
	a = typedConst2
	_ = 1

	// suspicious
	a = 10                                       // want `untyped constant in time.Duration assignment`
	a = untypedConst                             // want `untyped constant in time.Duration assignment`
	a = untypedConst2                            // want `untyped constant in time.Duration assignment`
	a = derivedUntypedConst                      // want `untyped constant in time.Duration assignment`
	a = 10 + time.Second                         // want `untyped constant in time.Duration assignment`
	b := TestStruct{DurationField: 20}           // want `untyped constant in time.Duration assignment`
	useTestStruct(TestStruct{DurationField: 20}) // want `untyped constant in time.Duration assignment`
	var c, d time.Duration = time.Second, 10     // want `untyped constant in time.Duration assignment`
	const e time.Duration = 10                   // want `untyped constant in time.Duration assignment`

	_ = a
	_ = b
	_ = c
	_ = d
}

func TestHttpTimeout() {
	const timeout = 5
	_ = http.Client{
		Timeout: timeout, // want `untyped constant in time.Duration assignment`
	}
	_ = http.Client{
		Timeout: 10, // want `untyped constant in time.Duration assignment`
	}
}

func TestTicker() {
	t := time.NewTicker(10) // want `untyped constant in time.Duration argument`
	t.Stop()
}

func TestSleep() {
	const timeout = 5
	const typedTimeout = 5 * time.Nanosecond

	time.Sleep(10)      // want `untyped constant in time.Duration argument`
	time.Sleep(timeout) // want `untyped constant in time.Duration argument`
	time.Sleep(typedTimeout)
	time.Sleep(5 * time.Nanosecond)
}
