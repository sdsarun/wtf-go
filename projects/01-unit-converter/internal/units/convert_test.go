package units

import "testing"

// TestConvertRatio should cover case rows S2-S14, S27-S32 (length/mass/data
// conversions), asserting results within relative 1e-9.
func TestConvertRatio(t *testing.T) {
	t.Skip(`TODO(you): implement — see docs/01-unit-converter-cli.md "Test requirements"`)
}

// TestConvertTemperature should cover case rows S15-S26, asserting results
// within 1e-9.
func TestConvertTemperature(t *testing.T) {
	t.Skip(`TODO(you): implement — see docs/01-unit-converter-cli.md "Test requirements"`)
}

// TestDomainErrors should cover case rows F9-F14: negative length/mass/data
// values, and below-absolute-zero temperatures for C/F/K.
func TestDomainErrors(t *testing.T) {
	t.Skip(`TODO(you): implement — see docs/01-unit-converter-cli.md "Test requirements"`)
}

// TestCrossDimension should cover case rows F1-F4, asserting the error message
// names both dimensions.
func TestCrossDimension(t *testing.T) {
	t.Skip(`TODO(you): implement — see docs/01-unit-converter-cli.md "Test requirements"`)
}

// TestRoundTrip should convert every registered unit's value 123.456 to its
// dimension's base unit and back, asserting the result is within relative 1e-9
// of the original.
func TestRoundTrip(t *testing.T) {
	t.Skip(`TODO(you): implement — see docs/01-unit-converter-cli.md "Test requirements"`)
}

// TestFormat should cover output formatting at precision 0, 2, 4, 15, and -exact
// (see the "Exact contract" flag table).
func TestFormat(t *testing.T) {
	t.Skip(`TODO(you): implement — see docs/01-unit-converter-cli.md "Test requirements"`)
}

// ExampleConvert is a testable example that should appear in `go doc`.
//
// TODO(you): implement with a real call to Convert and an "// Output:" comment
// once Convert exists — an empty Example with no Output comment is never run by
// `go test`, so this is a placeholder, not a passing test yet.
func ExampleConvert() {
}
