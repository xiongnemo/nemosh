#!/usr/bin/env python3
"""Generate a nemosh completion spec from a command's own help output.

The point of generating rather than transcribing: the claims come from the
binary that is actually installed, so they are right for its version by
construction, and the provenance in [meta] is a fact rather than a memory.
This is the offline half of the idea -- it runs here, on purpose, by a person.
Nothing in the shell ever runs a program to answer a Tab.

Usage: python3 scripts/completions/generate.py curl > completions/curl.toml
Supported shapes: curl (`--help all`), and any tool whose help lists options as
`  -x, --name <arg>  description` or `  -x  description`.
"""
import datetime
import re
import subprocess
import sys


def run(argv):
    result = subprocess.run(argv, capture_output=True, text=True)
    return (result.stdout or "") + (result.stderr or "")


# The argument must be spelled in angle brackets. Without that anchor the first
# word of the description is read as an argument, and every option in curl's help
# came back as taking one.
# Three shapes seen so far. The argument must be anchored -- by angle brackets
# (curl) or by an equals sign (aria2c) -- because without an anchor the first
# word of the description reads as an argument and every option comes back as
# taking one. A help that spells no arguments at all (wget2) yields none, which
# is the safe direction: an option not claimed to take a value is simply not
# treated as consuming the next word.
OPTION = re.compile(
    r"^\s+(?:-([a-zA-Z0-9])[,]?\s+)?--([a-z0-9.-]+)"
    r"(?:\s+<([^>]+)>|\[?=([A-Z][A-Za-z0-9|_]*)\]?)?\s*(.*)$"
)
BARE = re.compile(r"^\s+-([a-zA-Z0-9])\s\s+(.*)$")
# An argument spelled as a file, a directory or a path is one completion can
# answer with a filename. Anything else -- a port, a cipher, a number -- it
# cannot, and offering the current directory there would be an answer from the
# wrong universe.
FILEISH = re.compile(r"file|dir|path", re.I)


def parse(text):
    short, value_short, file_short = [], [], []
    long, value_long, file_long = [], [], []
    for line in text.splitlines():
        match = OPTION.match(line)
        if match:
            letter, name, bracketed, equalled, _ = match.groups()
            argument = bracketed or equalled
            if name in long:
                continue
            long.append(name)
            if letter and letter not in short:
                short.append(letter)
            if argument:
                value_long.append(name)
                if letter:
                    value_short.append(letter)
                if FILEISH.search(argument):
                    file_long.append(name)
                    if letter:
                        file_short.append(letter)
            continue
        bare = BARE.match(line)
        if bare and bare.group(1) not in short:
            short.append(bare.group(1))
    return short, value_short, file_short, long, value_long, file_long


def quote_list(names):
    return "[" + ", ".join('"%s"' % name for name in names) + "]"


def main():
    name = sys.argv[1]
    help_argv = {"curl": [name, "--help", "all"]}.get(name, [name, "--help"])
    text = run(help_argv)
    version = run([name, "--version"]).splitlines()
    version = version[0].strip() if version else "unknown"
    short, value_short, file_short, long, value_long, file_long = parse(text)
    print("[meta]")
    print('derived-from = "%s"' % " ".join(help_argv))
    print('tool-version = "%s"' % version.replace('"', "'"))
    print('measured-on = "%s"' % datetime.date.today().isoformat())
    print('generated-by = "scripts/completions/generate.py"')
    print()
    print("[command]")
    print('name = "%s"' % name)
    print('operand = "path"')
    print('short = "%s"' % "".join(short))
    print('value-short = "%s"' % "".join(value_short))
    print('file-short = "%s"' % "".join(file_short))
    print("long = %s" % quote_list(long))
    print("value-long = %s" % quote_list(value_long))
    print("file-long = %s" % quote_list(file_long))


if __name__ == "__main__":
    main()
