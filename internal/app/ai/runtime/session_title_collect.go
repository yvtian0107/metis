package runtime

import (
	"log/slog"

	"metis/internal/app"
)

func collectSessionTitleProviders() []app.SessionTitleProvider {
	var providers []app.SessionTitleProvider
	for _, a := range app.All() {
		provider, ok := a.(app.SessionTitleProvider)
		if !ok {
			continue
		}
		providers = append(providers, provider)
		slog.Info("AI: discovered session title provider", "app", a.Name())
	}
	return providers
}
