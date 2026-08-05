package runtime

import "context"

func (r Runtime) executeSimpleCommand(ctx context.Context, command simpleCommand, savedStatus int) lineResult {
	return r.runParsedWords(ctx, command.words, cloneRedirects(command.redirects), savedStatus)
}

func cloneRedirects(source []redirectOperation) []redirectOperation {
	return append([]redirectOperation(nil), source...)
}
