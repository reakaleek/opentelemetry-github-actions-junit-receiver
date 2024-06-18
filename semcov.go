package opentelemetrygithubactionsjunitreceiver

const (
	// suite keys
	FailedTestsCount  = "tests.suite.failed"
	ErrorTestsCount   = "tests.suite.error"
	PassedTestsCount  = "tests.suite.passed"
	SkippedTestsCount = "tests.suite.skipped"
	TestsDuration     = "tests.suite.duration"
	TestsSuiteName    = "tests.suite.suitename"
	TestsSystemErr    = "tests.suite.systemerr"
	TestsSystemOut    = "tests.suite.systemout"
	TotalTestsCount   = "tests.suite.total"

	// test keys
	TestClassName = "tests.case.classname"
	TestDuration  = "tests.case.duration"
	TestError     = "tests.case.error"
	TestMessage   = "tests.case.message"
	TestStatus    = "tests.case.status"
	TestSystemErr = "tests.case.systemerr"
	TestSystemOut = "tests.case.systemout"
)
