//go:build linux && cgo

package platform

/*
#cgo pkg-config: gtk4
#include <gtk/gtk.h>

static gboolean cadence_clear_size_request(gpointer data) {
	GtkWidget *widget = GTK_WIDGET(data);
	gtk_widget_set_size_request(widget, -1, -1);
	g_object_unref(widget);
	return G_SOURCE_REMOVE;
}

static void cadence_resize_window(GtkWidget *widget, int width, int height) {
	// A size request reliably resizes an already mapped GTK4 window. Keep it
	// briefly so the compositor latches the new geometry, then release it so
	// the user can resize the window freely again.
	gtk_widget_set_size_request(widget, width, height);
	gtk_window_set_default_size(GTK_WINDOW(widget), width, height);
	gtk_widget_queue_resize(widget);
	g_timeout_add(1000, cadence_clear_size_request, g_object_ref(widget));
}
*/
import "C"

import "unsafe"

func resizeWindow(handle unsafe.Pointer, width, height int) {
	C.cadence_resize_window((*C.GtkWidget)(handle), C.int(width), C.int(height))
}
