package handlers

import (
	"advanced-purge/purge"

	"github.com/disgoorg/disgo/handler"
)

func NewHandler() *Handler {
	mux := handler.New()
	handlers := &Handler{
		controller: purge.NewController(),
		Router:     mux,
	}

	mux.SlashCommand("/purge", handlers.HandlePurge)
	mux.Group(func(r handler.Router) {
		r.Use(handlers.MiddlewareButtonUser())

		r.Route("/purge", func(r handler.Router) {
			r.ButtonComponent("/simple", handlers.HandleSimple)
			r.ButtonComponent("/advanced", handlers.HandleAdvanced)
			r.ButtonComponent("/cancel", handlers.HandleCancel)

			r.Route("/start-change", func(r handler.Router) {
				r.ButtonComponent("/keep", handlers.HandleStartKeep)
				r.ButtonComponent("/{new-id}", handlers.HandleStartChange)
			})
			r.Route("/end-change", func(r handler.Router) {
				r.ButtonComponent("/keep", handlers.HandleEndKeep)
				r.ButtonComponent("/{new-id}", handlers.HandleEndChange)
			})

			r.ButtonComponent("/run", handlers.HandleRun)
		})

		r.MessageCommand("/Set as start", handlers.HandleStart)
		r.MessageCommand("/Set as end", handlers.HandleEnd)

		r.MessageCommand("/Exclude message", handlers.HandleExclude)
		r.MessageCommand("/Include message", handlers.HandleInclude)

	})
	mux.Modal("/purge", handlers.HandleLimit)
	return handlers
}

type Handler struct {
	controller *purge.Controller
	handler.Router
}
