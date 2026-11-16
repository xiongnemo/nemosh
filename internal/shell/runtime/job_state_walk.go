package runtime

// jobStateWalk names every field of jobState once, in the order the encoding has them, for
// jobCodec to read or write. A field added to jobState and not here comes back zero, which
// job_state_codec_test.go's round trip of every field catches.
func jobStateWalk(c *jobCodec, s *jobState) {
	c.string(&s.Program)
	c.int(&s.Line)
	c.string(&s.Functions)
	c.stringMap(&s.FunctionFiles)
	c.stringMap(&s.Vars)
	codecMap(c, &s.Indexed, func(c *jobCodec, array *jobIndexedArray) {
		c.strings(&array.Values)
		c.ints(&array.Live)
		c.bool(&array.Unassigned)
	})
	codecMap(c, &s.Associative, func(c *jobCodec, array *jobAssociativeArray) {
		c.strings(&array.Keys)
		c.strings(&array.Values)
		c.uint32(&array.Buckets)
		c.bool(&array.Unassigned)
	})
	codecMap(c, &s.Attributes, func(c *jobCodec, a *jobAttributes) {
		for _, flag := range []*bool{&a.Integer, &a.Lower, &a.Upper, &a.Exported, &a.Nameref, &a.Declared} {
			c.bool(flag)
		}
	})
	c.strings(&s.Readonly)
	c.stringMap(&s.Aliases)
	codecMap(c, &s.Options, (*jobCodec).bool)
	c.string(&s.Invocation)
	c.string(&s.Name)
	c.strings(&s.Positional)
	c.string(&s.Function)
	codecSlice(c, &s.Frames, func(c *jobCodec, frame *jobFrame) {
		c.string(&frame.Name)
		c.string(&frame.File)
		c.int(&frame.Line)
	})
	c.string(&s.ScriptFile)
	c.stringMap(&s.Traps)
	c.uint16(&s.Umask)
	c.strings(&s.DirStack)
	c.strings(&s.AliasChain)
	c.stringMap(&s.AliasView)
	c.int(&s.Getopts[0])
	c.int(&s.Getopts[1])
	for _, number := range []*int{&s.Seconds, &s.ShellPID, &s.ShellPPID, &s.FunctionDepth, &s.SourceDepth, &s.SubshellDepth} {
		c.int(number)
	}
	c.bool(&s.ErrExitSuppressed)
	codecSlice(c, &s.Descriptors, func(c *jobCodec, descriptor *jobDescriptor) {
		c.int(&descriptor.FD)
		c.string(&descriptor.Handle)
		c.bool(&descriptor.Readable)
		c.bool(&descriptor.Writable)
	})
	c.string(&s.Control)
}
