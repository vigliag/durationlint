package testcase

import (
	"time"
)

const someTime = 30
const someTime2 time.Duration = time.Second
const someTime3 = someTime2

type MyStruct struct {
	A time.Duration
}

func Fn1() {
	var a time.Duration

	a = 10                // marked
	a = someTime          // marked
	a = 10 * time.Second  // non-marked
	a = 10 + time.Second  // marked
	a = time.Second       // non-marked
	a = time.Duration(10) // non-marked
	a = someTime2         // non-marked
	a = someTime3         // non-marked
	b := MyStruct{A: 20}  // marked

	_ = a
	_ = b
}
