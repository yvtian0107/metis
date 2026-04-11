## ADDED Requirements

### Requirement: Error key convention
All backend service-layer errors SHALL use a structured key format: `error.<domain>.<specific>`. The error key MUST be a valid i18n key string (lowercase, dots as separators). Existing sentinel errors (e.g., `ErrInvalidCredentials`, `ErrUserNotFound`) MUST be converted to this format.

#### Scenario: Auth error returns error key
- **WHEN** login fails due to wrong password
- **THEN** the API responds with `{"code": -1, "message": "error.auth.invalid_credentials"}`

#### Scenario: User service error returns error key
- **WHEN** creating a user with an existing username
- **THEN** the API responds with `{"code": -1, "message": "error.user.username_exists"}`

### Requirement: go-i18n integration for notifications
The backend SHALL integrate `go-i18n` v2 for translating notification/email content. Translation files are stored under `internal/locales/{locale}.json` for kernel, and `internal/app/<name>/locales/{locale}.json` for App modules. The translation service MUST be registered in the IOC container.

#### Scenario: Send email in user's preferred locale
- **WHEN** the notification channel sends an email to a user with `locale: "en"`
- **THEN** the email subject and body are rendered using the English translation templates

#### Scenario: Fallback to system locale for notification
- **WHEN** the target user has no locale preference
- **THEN** the notification uses the system default locale from SystemConfig

### Requirement: App interface Locales extension
The `app.App` interface SHALL support an optional `Locales() embed.FS` method for Apps to provide their own backend translation files. If an App does not need backend translations, it MAY return nil. The main bootstrap loop MUST load App locale files into the translation service.

#### Scenario: App provides backend translations
- **WHEN** the AI App implements `Locales()` returning an embed.FS with `locales/zh-CN.json` and `locales/en.json`
- **THEN** the translation service merges these translations under the App's namespace

#### Scenario: App without backend translations
- **WHEN** an App's `Locales()` returns nil
- **THEN** the translation service skips loading and no error occurs
