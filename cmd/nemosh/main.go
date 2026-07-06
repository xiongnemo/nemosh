package main

import (
	"context"
	"fmt"
	"os"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func main() {
	if err := run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	name := applets.InvocationName(args)
	if applet, ok := applets.DefaultRegistry.Lookup(name); ok {
		return applet.Run(ctx, args[1:], os.Stdin, os.Stdout, os.Stderr)
	}

	if len(args) > 1 {
		if applet, ok := applets.DefaultRegistry.Lookup(args[1]); ok {
			return applet.Run(ctx, args[2:], os.Stdin, os.Stdout, os.Stderr)
		}
	}

	return fmt.Errorf("nemosh: shell runtime is not implemented yet")
}
