package runtime

import (
	"fmt"
	"io"
)

func (r Runtime) applyRedirects(args []string) ([]string, Streams, func() error, error) {
	streams := r.streams
	var files []io.Closer
	cleanup := func() error {
		var closeErr error
		for i := len(files) - 1; i >= 0; i-- {
			if err := files[i].Close(); err != nil && closeErr == nil {
				closeErr = err
			}
		}
		return closeErr
	}
	commandArgs := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case ">":
			if i+1 >= len(args) {
				if err := cleanup(); err != nil {
					return nil, Streams{}, func() error { return nil }, err
				}
				return nil, Streams{}, func() error { return nil }, fmt.Errorf(">: missing target")
			}
			file, err := openOutputRedirect(args[i+1], streams)
			if err != nil {
				if closeErr := cleanup(); closeErr != nil {
					return nil, Streams{}, func() error { return nil }, closeErr
				}
				return nil, Streams{}, func() error { return nil }, fmt.Errorf("%s: %w", args[i+1], err)
			}
			streams.Stdout = file
			files = append(files, file)
			i++
		case "<":
			if i+1 >= len(args) {
				if err := cleanup(); err != nil {
					return nil, Streams{}, func() error { return nil }, err
				}
				return nil, Streams{}, func() error { return nil }, fmt.Errorf("<: missing target")
			}
			file, err := openInputRedirect(args[i+1], streams)
			if err != nil {
				if closeErr := cleanup(); closeErr != nil {
					return nil, Streams{}, func() error { return nil }, closeErr
				}
				return nil, Streams{}, func() error { return nil }, fmt.Errorf("%s: %w", args[i+1], err)
			}
			streams.Stdin = file
			files = append(files, file)
			i++
		case "2>&1":
			streams.Stderr = streams.Stdout
		default:
			commandArgs = append(commandArgs, args[i])
		}
	}
	if len(commandArgs) == 0 {
		if err := cleanup(); err != nil {
			return nil, Streams{}, func() error { return nil }, err
		}
		return nil, Streams{}, func() error { return nil }, fmt.Errorf("empty command after redirection")
	}
	return commandArgs, streams, cleanup, nil
}
