//go:build linux && cgo

package platform

/*
#cgo pkg-config: gtk4
#include <gtk/gtk.h>

static void cadence_resize_window(GtkWidget *widget, int width, int height) {
	// A size request forces GTK to allocate the new geometry even when the
	// window is already mapped. Setting the default size as well keeps the
	// next show/present at the requested geometry.
	gtk_widget_set_size_request(widget, width, height);
	gtk_window_set_default_size(GTK_WINDOW(widget), width, height);
	gtk_widget_queue_resize(widget);
}
*/
import "C"

import "unsafe"

func resizeWindow(handle unsafe.Pointer, width, height int) {
	C.cadence_resize_window((*C.GtkWidget)(handle), C.int(width), C.int(height))
}
