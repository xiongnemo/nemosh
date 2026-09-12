package applets

import (
	"math"
	"math/rand"
	"time"
)

// The numeric built-ins.
//
// **`rand` is reproducible without `srand`.** The sequence starts from seed 1 on every run,
// which is what both references do and what makes an awk program that samples or shuffles
// give the same answer twice. `srand(x)` sets the seed and answers the *previous* one --
// measured: `srand(42)` prints 1, and a following `srand(7)` prints 42.
//
// The numbers themselves are not the references' numbers and cannot be: which floats a seed
// produces belongs to the generator, not to the language. What is promised is the range,
// `0 <= rand() < 1`, and that one seed always gives one sequence. The tests check those two
// things rather than particular values.

func (in *awkInterp) evalMathBuiltin(node awkBuiltinExpr) (awkValue, error) {
	switch node.name {
	case "rand":
		return awkNum(in.random.Float64()), nil
	case "srand":
		return in.builtinSrand(node)
	case "atan2":
		return in.builtinAtan2(node)
	}
	operand, err := in.argNumber(node, 0)
	if err != nil {
		return awkValue{}, err
	}
	switch node.name {
	case "int":
		// Truncated toward zero, so `int(-3.9)` is -3 rather than -4.
		return awkNum(math.Trunc(operand)), nil
	case "sqrt":
		return awkNum(math.Sqrt(operand)), nil
	case "exp":
		return awkNum(math.Exp(operand)), nil
	case "log":
		// `log(0)` is -inf and `log(-1)` is nan rather than an error: both references
		// carry the value through, and awk_value.go already renders them.
		return awkNum(math.Log(operand)), nil
	case "sin":
		return awkNum(math.Sin(operand)), nil
	case "cos":
		return awkNum(math.Cos(operand)), nil
	}
	return awkValue{}, in.errorf("%s is not supported yet", node.name)
}

func (in *awkInterp) builtinAtan2(node awkBuiltinExpr) (awkValue, error) {
	y, err := in.argNumber(node, 0)
	if err != nil {
		return awkValue{}, err
	}
	x, err := in.argNumber(node, 1)
	if err != nil {
		return awkValue{}, err
	}
	return awkNum(math.Atan2(y, x)), nil
}

// builtinSrand reseeds and answers the seed that was in force before.
func (in *awkInterp) builtinSrand(node awkBuiltinExpr) (awkValue, error) {
	previous := in.randSeed
	seed := float64(time.Now().UnixNano())
	if len(node.args) > 0 {
		given, err := in.argNumber(node, 0)
		if err != nil {
			return awkValue{}, err
		}
		seed = given
	}
	in.randSeed = seed
	in.random = rand.New(rand.NewSource(awkToInt64(seed)))
	return awkNum(previous), nil
}
