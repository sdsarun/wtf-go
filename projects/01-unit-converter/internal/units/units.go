// Package units implements the unit registry for convert: the set of known units
// grouped by dimension (length, mass, temperature, data), and lookup by token.
//
// See docs/01-unit-converter-cli.md for the full specification — in particular
// "Go concepts you MUST use", "Background", and the "Exact contract" unit tables.
package units

// Dimension identifies a family of comparable units.
//
// TODO(you): add an iota-based const block of Dimension values — Length, Mass,
// Temperature, Data — per docs/01-unit-converter-cli.md "Requirements" point 2.
// TODO(you): give Dimension a String() method (fmt.Stringer) so error messages
// can name a dimension (see the "cannot convert" error catalogue entry).
type Dimension int

// Unit describes one named unit within a Dimension.
//
// TODO(you): add fields per the "Units — canonical token → base-unit factor"
// table in docs/01-unit-converter-cli.md's "Exact contract" — at minimum a
// canonical token, its Dimension, and its base-unit factor (temperature units
// don't have a simple factor; decide how Unit represents that case too).
type Unit struct {
}

// TODO(you): populate the unit registry here from the doc's unit tables (length,
// mass, temperature, data). This is the "init() to populate the unit registry"
// checklist item.
func init() {
}

// Lookup resolves a case-insensitive unit token (e.g. "km", "KM", "Km") to its
// Unit. ok is false for an unknown token.
//
// TODO(you): implement per docs/01-unit-converter-cli.md — see case rows S39/S40
// (case-insensitivity) and F5-F8 (unknown unit).
func Lookup(token string) (Unit, bool) {
	panic("TODO: implement Lookup — see docs/01-unit-converter-cli.md")
}

// List renders every unit grouped by dimension, in the exact deterministic format
// documented in docs/01-unit-converter-cli.md's "Exact contract" (the "-list"
// output block) — units in ascending-factor order, temperature C F K, data
// decimal then binary.
//
// TODO(you): implement.
func List() string {
	panic("TODO: implement List — see docs/01-unit-converter-cli.md")
}
