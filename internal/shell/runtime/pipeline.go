package runtime

import (
	"bytes"
	"context"
	"fmt"
)

func (r Runtime) runPipeline(ctx context.Context, args []string) int {
	commands, err := splitPipeline(args)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		return 2
	}
	if len(commands) == 1 {
		return r.runCommandWithRedirects(ctx, commands[0])
	}
	input := r.streams.Stdin
	status := 0
	pipefailStatus := 0
	for i, command := range commands {
		last := i == len(commands)-1
		streams := r.streams
		streams.Stdin = input
		var output bytes.Buffer
		if !last {
			streams.Stdout = &output
		}
		status = (Runtime{registry: r.registry, streams: streams, vars: r.vars, traps: r.traps, params: r.params, options: r.options, readonly: r.readonly, mask: r.mask, sourceDepth: r.sourceDepth}).runCommandWithRedirects(ctx, command)
		if status != 0 {
			pipefailStatus = status
		}
		input = bytes.NewReader(output.Bytes())
	}
	if r.options.pipefail {
		return pipefailStatus
	}
	return status
}

func splitPipeline(args []string) ([][]string, error) {
	commands := [][]string{{}}
	for _, arg := range args {
		if arg == "|" {
			if len(commands[len(commands)-1]) == 0 {
				return nil, fmt.Errorf("empty command in pipeline")
			}
			commands = append(commands, []string{})
			continue
		}
		commands[len(commands)-1] = append(commands[len(commands)-1], arg)
	}
	if len(commands[len(commands)-1]) == 0 {
		return nil, fmt.Errorf("empty command in pipeline")
	}
	return commands, nil
}
