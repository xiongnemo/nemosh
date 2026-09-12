package applets

import (
	"fmt"
	"math/big"
)

// The fixed-point decimal arithmetic `bc` and `dc` are built on.
//
// A number is an **integer and a scale**: `unscaled × 10^-scale`, so 3.14 is 314 with a
// scale of 2. Not a float, and not a rational. A float cannot represent `0.1` and bc's whole
// point is that it can; a rational would make `1/3` exact and then have no answer to "print
// it", which is the question bc exists to answer.
//
// **The scale of a result is part of the specification, not an implementation detail.** Each
// operation below says what its result's scale is, because POSIX does, and because the
// answers differ visibly: with `scale=0`, `1.5 + 1.5` is `3.0` while `1.50 - 1.5` is `0`, and
// `2.5 * 2.5` is `6.2` rather than `6.25` or `6`. All three were measured.

// bigDecimal is unscaled × 10^-scale. The zero value is 0 with a scale of 0.
type bigDecimal struct {
	unscaled *big.Int
	scale    int
}

func decimalFromInt(value int64) bigDecimal {
	return bigDecimal{unscaled: big.NewInt(value)}
}

func (d bigDecimal) integer() *big.Int {
	if d.unscaled == nil {
		return big.NewInt(0)
	}
	return d.unscaled
}

func (d bigDecimal) isZero() bool { return d.integer().Sign() == 0 }
func (d bigDecimal) sign() int    { return d.integer().Sign() }

// rescale answers the same value written with a different scale, truncating toward zero when
// the new scale is smaller.
//
// Truncation rather than rounding, throughout: bc truncates, which is why `scale=2; 1/3*3`
// is `.99` and not `1.00`.
func (d bigDecimal) rescale(scale int) bigDecimal {
	if scale == d.scale {
		return d
	}
	value := new(big.Int).Set(d.integer())
	if scale > d.scale {
		value.Mul(value, powerOfTen(scale-d.scale))
		return bigDecimal{unscaled: value, scale: scale}
	}
	value.Quo(value, powerOfTen(d.scale-scale))
	return bigDecimal{unscaled: value, scale: scale}
}

func powerOfTen(exponent int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(exponent)), nil)
}

// addDecimal and subDecimal keep the **larger** of the two scales, which is why `1.5 + 1.5`
// prints `3.0` even when scale is 0.
func addDecimal(a, b bigDecimal) bigDecimal {
	scale := max(a.scale, b.scale)
	return bigDecimal{unscaled: new(big.Int).Add(a.rescale(scale).integer(), b.rescale(scale).integer()), scale: scale}
}

func subDecimal(a, b bigDecimal) bigDecimal {
	scale := max(a.scale, b.scale)
	return bigDecimal{unscaled: new(big.Int).Sub(a.rescale(scale).integer(), b.rescale(scale).integer()), scale: scale}
}

// mulDecimal answers a scale of `min(scale(a)+scale(b), max(scale, scale(a), scale(b)))`.
//
// That rule is why `scale=0; 2.5*2.5` is `6.2`: the exact product has two decimal places,
// but the result may keep only one, so it is truncated to `6.2` rather than rounded to `6.3`
// or cut to `6`.
func mulDecimal(a, b bigDecimal, scale int) bigDecimal {
	exact := bigDecimal{unscaled: new(big.Int).Mul(a.integer(), b.integer()), scale: a.scale + b.scale}
	want := min(a.scale+b.scale, max(scale, max(a.scale, b.scale)))
	return exact.rescale(want)
}

// divDecimal answers a scale of exactly `scale`, truncated toward zero.
func divDecimal(a, b bigDecimal, scale int) (bigDecimal, error) {
	if b.isZero() {
		return bigDecimal{}, fmt.Errorf("divide by zero")
	}
	// Both sides are lifted so the quotient has `scale` digits before the integer
	// division, which is what makes the truncation happen in the right place.
	numerator := new(big.Int).Mul(a.integer(), powerOfTen(scale+b.scale))
	denominator := new(big.Int).Mul(b.integer(), powerOfTen(a.scale))
	return bigDecimal{unscaled: numerator.Quo(numerator, denominator), scale: scale}, nil
}

