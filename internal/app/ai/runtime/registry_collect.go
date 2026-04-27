package runtime

import (
	"log/slog"

	"github.com/samber/do/v2"

	"metis/internal/app"
)

func collectToolRegistries(i do.Injector) []ToolHandlerRegistry {
	generalReg := do.MustInvoke[*GeneralToolRegistry](i)
	knowledgeReg := do.MustInvoke[*KnowledgeToolRegistry](i)

	var registries []ToolHandlerRegistry
	registries = append(registries, generalReg)
	registries = append(registries, knowledgeReg)

	for _, a := range app.All() {
		trp, ok := a.(app.ToolRegistryProvider)
		if !ok {
			continue
		}
		raw := trp.GetToolRegistry()
		if reg, ok := raw.(ToolHandlerRegistry); ok {
			registries = append(registries, reg)
			slog.Info("AI: discovered tool registry", "app", a.Name())
		}
	}

	return registries
}

func collectRuntimeContextProviders() []app.AgentRuntimeContextProvider {
	var providers []app.AgentRuntimeContextProvider
	for _, a := range app.All() {
		provider, ok := a.(app.AgentRuntimeContextProvider)
		if !ok {
			continue
		}
		providers = append(providers, provider)
		slog.Info("AI: discovered runtime context provider", "app", a.Name())
	}
	return providers
}
