package units

import "testing"

// TestParseValue should cover docs/01-unit-converter-cli.md's value-parsing table:
// "100", "-273.15", "1.5e3", "+4" (ok); "abc", "" (not a number); "NaN", "Inf",
// "+Inf", "-Inf" (not finite).
func TestParseValue(t *testing.T) {
	t.Skip(`TODO(you): implement — see docs/01-unit-converter-cli.md "Test requirements"`)
}

// TestUnknownUnit should cover case rows F5-F8 — asserting "from" is reported
// before "to" when both are unknown.
func TestUnknownUnit(t *testing.T) {
	t.Skip(`TODO(you): implement — see docs/01-unit-converter-cli.md "Test requirements"`)
}

// TestCaseInsensitive should cover: KM/Km/km resolve to the same unit; kib == KiB.
func TestCaseInsensitive(t *testing.T) {
	t.Skip(`TODO(you): implement — see docs/01-unit-converter-cli.md "Test requirements"`)
}

// TestListDeterministic should render List() twice and assert byte-identical
// output (see case rows S41-S43 and the Definition of Done).
func TestListDeterministic(t *testing.T) {
	t.Skip(`TODO(you): implement — see docs/01-unit-converter-cli.md "Test requirements"`)
}
