package applets

import (
	"fmt"
	"strings"
)

// dateLongOptions are busybox's (coreutils/date.c:144): the long forms and the letter each is,
// taken by any prefix that names one alone, as getopt_long takes them. Only the whole name was
// taken, so `date --da=...` was an unknown option.
var dateLongOptions = map[string]string{
	"rfc-822": "R", "rfc-2822": "R", "set": "s", "utc": "u", "date": "d", "reference": "r",
}

// parseDateArgs reads date's options and operands: a +FMT, and then TIME to set the clock to,
// `MMDDhhmm[[CC]YY][.ss]` among its forms. -I takes its SPEC only in the same word.
func parseDateArgs(args []string, permute bool) (dateRequest, error) {
	request := dateRequest{iso: -1}
	var operands []string
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			operands = append(operands, args[index+1:]...)
			break
		}
		if len(arg) < 2 || arg[0] != '-' {
			if operands = append(operands, arg); !permute {
				operands = append(operands, args[index+1:]...)
				break
			}
			continue
		}
		if strings.HasPrefix(arg, "--") {
			name, value, valued := strings.Cut(arg[2:], "=")
			letter, err := longOptionLetter(dateLongOptions, name)
			takesValue := letter != 0 && strings.IndexByte("sdr", letter) >= 0
			switch {
			case err != nil:
				return request, fmt.Errorf("date: %w", err)
			case letter == 0:
				return request, fmt.Errorf("date: %w", unknownLongOption(arg))
			case valued && !takesValue:
				return request, fmt.Errorf("date: %w", optionTakesNoArgument(name))
			case takesValue && !valued:
				if index+1 >= len(args) {
					return request, fmt.Errorf("date: %w", missingOptionArgument(name))
				}
				index++
				value = args[index]
			}
			request.set(letter, value)
			continue
		}
		for position := 1; position < len(arg); position++ {
			letter := arg[position]
			switch {
			case letter == 'R' || letter == 'u':
				request.set(letter, "")
			case letter == 'I':
				if err := request.setISO(arg[position+1:]); err != nil {
					return request, err
				}
				position = len(arg)
			case strings.IndexByte("sdrD", letter) >= 0:
				value := arg[position+1:]
				if value == "" {
					if index+1 >= len(args) {
						return request, fmt.Errorf("date: %w", missingOptionArgument(string(letter)))
					}
					index++
					value = args[index]
				}
				request.set(letter, value)
				position = len(arg)
			default:
				return request, fmt.Errorf("date: %w", invalidOption(letter))
			}
		}
	}
	return request, request.takeOperands(operands)
}

func (r *dateRequest) set(letter byte, value string) {
	switch letter {
	case 'R':
		r.rfc2822 = true
	case 'u':
		r.utc = true
	case 's', 'd':
		r.input, r.hasInput, r.setting = value, true, letter == 's'
	case 'r':
		r.reference = value
	case 'D':
		r.inputFormat = value
	}
}

// setISO is -I's SPEC, which is date when there is none.
func (r *dateRequest) setISO(spec string) error {
	r.iso = 0
	for index, name := range []string{"date", "hours", "minutes", "seconds", "ns"} {
		if spec == name {
			r.iso = index
			return nil
		}
	}
	if spec != "" {
		return fmt.Errorf("date: invalid argument '%s' for -I", spec)
	}
	return nil
}

// takeOperands reads +FMT and then, without -d or -s, a TIME to set the clock to. The
// `MMDDhhmm[[CC]YY][.ss]` form has its year moved to the front, where touch -t's form has it.
func (r *dateRequest) takeOperands(operands []string) error {
	if r.rfc2822 && r.iso >= 0 {
		return fmt.Errorf("date: -R and -I cannot both be given")
	}
	if len(operands) > 0 && strings.HasPrefix(operands[0], "+") {
		r.format, r.hasFormat = operands[0][1:], true
		operands = operands[1:]
	}
	if !r.hasInput && len(operands) > 0 {
		r.input, r.hasInput, r.setting = moveYearToFront(operands[0]), true, true
		operands = operands[1:]
	}
	if len(operands) > 0 {
		return fmt.Errorf("date: extra operand '%s'", operands[0])
	}
	return nil
}

// moveYearToFront turns `MMDDhhmm[[CC]YY][.ss]` into `[[CC]YY]MMDDhhmm[.ss]`, as busybox's
// date_main does before reading it.
func moveYearToFront(input string) string {
	digits, seconds, dotted := strings.Cut(input, ".")
	if !allDigits(digits) || dotted && (len(seconds) != 2 || !allDigits(seconds)) {
		return input
	}
	year := len(digits) - 8
	if year <= 0 || year > 4 || year%2 == 1 {
		return input
	}
	moved := digits[8:] + digits[:8]
	if dotted {
		moved += "." + seconds
	}
	return moved
}
