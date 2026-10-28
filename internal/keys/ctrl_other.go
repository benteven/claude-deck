//go:build !darwin || !cgo

package keys

func CtrlDown() (down, known bool) {
	return false, false
}
