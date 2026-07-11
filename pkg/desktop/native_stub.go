//go:build !darwin

package desktop

import (
	"context"
	"errors"
)

var ErrNativeUnsupported = errors.New("native desktop window is only supported on macOS")

func runNative(context.Context, string, Commands) error {
	return ErrNativeUnsupported
}
