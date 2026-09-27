//go:build !windows

package applets

import "path/filepath"

// FollowLinks is path with every symbolic link in it followed.
func FollowLinks(path string) (string, error) { return filepath.EvalSymlinks(path) }
