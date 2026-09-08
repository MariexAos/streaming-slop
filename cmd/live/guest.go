package main

import (
	"context"
	"net/http"

	"streaming-agent/internal/monitoring"
	httpapi "streaming-agent/internal/server"
	"streaming-agent/internal/server/webui"
)

func serveGuests(ctx context.Context, console http.Handler, hub *monitoring.Hub, controller bilibiliConfigController) error {
	guest := httpapi.GuestHandler(hub, controller, console, webui.NewHandler())
	return serve(ctx, console, guest, controller.runtime)
}
