// Conversion logic for convert: ratio-based conversion (length, mass, data) and
// temperature's formula-based conversion, plus the public Convert entry point.
//
// See docs/01-unit-converter-cli.md "Requirements" and "Background" for the exact
// formulas, the absolute-zero domain checks, and the negative-value rule for
// length/mass/data.
package units

// toBase converts a value in unit u to its dimension's base unit.
//
// Named returns per docs/01-unit-converter-cli.md self-check 5 — think about what
// base should be before your first assignment to it, and what a naked return in
// the error branch sends back.
//
// TODO(you): implement. Ratio dimensions (length/mass/data) are a simple
// multiply; temperature needs its own formula (see "Background") and an
// absolute-zero domain check (see the "below absolute zero" error catalogue
// entry and resolution-order step 10).
func toBase(v float64, u Unit) (base float64, err error) {
	panic("TODO: implement toBase — see docs/01-unit-converter-cli.md")
}

// fromBase converts a value in a dimension's base unit to unit u.
//
// TODO(you): implement, symmetric to toBase.
func fromBase(base float64, u Unit) (v float64, err error) {
	panic("TODO: implement fromBase — see docs/01-unit-converter-cli.md")
}

// Convert converts value from the unit named from to the unit named to. It
// implements the full "Resolution / precedence order" from
// docs/01-unit-converter-cli.md, steps 7-10 (unit lookup, dimension check, domain
// checks) — steps 1-6 and 11 (flags, formatting, printing) belong in cmd/convert.
//
// TODO(you): implement. See the error catalogue for exact message formats and
// the SUCCESS/FAILURE case tables for the full behaviour this must satisfy.
func Convert(value float64, from, to string) (float64, error) {
	panic("TODO: implement Convert — see docs/01-unit-converter-cli.md")
}
