## REMOVED Requirements

### Requirement: Hand-rolled ReAct loop in smart_react.go
**Reason**: Replaced by `DecisionExecutor` interface. The SmartEngine no longer builds its own `llm.Client` or manages a ReAct tool-calling loop internally. This logic moves to the AI App's `DecisionExecutor` implementation.
**Migration**: All call sites that previously relied on `agenticDecision` performing its own LLM calls now delegate to `DecisionExecutor.Execute()`. The `buildInitialSeed` and `buildAgenticSystemPrompt` helper functions remain in the engine package for constructing the seed messages.
