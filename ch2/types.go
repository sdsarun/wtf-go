package ch2

// Type declarations create a new named type from an existing underlying type.
// Syntax: type NewType UnderlyingType
//
// Even though Celsius and Fahrenheit both have float64 as their underlying type,
// the compiler treats them as distinct types — you cannot mix them without an
// explicit conversion. This prevents accidental bugs like adding °C to °F.
//
// When to use a type declaration:
//  1. Add meaning / safety — give a plain primitive a domain-specific name so
//     the compiler rejects wrong-unit assignments at compile time.
//  2. Attach methods — you cannot add methods to a built-in type (float64),
//     but you can add methods to your own named type.
//  3. Improve readability — func Boiling() Celsius is clearer than func Boiling() float64.

// Celsius represents a temperature in degrees Celsius.
// Underlying type is float64, so arithmetic works normally.
//
//	var boiling Celsius = 100  // ✓
//	var body Fahrenheit = 98.6
//	boiling = body             // ✗ compile error: cannot use Fahrenheit as Celsius
//	boiling = Celsius(body)    // ✓ explicit conversion allowed
type Celsius float64

// Fahrenheit represents a temperature in degrees Fahrenheit.
// Same underlying type as Celsius but a different named type.
type Fahrenheit float64

// CToF converts Celsius to Fahrenheit.
// Explicit conversion Fahrenheit(c) is required; the compiler will not do it
// automatically even though both types share the same underlying type.
func CToF(c Celsius) Fahrenheit {
	return Fahrenheit(c*9/5 + 32)
}

// FToC converts Fahrenheit to Celsius.
func FToC(f Fahrenheit) Celsius {
	return Celsius((f - 32) * 5 / 9)
}

func ShowNamedTypeEnforcement() {
	// All three variables hold the same underlying type (float64),
	// but the compiler sees them as completely different types.
	var raw float64 = 20.0
	var c Celsius = 20.0
	var f Fahrenheit = 20.0

	// ✓ Comparing a named type to an untyped constant is fine.
	//   Untyped constants have no fixed type; Go picks the best fit.
	_ = c > 0.5
	_ = f > 0.5

	// ✗ Cannot compare Celsius and Fahrenheit directly — compile error:
	//     invalid operation: c > f (mismatched types Celsius and Fahrenheit)
	// _ = c > f

	// ✗ Cannot assign float64 to Celsius directly — compile error:
	//     cannot use raw (variable of type float64) as type Celsius
	// c = raw

	// ✓ Explicit conversion required when crossing named-type boundaries.
	c = Celsius(raw)
	f = Fahrenheit(raw)
	_ = c
	_ = f

	// ✓ You can pass a Celsius to CToF — types match exactly.
	result := CToF(Celsius(raw))

	// ✗ Cannot pass raw float64 directly — compile error:
	//     cannot use raw (variable of type float64) as type Celsius
	// result := CToF(raw)

	// ✗ Cannot compare Fahrenheit result to Celsius — compile error:
	//     invalid operation: result > c (mismatched types Fahrenheit and Celsius)
	// _ = result > c

	// ✓ Compare result (Fahrenheit) to an untyped constant — fine.
	if result > 0.5 {
		_ = result
	}
}