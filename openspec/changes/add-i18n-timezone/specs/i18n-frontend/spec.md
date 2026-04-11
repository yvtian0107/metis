## ADDED Requirements

### Requirement: i18next initialization
The system SHALL initialize i18next with react-i18next at application startup, before any route renders. The initialization MUST configure: fallback language `zh-CN`, default namespace `common`, supported languages list `['zh-CN', 'en']`, and interpolation with escaping disabled (React handles XSS).

#### Scenario: App bootstraps with i18next ready
- **WHEN** the React application mounts
- **THEN** i18next is initialized and `useTranslation()` hook is available in all components

#### Scenario: Fallback to zh-CN when translation missing
- **WHEN** a translation key has no entry in the active language
- **THEN** the system falls back to the zh-CN translation for that key

### Requirement: Kernel translation file structure
The system SHALL organize kernel translations under `web/src/i18n/locales/{locale}/` with one JSON file per namespace. Namespaces MUST include: `common`, `auth`, `install`, `users`, `roles`, `menus`, `settings`, `tasks`, `audit`, `errors`. Each namespace corresponds to a functional area.

#### Scenario: Namespace maps to page area
- **WHEN** a component in `pages/users/` needs translated text
- **THEN** it uses `useTranslation('users')` to access the `users` namespace

### Requirement: App module translation registration
Each App module SHALL be able to register its own translations via a `registerTranslations(namespace, resources)` function. App translations are co-located at `web/src/apps/<name>/locales/{locale}/<name>.json`. Registration MUST happen in the App's `module.ts` alongside route registration.

#### Scenario: AI app registers its translations
- **WHEN** the AI app module is imported via registry
- **THEN** its translations are added to i18next under the `ai` namespace
- **AND** components in the AI app can use `useTranslation('ai')`

#### Scenario: App excluded by build does not register translations
- **WHEN** `APPS=system` is used (AI app excluded from registry)
- **THEN** the AI app's translations are not bundled or registered

### Requirement: Locale resolution priority
The system SHALL resolve the active locale in this order: (1) user's `locale` preference from auth store, (2) system default `system.locale` from site info API, (3) browser's `navigator.language`, (4) `zh-CN` as hardcoded fallback. If a resolved locale is not in the supported list, the system MUST fall to the next level.

#### Scenario: User has locale preference set
- **WHEN** the authenticated user has `locale: "en"` in their profile
- **THEN** the UI displays in English regardless of system default

#### Scenario: User has no preference, system default used
- **WHEN** the user's `locale` field is empty and system default is `zh-CN`
- **THEN** the UI displays in Chinese

#### Scenario: No user, no system default, browser fallback
- **WHEN** on the login page (no authenticated user) and system locale is empty
- **THEN** the system uses the browser's `navigator.language` if it matches a supported locale

### Requirement: Language switcher
The system SHALL provide a language switcher component. On unauthenticated pages (login, register, install), it MUST appear in the page header area. On authenticated pages, language preference is managed via user profile settings.

#### Scenario: Switch language on login page
- **WHEN** user selects "English" from the language switcher on the login page
- **THEN** the login page immediately re-renders in English
- **AND** the selected locale is stored in localStorage for persistence

### Requirement: Error key translation
The frontend SHALL maintain an `errors` namespace containing translations for all backend error keys. When displaying an API error message, the system MUST attempt `t('errors:<error_key>')`. If no translation is found, the raw error key is displayed as-is.

#### Scenario: Known error key translated
- **WHEN** the backend returns `{"message": "error.auth.invalid_credentials"}`
- **THEN** the UI displays "用户名或密码错误" (zh-CN) or "Invalid credentials" (en)

#### Scenario: Unknown error key displayed raw
- **WHEN** the backend returns an error key not in the translations
- **THEN** the UI displays the raw key string
