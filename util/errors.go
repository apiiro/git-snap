package util

const (
	ERROR_BAD_CLONE_PATH    = 201
	ERROR_BAD_CLONE_GIT     = 202
	ERROR_BAD_OUTPUT_PATH   = 203
	ERROR_NO_SHORT_SHA      = 204
	ERROR_NO_REVISION       = 205
	ERROR_FILES_DISCREPANCY = 206
	ERROR_PATH_TOO_LONG     = 101
)

type ErrorWithCode struct {
	StatusCode    int
	InternalError error
}

var _ error = &ErrorWithCode{}

func (e ErrorWithCode) Error() string {
	return e.InternalError.Error()
}
