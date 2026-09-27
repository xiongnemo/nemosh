package runtime_test

import "testing"

// An associative array's keys come out in bash's hash order -- busybox has none, so bash
// decides -- and that order is the same on every run: bucket by bucket, by the key's FNV-1
// hash, the last one set first within a bucket, and every bucket rehashed when the table
// grows. They came out in the order they were set, on the belief that bash's changed from
// run to run. Every row is bash 5.3's, measured.
func TestRuntime_associativeKeysComeOutInBashsHashOrder(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`declare -A m=([x]=1 [y]=2 [z]=3 [xy]=4 [foo]=5 [bar]=6 [10]=7 [2]=8); echo "${!m[@]}"; echo "${m[@]}"`, "2 xy z y x foo bar 10\n8 4 3 2 1 5 6 7\n"},
		{`declare -A n; n[a]=1; n[b]=2; n[c]=3; n[d]=4; n[e]=5; echo "${!n[@]}"`, "e d c b a\n"},
		// z and sa share a bucket: the one set last comes first, an overwrite keeps its
		// place, and a key unset and set again is the last one set.
		{`declare -A m; m[z]=1; m[sa]=2; echo "${!m[@]}"; declare -A n; n[sa]=1; n[z]=2; echo "${!n[@]}"; n[sa]=3; echo "${!n[@]}"; unset 'n[sa]'; n[sa]=4; echo "${!n[@]}"`, "sa z\nz sa\nz sa\nsa z\n"},
		{`declare -A s=([a]=x [b]=y); declare -p s`, "declare -A s=([b]=\"y\" [a]=\"x\" )\n"},
		// The table grows to four times its buckets when it holds twice as many keys.
		{`declare -A g; for ((i=0;i<2047;i++)); do g[k$i]=$i; done; set -- ${!g[@]}; echo "$# $1 $2 $3"; g[x]=1; set -- ${!g[@]}; echo "$# $1 $2 $3"; g[y]=1; set -- ${!g[@]}; echo "$# $1 $2 $3"`, "2047 k1775 k1191 k71\n2048 k1775 k1191 k71\n2049 k1698 k1699 k1696\n"},
		// A subshell's copy and a job's walk the same way.
		{`declare -A m=([x]=1 [z]=2 [sa]=3); ( echo "${!m[@]}" ); { echo "${!m[@]}"; } & wait`, "sa z x\nsa z x\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
