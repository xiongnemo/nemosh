import io

p = 'internal/applets/top_help.go'
s = io.open(p, encoding='utf-8').read()

# A meter legend, which htop has on its F1 screen and this did not.
old = '''	out.WriteString("\\n[aqua]COLOUR[white]\\n")'''
new = '''	out.WriteString("\\n[aqua]THE METERS AT THE TOP[white]\\n")
	for _, meter := range topMeterLegend {
		fmt.Fprintf(&out, "  [yellow]%-6s[white] %s\\n", meter.label, meter.what)
	}

	out.WriteString("\\n[aqua]COLOUR[white]\\n")'''
assert old in s
s = s.replace(old, new, 1)

old = '''// topKeyGroups is the keyboard'''
new = '''// topMeterLegend explains the header, which htop explains on its own help screen and this did not.
//
// htop draws a live mock bar there with each coloured segment labelled -- low, normal, kernel, irq,
// steal, guest, io-wait for the processor, and used, shared, compressed, buffers, cache for memory.
// That is the right idea and the wrong shape to copy: those segments exist because Linux reports CPU
// time split seven ways and memory split six, and the Windows process table does not. A bar here is
// one figure, so what needs explaining is not which colour is which but *what the number is a
// fraction of* -- and for the third one, that it is not swap.
var topMeterLegend = []struct{ label, what string }{
	{"CPU", "how much of every processor was busy since the last sample, then how many there are"},
	{"Mem", "physical memory in use, then the figures: used of total, and how much of it is cache the system can reclaim"},
	{"Cmt", "commit charge -- memory the system has *promised*, against the ceiling on promises. Not swap: Windows has no measure that means what swap means, and this can run out while physical memory still looks free"},
	{"0 1 2", "the same busy fraction per processor, one meter each. Every processor gets one, so the header grows on a machine with many"},
	{"--", "no rate yet. The first sample has nothing to compare against, so the figure is unknown rather than zero"},
}

// topKeyGroups is the keyboard'''
assert old in s
s = s.replace(old, new, 1)
io.open(p, 'w', encoding='utf-8', newline='\n').write(s)

# And a glossary that does not need a terminal, which is htop's `--sort-key help`.
p = 'internal/applets/top.go'
s = io.open(p, encoding='utf-8').read()
old = '''			if options.batch {
				return runTopBatch(ctx, options, stdout)
			}'''
new = '''			if options.glossary {
				// Before anything samples: this answers a question about the table
				// rather than about the machine.
				return writeTopGlossary(stdout)
			}
			if options.batch {
				return runTopBatch(ctx, options, stdout)
			}'''
assert old in s
s = s.replace(old, new, 1)

old = '''	// columns overrides the layout, which is how a script asks for exactly the figures it
	// wants rather than whatever fits the window it was given.
	columns []string
}'''
new = '''	// columns overrides the layout, which is how a script asks for exactly the figures it
	// wants rather than whatever fits the window it was given.
	columns []string
	// glossary asks for the column list and what each one means, on standard output.
	//
	// `-s help` and `-o help`, which is htop's `--sort-key help` and worth copying: it puts
	// the terminology somewhere a person can read, grep and paste without opening the drawn
	// form at all. On this platform that matters more than it does on Linux, because the drawn
	// form needs a real console and a good many terminals here are not one.
	glossary bool
}'''
assert old in s
s = s.replace(old, new, 1)

for flag in ['"-s"', '"-o"']:
    old = '''			case %s:
				text, err := value()
				if err != nil {
					return options, err
				}''' % flag
    assert old in s, flag
    s = s.replace(old, old + '''
				if text == "help" {
					options.glossary = true
					return options, nil
				}''', 1)
io.open(p, 'w', encoding='utf-8', newline='\n').write(s)
print("meter legend and glossary written")