// modDecimal is `a - (a/b)*b`, with the division done at the **current scale**.
//
// That is POSIX's definition and it is not the remainder people expect: with `scale=2`,
// `10 % 3` is `.01`, because `10/3` is `3.33` and `10 - 3.33*3` is `0.01`. Measured, and
// worth stating because a script using `%` as an integer remainder has to set `scale=0`.
func modDecimal(a, b bigDecimal, scale int) (bigDecimal, error) {
	quotient, err := divDecimal(a, b, scale)
	if err != nil {
		return bigDecimal{}, err
	}
	product := mulDecimal(quotient, b, scale+b.scale)
	result := subDecimal(a, product)
	return result.rescale(max(scale+b.scale, a.scale)), nil
}

// powDecimal raises a number to an **integer** power, which is all bc allows.
//
// A negative exponent is `1/(a^-n)` at the current scale, so `scale=4; 2^-2` is `.2500`.
func powDecimal(a bigDecimal, exponent *big.Int, scale int) (bigDecimal, error) {
	if !exponent.IsInt64() {
		return bigDecimal{}, fmt.Errorf("exponent too large")
	}
	power := exponent.Int64()
	if power < 0 {
		positive, err := powDecimal(a, new(big.Int).Neg(exponent), scale)
		if err != nil {
			return bigDecimal{}, err
		}
		return divDecimal(decimalFromInt(1), positive, scale)
	}
	result := decimalFromInt(1)
	for index := int64(0); index < power; index++ {
		result = bigDecimal{
			unscaled: new(big.Int).Mul(result.integer(), a.integer()),
			scale:    result.scale + a.scale,
		}
	}
	// The exact product, then truncated the way a repeated multiplication would be:
	// min(scale(a)*n, max(scale, scale(a))).
	want := min(a.scale*int(power), max(scale, a.scale))
	if power == 0 {
		want = 0
	}
	return result.rescale(want), nil
}

// sqrtDecimal answers the square root at `max(scale, scale(x))`, truncated.
//
// Newton's method on the scaled integer, which is exact arithmetic throughout -- a float
// square root would be wrong in the last digits at any scale worth asking for.
func sqrtDecimal(a bigDecimal, scale int) (bigDecimal, error) {
	if a.sign() < 0 {
		return bigDecimal{}, fmt.Errorf("square root of a negative number")
	}
	want := max(scale, a.scale)
	if a.isZero() {
		return bigDecimal{scale: want}.rescale(want), nil
	}
	// Two extra digits so the truncation at the end is of a correct digit rather than of
	// an approximation's last one.
	working := want + 2
	lifted := a.rescale(2 * working).integer()
	root := new(big.Int).Sqrt(lifted)
	return bigDecimal{unscaled: root, scale: working}.rescale(want), nil
}

// isIntegral reports whether the number has nothing after the decimal point.
//
// Not the same as `scale == 0`: `2.0` has a scale of 1 and is still an integer, and it is the
// *value* that decides whether it can be an exponent.
func (d bigDecimal) isIntegral() bool {
	if d.scale == 0 {
		return true
	}
	remainder := new(big.Int).Rem(d.integer(), powerOfTen(d.scale))
	return remainder.Sign() == 0
}

func compareDecimal(a, b bigDecimal) int {
	scale := max(a.scale, b.scale)
	return a.rescale(scale).integer().Cmp(b.rescale(scale).integer())
}

func negateDecimal(a bigDecimal) bigDecimal {
	return bigDecimal{unscaled: new(big.Int).Neg(a.integer()), scale: a.scale}
}

// digitCount is bc's `length`: how many significant digits the number has altogether.
//
// A zero has length 1, and a pure fraction counts its leading zeros -- `length(.05)` is 2 --
// which is what makes it a count of the digits written rather than of the value.
func digitCount(a bigDecimal) int {
	text := new(big.Int).Abs(a.integer()).String()
	if text == "0" {
		return 1
	}
	if len(text) < a.scale {
		return a.scale
	}
	return len(text)
}
