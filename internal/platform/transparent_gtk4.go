//go:build linux && cgo && !gtk3 && !android && !server

package platform

/*
#cgo pkg-config: gtk4
#include <gtk/gtk.h>

static void cadence_make_window_transparent(void *ptr) {
	GtkWidget *window = GTK_WIDGET(ptr);
	GtkCssProvider *provider = gtk_css_provider_new();
	gtk_css_provider_load_from_string(provider,
		"window, window.background, #webview-box {"
		" background-color: rgba(0, 0, 0, 0);"
		" background-image: none;"
		" box-shadow: none;"
		"}");
	gtk_style_context_add_provider_for_display(
		gdk_display_get_default(),
		GTK_STYLE_PROVIDER(provider),
		GTK_STYLE_PROVIDER_PRIORITY_USER);
	g_object_unref(provider);
	gtk_widget_add_css_class(window, "cadence-transparent");
	gtk_widget_queue_draw(window);
}
*/
import "C"

import "unsafe"

func makeWindowTransparent(handle unsafe.Pointer) {
	if handle == nil {
		return
	}
	C.cadence_make_window_transparent(handle)
}
