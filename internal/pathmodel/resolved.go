package pathmodel

type ResolvedPath struct {
	Canonical Path
	Native    string
	Device    bool
}
