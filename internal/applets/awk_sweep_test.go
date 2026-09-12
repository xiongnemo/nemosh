package applets

import (
	"os/exec"
	"testing"
)

// A differential sweep over the one-liners people actually write.
//
// The per-feature tests each carry a literal expectation, so they hold where no reference
// is installed. This one is the other half: a corpus of whole programs handed to gawk and
// busybox-w32 with **no expectation of its own**, on the reasoning that the answer to
// `{s+=$1} END{print s}` is not something worth typing out -- what is worth knowing is
// whether three awks agree about it.
//
// It skips when neither reference is present, which is what makes it honest rather than a
// test that passes by doing nothing on CI: the count it checks is reported by the run.
//
// `for (k in a)` is deliberately absent from the corpus. POSIX leaves the order
// unspecified, the two references answer differently, and this awk walks in insertion
// order on purpose -- so a case using it would measure the disagreement rather than
// anything about correctness. Anything dispatching to a command is absent for the same
// kind of reason: a command here is an applet, so the references cannot be asked.

// awkSweepInput is deliberately awkward: ragged field counts, a blank line, leading blanks,
// numbers that are also strings, and a line with no trailing newline problem.
const awkSweepInput = "alpha beta gamma\n1 2 3\n\n  foo bar\n10 9 8\nLast Line Here\n007 x\n"

func TestAwkDifferentialSweep(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("gawk"); err != nil && !busyboxIsTheReference() {
		t.Skip("neither reference is installed")
	}
	corpus := []string{
		// Numbering, counting and selecting.
		`{print NR": "$0}`,
		`NF`,
		`NF == 0`,
		`END{print NR}`,
		`END{print NR" lines"}`,
		`NR % 2`,
		`NR == 2, NR == 4`,
		`NR > 2 && NR < 5 {print}`,
		`/foo/{print}`,
		`!/foo/{print}`,
		`$0 ~ /^[0-9]/ {print "starts with a digit:", $0}`,
		`$1 ~ /^[0-9]+$/ {print "numeric first field:", $1}`,

		// Fields.
		`{print $NF}`,
		`{print $1, $3}`,
		`{print $1 $3}`,
		`{print NF}`,
		`{$1=""; print}`,
		`{$2="X"; print}`,
		`{NF=2; print; print NF}`,
		`{t=$1; $1=$2; $2=t; print}`,
		`{for (i = NF; i > 0; i--) printf "%s ", $i; print ""}`,
		`BEGIN{OFS="-"} {$1=$1; print}`,
		`BEGIN{FS=""} {print NF}`,
		`BEGIN{FS="[ ]+"} {print NF}`,
		`{print ($1 > 5), ($1 == "10"), ($1 < $2)}`,

		// Arithmetic and accumulation.
		`{s += $1} END{print s}`,
		`{s += NF} END{printf "%.2f\n", s / NR}`,
		`{if (length > max) {max = length; line = $0}} END{print max, line}`,
		`{n[NR] = $0} END{for (i = NR; i > 0; i--) print n[i]}`,
		`BEGIN{x = 2} {x *= 2} END{print x}`,
		`{print $1 + 0, $1 "", -$1}`,
		`{print int($1), $1 % 3, $1 ^ 2}`,

		// Strings.
		`{print length($0), $0}`,
		`{print toupper($0)}`,
		`{print tolower($0)}`,
		`{print substr($0, 2, 5)}`,
		`{print index($0, "a")}`,
		`{gsub(/a/, "A"); print}`,
		`{n = gsub(/[aeiou]/, "."); print n, $0}`,
		`{sub(/^[ \t]+/, ""); print "[" $0 "]"}`,
		`{sub(/[ \t]+$/, ""); print "[" $0 "]"}`,
		`{print match($0, /[0-9]+/), RSTART, RLENGTH}`,
		`{n = split($0, w, " "); print n, w[1], w[n]}`,
		`{print $0 $0}`,
		`{printf "%-15s|%5d|%s\n", $1, NR, $NF}`,
		`{printf "%s", $0} END{print ""}`,
		`{print (NR > 1 ? "cont" : "first"), $0}`,

		// Arrays and membership, without depending on iteration order.
		`{seen[$1]++} END{print length(seen), (("foo" in seen) ? "yes" : "no")}`,
		`{a[NR, 1] = $1} END{print a[2, 1], ((3 SUBSEP 1) in a)}`,
		`{a[$1] = NR} END{delete a["foo"]; print length(a)}`,

		// Control flow.
		`{i = 0; while (i < NF) {i++; s = s $i} } END{print s}`,
		`{for (i = 1; i <= NF; i++) if ($i ~ /a/) c++} END{print c + 0}`,
		`{do {n++} while (0)} END{print n}`,
		`NR == 3 {next} {print}`,
		`NR == 3 {exit 0} {print} END{print "end", NR}`,

		// Functions.
		`function len(s) { return length(s) } {print len($0)}`,
		`function max(a, b) { return a > b ? a : b } {m = max(m, NF)} END{print m}`,
		`function fill(A,   i) { for (i = 1; i <= NF; i++) A[i] = $i; return NF }` +
			` {print fill(parts), parts[1]}`,
		`function fact(n) { return n <= 1 ? 1 : n * fact(n - 1) } END{print fact(5)}`,

		// getline from the program's own input, and the built-in variables.
		`BEGIN{while ((getline line) > 0) n++; print n}`,
		`{if ((getline nxt) > 0) print $0 "|" nxt; else print $0 "|<none>"}`,
		`BEGIN{print NR, NF, "[" FILENAME "]", "[" FS "]", "[" OFS "]"}`,
		`END{print NR, FNR, NF}`,
		`BEGIN{CONVFMT="%.2f"} {x = $1 / 3; print x ""}`,
		`BEGIN{OFMT="%.3f"} {print $1 / 7}`,
		`BEGIN{ORS="|"} {print $1}`,
	}
	// Not parallel, deliberately: each case starts two reference processes, and seventy
	// cases racing to spawn them is enough to make Windows refuse some of the launches.
	for _, program := range corpus {
		t.Run(program, func(t *testing.T) {
			got, stderr, status := runAwk(t, program, awkSweepInput)
			if stderr != "" || status != 0 {
				t.Fatalf("%s failed: stderr %q status %d", program, stderr, status)
			}
			checkAgainstReferences(t, program, awkSweepInput, got)
		})
	}
	t.Logf("swept %d one-liners against the installed references", len(corpus))
}
