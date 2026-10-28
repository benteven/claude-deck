//go:build darwin && cgo

package keys

/*
#cgo LDFLAGS: -framework CoreGraphics
#include <CoreGraphics/CoreGraphics.h>
static int ctrlDown(void) {
	return (CGEventSourceFlagsState(kCGEventSourceStateCombinedSessionState) & kCGEventFlagMaskControl) != 0;
}
*/
import "C"

func CtrlDown() (down, known bool) {
	return C.ctrlDown() != 0, true
}
