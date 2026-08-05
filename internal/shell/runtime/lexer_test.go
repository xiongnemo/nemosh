package runtime

import (
	"reflect"
	"testing"
)

func TestScanShellTokens_classifiesOnlyActiveOperators(t *testing.T) {
	tokens, err := scanShellTokens(`echo "|" \&\& '||' \> "2>&1" 2>&1 >out`)
	if err != nil {
		t.Fatalf("scan tokens: %v", err)
	}

	want := []shellToken{
		{kind: tokenWord, value: "echo"},
		{kind: tokenWord, value: "|"},
		{kind: tokenWord, value: "&&"},
		{kind: tokenWord, value: "||"},
		{kind: tokenWord, value: ">"},
		{kind: tokenWord, value: "2>&1"},
		{kind: tokenRedirect, value: "2>&"},
		{kind: tokenWord, value: "1"},
		{kind: tokenRedirect, value: ">"},
		{kind: tokenWord, value: "out"},
	}
	if !reflect.DeepEqual(lexicalTokens(tokens), want) {
		t.Fatalf("tokens:\n got %#v\nwant %#v", tokens, want)
	}
}

func TestScanShellTokens_separatesRedirectOperatorsFromQuotedAndEscapedOperands(t *testing.T) {
	tokens, err := scanShellTokens(`echo>"$file" 2>'file name' 3>file\ name <in>out`)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	want := []shellToken{
		{kind: tokenWord, value: "echo"},
		{kind: tokenRedirect, value: ">"},
		{kind: tokenWord, value: "$file"},
		{kind: tokenRedirect, value: "2>"},
		{kind: tokenWord, value: "file name"},
		{kind: tokenRedirect, value: "3>"},
		{kind: tokenWord, value: "file name"},
		{kind: tokenRedirect, value: "<"},
		{kind: tokenWord, value: "in"},
		{kind: tokenRedirect, value: ">"},
		{kind: tokenWord, value: "out"},
	}
	if !reflect.DeepEqual(lexicalTokens(tokens), want) {
		t.Fatalf("tokens:\n got %#v\nwant %#v", tokens, want)
	}
}

func TestScanShellTokens_keepsQuotedAndEscapedRedirectSpellingsAsWords(t *testing.T) {
	tokens, err := scanShellTokens(`echo '>' \> "2>&1"`)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	for _, token := range tokens {
		if token.kind != tokenWord {
			t.Fatalf("active token: %#v", token)
		}
	}
}

func TestScanShellTokens_foldsIONumbersOnlyForRawUnquotedDigits(t *testing.T) {
	echo := shellToken{kind: tokenWord, value: "echo"}
	file := shellToken{kind: tokenWord, value: "file"}
	bare := shellToken{kind: tokenRedirect, value: ">"}
	protected := func(retained string) []shellToken {
		return []shellToken{echo, {kind: tokenWord, value: retained}, bare, file}
	}
	numbered := func(descriptor string) []shellToken {
		return []shellToken{echo, {kind: tokenRedirect, value: descriptor + ">"}, file}
	}
	for _, testCase := range []struct {
		name      string
		input     string
		want      []shellToken
		wantParts []wordPart
	}{
		{name: "unquoted digit", input: `echo 2>file`, want: numbered("2")},
		{name: "unquoted multi digit", input: `echo 23>file`, want: numbered("23")},
		{name: "unquoted leading zero", input: `echo 02>file`, want: numbered("02")},
		{
			name: "double quoted digit", input: `echo "2">file`, want: protected("2"),
			wantParts: []wordPart{{kind: wordPartLiteral, text: "2", quote: quoteDouble}},
		},
		{
			name: "single quoted digit", input: `echo '2'>file`, want: protected("2"),
			wantParts: []wordPart{{kind: wordPartLiteral, text: "2", quote: quoteSingle}},
		},
		{
			name: "escaped digit", input: `echo \2>file`, want: protected("2"),
			wantParts: []wordPart{{kind: wordPartEscaped, text: "2", quote: quoteUnquoted}},
		},
		{name: "leading empty single quotes", input: `echo ''2>file`, want: protected("2")},
		{name: "trailing empty single quotes", input: `echo 2''>file`, want: protected("2")},
		{name: "leading empty double quotes", input: `echo ""2>file`, want: protected("2")},
		{name: "trailing empty double quotes", input: `echo 2"">file`, want: protected("2")},
		{name: "detached digit", input: `echo 2 >file`, want: protected("2")},
		{name: "digit after letter", input: `echo a2>file`, want: protected("a2")},
		{name: "digit after sign", input: `echo -2>file`, want: protected("-2")},
		{name: "parameter spelled digit", input: `echo $fd>file`, want: protected("$fd")},
		{
			name:  "escaped operator after digit",
			input: `echo 2\>file`,
			want:  []shellToken{echo, {kind: tokenWord, value: "2>file"}},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			tokens, err := scanShellTokens(testCase.input)
			if err != nil {
				t.Fatalf("scan: %v", err)
			}
			if !reflect.DeepEqual(lexicalTokens(tokens), testCase.want) {
				t.Fatalf("tokens:\n got %#v\nwant %#v", tokens, testCase.want)
			}
			if testCase.wantParts == nil {
				return
			}
			if tokens[1].parsed == nil || !reflect.DeepEqual(tokens[1].parsed.parts, testCase.wantParts) {
				t.Fatalf("retained word provenance: %#v want %#v", tokens[1].parsed, testCase.wantParts)
			}
		})
	}
}

func TestScanShellTokens_rejectsTrailingUnquotedBackslash(t *testing.T) {
	if _, err := scanShellTokens(`echo trailing\`); err == nil {
		t.Fatal("expected trailing backslash error")
	}
}

func TestScanShellTokens_keepsCommandSubstitutionAtomic(t *testing.T) {
	tokens, err := scanShellTokens(`echo $(printf 'a|b && c') | cat`)
	if err != nil {
		t.Fatalf("scan tokens: %v", err)
	}

	want := []shellToken{
		{kind: tokenWord, value: "echo"},
		{kind: tokenWord, value: "$(printf 'a|b && c')"},
		{kind: tokenPipe, value: "|"},
		{kind: tokenWord, value: "cat"},
	}
	if !reflect.DeepEqual(lexicalTokens(tokens), want) {
		t.Fatalf("tokens:\n got %#v\nwant %#v", tokens, want)
	}
}

func lexicalTokens(tokens []shellToken) []shellToken {
	result := append([]shellToken(nil), tokens...)
	for index := range result {
		result[index].parsed = nil
	}
	return result
}

func TestSplitTokenListAndPipeline_usesKinds(t *testing.T) {
	tokens := []shellToken{
		{kind: tokenWord, value: "echo"},
		{kind: tokenWord, value: "&&"},
		{kind: tokenAndIf, value: "&&"},
		{kind: tokenWord, value: "echo"},
		{kind: tokenWord, value: "|"},
		{kind: tokenPipe, value: "|"},
		{kind: tokenWord, value: "cat"},
	}

	segments := splitTokenList(tokens)
	commands, err := splitTokenPipeline(segments[2].tokens)
	if err != nil {
		t.Fatalf("split pipeline: %v", err)
	}

	if len(segments) != 3 || segments[1].operator != tokenAndIf {
		t.Fatalf("list segments: %#v", segments)
	}
	if len(commands) != 2 || commands[0][1].kind != tokenWord || commands[0][1].value != "|" {
		t.Fatalf("pipeline commands: %#v", commands)
	}
}
