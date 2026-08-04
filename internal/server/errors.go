package server

import "errors"

var (
	errDuplicateSlug     = errors.New("profile slug already exists")
	errEmptyName         = errors.New("profile name is required")
	errEmptyProxy        = errors.New("profile proxy host is required")
	errLastProfile       = errors.New("cannot delete the last profile")
	errNoImportedDomains = errors.New("domain list did not contain importable domains")
	errProfileNotFound   = errors.New("profile not found")
)
