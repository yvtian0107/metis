## ADDED Requirements

### Requirement: Language and timezone settings section
The settings page SHALL include a "Language & Timezone" section (as a new tab or within the Site Info tab). It MUST allow admins to configure: system default locale (dropdown of supported locales) and system default timezone (searchable timezone selector with IANA identifiers).

#### Scenario: Admin changes system locale
- **WHEN** the admin selects "English" as the system locale and saves
- **THEN** `system.locale` is updated to `"en"` in SystemConfig
- **AND** new sessions without user-level override will display in English

#### Scenario: Admin changes system timezone
- **WHEN** the admin selects "America/New_York" as the system timezone and saves
- **THEN** `system.timezone` is updated to `"America/New_York"` in SystemConfig

### Requirement: All settings page text is translatable
All user-facing text in the settings page SHALL use i18n translation keys from the `settings` namespace. Tab labels, section titles, field labels, help text, and button labels MUST NOT contain hardcoded text.

#### Scenario: Settings page displays in English
- **WHEN** the active locale is `en`
- **THEN** tab label shows "Site Info" instead of "站点信息", "Security" instead of "安全设置"
