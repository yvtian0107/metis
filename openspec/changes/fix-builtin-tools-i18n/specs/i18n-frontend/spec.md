## MODIFIED Requirements

### Requirement: App module translation registration
Each App module SHALL be able to register its own translations via a `registerTranslations(namespace, resources)` function. App translations are co-located at `web/src/apps/<name>/locales/{locale}/<name>.json`. Registration MUST happen in the App's `module.ts` alongside route registration.

The AI app's locale files (`web/src/apps/ai/locales/{zh-CN,en}.json`) SHALL contain i18n entries for **all** builtin tools registered in the `ai_tools` table, including those seeded by non-AI apps (ITSM, org, etc.). The required key patterns are:
- `tools.toolkits.<toolkit>.name` / `.description` — one entry per distinct `toolkit` value
- `tools.toolDefs.<tool.name>.name` / `.description` — one entry per tool

#### Scenario: AI app registers its translations
- **WHEN** the AI app module is imported via registry
- **THEN** its translations are added to i18next under the `ai` namespace
- **AND** components in the AI app can use `useTranslation('ai')`

#### Scenario: App excluded by build does not register translations
- **WHEN** `APPS=system` is used (AI app excluded from registry)
- **THEN** the AI app's translations are not bundled or registered

#### Scenario: All builtin toolkit groups display translated names
- **WHEN** the builtin tools tab renders a toolkit group whose `toolkit` value is `general`, `itsm`, `decision`, or `organization`
- **THEN** the toolkit name and description SHALL display the translated text from `tools.toolkits.<toolkit>.{name,description}`, not the raw i18n key

#### Scenario: All builtin tool definitions display translated names
- **WHEN** the builtin tools tab renders a tool whose `name` is any of the 24 registered builtin tools
- **THEN** the tool name and description SHALL display translated text from `tools.toolDefs.<name>.{name,description}` or fall back to the backend `displayName`/`description` field
