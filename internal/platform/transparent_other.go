//go:build !linux || !cgo || gtk3 || android || server

package platform

import "unsafe"

// GTK3 and non-Linux backends already have a platform transparency path, or
// cannot use the GTK4 CSS provider implemented here.
func makeWindowTransparent(_ unsafe.Pointer) {}
