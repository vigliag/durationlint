package forbid_improper_conversion

import (
	"time"
)

const untypedConst = 30
const implicitTimeConst = time.Second
const typedConst time.Duration = time.Second
const ignored2, untypedConst2 = 10, 10

const derivedConst = typedConst
const derivedUntypedConst = untypedConst
const intTypedConst int = 10

type CustomDuration int

const customDurationConst CustomDuration = 10

type TestStruct struct {
	DurationField1 time.Duration
	DurationField2 time.Duration
	DurationField3 time.Duration
}

func useTestStruct(TestStruct) {
}

func returnsInteger() uint8 {
	return 5
}

func returnsDuration(integer int) time.Duration {
	return time.Duration(integer) * time.Second
}

func acceptsDuration(time.Duration) {
}

func TestDurationConversionSuccess() {
	var predefindInt int = 5
	_ = time.Duration(customDurationConst)
	_ = time.Duration(20 * (time.Second + time.Microsecond))
	_ = time.Duration(5) * time.Second
	_ = time.Duration(predefindInt) * time.Second
	_ = time.Duration(untypedConst) * time.Nanosecond
	_ = 50 * time.Duration(untypedConst) * time.Nanosecond * time.Duration(predefindInt)
	_ = time.Duration(typedConst)
	_ = time.Duration(implicitTimeConst)
	_ = time.Duration(returnsDuration(5))
}

func TestDurationConversionErrors() {
	var predefindInt int = 5
	_ = time.Duration(10)                  // want `converting integer via time.Duration.. without multiplication by proper duration`
	_ = time.Duration(untypedConst)        // want `converting integer via time.Duration.. without multiplication by proper duration`
	_ = time.Duration(derivedUntypedConst) // want `converting integer via time.Duration.. without multiplication by proper duration`
	_ = time.Duration(intTypedConst)       // want `converting integer via time.Duration.. without multiplication by proper duration`

	// more complex nesting with conversion over call expression
	useTestStruct(TestStruct{
		DurationField1: time.Duration(returnsInteger()), // want `converting integer via time.Duration.. without multiplication by proper duration`
		DurationField2: time.Duration(predefindInt),     // want `converting integer via time.Duration.. without multiplication by proper duration`
		DurationField3: time.Duration(10),               // want `converting integer via time.Duration.. without multiplication by proper duration`
	})
	acceptsDuration(time.Duration(returnsInteger())) // want `converting integer via time.Duration.. without multiplication by proper duration`
	acceptsDuration(time.Duration(predefindInt))     // want `converting integer via time.Duration.. without multiplication by proper duration`
	acceptsDuration(time.Duration(10))               // want `converting integer via time.Duration.. without multiplication by proper duration`
}
