package applets

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// dd copies blocks, which is what it is for: everything else copies *files*, and dd is how a
// script takes the first 512 bytes of one, or writes into the middle of another without
// disturbing the rest.
//
// Its operands are `name=value`, not options, which is the oldest interface in Unix and the
// reason it needs its own parser. An operand this build does not know is **refused by name**
// rather than ignored -- busybox prints its usage and exits 0 for an unknown one, which
// means `dd if=x of=y bs=1M cnt=3` (a typo for `count`) copies the whole file and reports
// success.
//
// The record counts go to **stderr**, so `dd if=big.bin bs=1 count=10` can still be used in
// a pipeline. `status=none` silences them.

type ddRequest struct {
	input, output      string
	inputSize          int64
	outputSize         int64
	count, skip, seek  int64
	hasCount           bool
	notrunc, syncPad   bool
	upper, lower, swab bool
	silent             bool
}

func newDdApplet() Applet {
	return simpleApplet{name: "dd", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		request, err := parseDdOperands(args)
		if err != nil {
			return err
		}
		return request.run(ctx, ProcessViewFromContext(ctx), stdin, stdout, stderr)
	}}
}

func parseDdOperands(args []string) (ddRequest, error) {
	request := ddRequest{inputSize: 512, outputSize: 512}
	for _, argument := range args {
		name, value, found := strings.Cut(argument, "=")
		if !found {
			return request, fmt.Errorf("unrecognized operand '%s'", argument)
		}
		if err := request.apply(name, value); err != nil {
			return request, err
		}
	}
	return request, nil
}

func (r *ddRequest) apply(name, value string) error {
	switch name {
	case "if":
		r.input = value
		return nil
	case "of":
		r.output = value
		return nil
	case "conv":
		return r.applyConv(value)
	case "status":
		switch value {
		case "none":
			r.silent = true
		case "noxfer", "progress":
			// noxfer drops a line this never prints; progress is a running report, and
			// pretending to honour it would be worse than taking it as the default.
		default:
			return fmt.Errorf("invalid status '%s'", value)
		}
		return nil
	}
	number, err := ddNumber(value)
	if err != nil {
		return err
	}
	switch name {
	case "bs":
		r.inputSize, r.outputSize = number, number
	case "ibs":
		r.inputSize = number
	case "obs":
		r.outputSize = number
	case "count":
		r.count, r.hasCount = number, true
	case "skip":
		r.skip = number
	case "seek":
		r.seek = number
	default:
		return fmt.Errorf("unrecognized operand '%s'", name)
	}
	if r.inputSize < 1 || r.outputSize < 1 {
		return fmt.Errorf("invalid number '%s'", value)
	}
	return nil
}

func (r *ddRequest) applyConv(list string) error {
	for _, conversion := range strings.Split(list, ",") {
		switch conversion {
		case "notrunc":
			r.notrunc = true
		case "sync":
			r.syncPad = true
		case "ucase":
			r.upper = true
		case "lcase":
			r.lower = true
		case "swab":
			r.swab = true
		case "fsync", "noerror":
			// fsync is a flush this already does by closing; noerror would continue past
			// a read failure, which is a data-recovery mode this does not implement and
			// will not pretend to.
			if conversion == "noerror" {
				return fmt.Errorf("conv=noerror is not supported: a read failure is reported rather than skipped")
			}
		default:
			return fmt.Errorf("invalid conversion '%s'", conversion)
		}
	}
	if r.upper && r.lower {
		return fmt.Errorf("conv=ucase and conv=lcase cannot both be given")
	}
	return nil
}

// ddNumber reads a block count or size, with the suffixes dd has always taken.
//
// `b` is 512 and `c` is 1, which are dd's own and catch people who expect them to mean
// bytes and characters the other way round. A product form -- `2x512` -- is accepted because
// scripts use it for exactly that reason.
func ddNumber(text string) (int64, error) {
	total := int64(1)
	for _, factor := range strings.Split(text, "x") {
		value, err := ddOneNumber(factor)
		if err != nil {
			return 0, err
		}
		total *= value
	}
	return total, nil
}

func ddOneNumber(text string) (int64, error) {
	multipliers := []struct {
		suffix string
		factor int64
	}{
		{"KB", 1000}, {"MB", 1000 * 1000}, {"GB", 1000 * 1000 * 1000},
		{"K", 1024}, {"M", 1024 * 1024}, {"G", 1024 * 1024 * 1024},
		{"b", 512}, {"w", 2}, {"c", 1},
	}
	digits, factor := text, int64(1)
	for _, multiplier := range multipliers {
		// Case-sensitive for `b`, `w` and `c`, which are lower case in every dd, and
		// insensitive for the byte units, which are not.
		if multiplier.factor >= 1000 || multiplier.suffix == "K" || multiplier.suffix == "M" || multiplier.suffix == "G" {
			if strings.HasSuffix(strings.ToUpper(text), multiplier.suffix) {
				digits, factor = text[:len(text)-len(multiplier.suffix)], multiplier.factor
				break
			}
			continue
		}
		if strings.HasSuffix(text, multiplier.suffix) {
			digits, factor = text[:len(text)-len(multiplier.suffix)], multiplier.factor
			break
		}
	}
	value, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("invalid number '%s'", text)
	}
	return value * factor, nil
}
