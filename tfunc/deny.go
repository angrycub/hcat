package tfunc

import "errors"

var errDisabled = errors.New("function disabled")

// DenyFunc always returns an error, to be used in place of template functions
// that you want denied. For use with the FuncMapMerge.
func DenyFunc(...interface{}) (string, error) {
	return "", errDisabled
}
