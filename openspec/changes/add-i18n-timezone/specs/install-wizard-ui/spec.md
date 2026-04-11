## MODIFIED Requirements

### Requirement: Install wizard step progression
The install wizard SHALL display a multi-step progress indicator. Steps are: **Language & Timezone** → Database → Site Info → Admin → Complete. The step indicator MUST show the current step as active, completed steps with a checkmark, and future steps as dimmed. Step labels MUST be translated using the active locale.

#### Scenario: Step indicator shows 5 steps
- **WHEN** the install wizard loads
- **THEN** the step indicator shows 5 steps: Language, Database, Site Info, Admin, Complete

#### Scenario: Language step is first and active on initial load
- **WHEN** the user opens `/install` for the first time
- **THEN** the first step "Language & Timezone" is active

## ADDED Requirements

### Requirement: Language and timezone selection step
The install wizard SHALL have a first step for selecting the system language and timezone. This step is purely frontend — it does not require database connectivity. The language selector MUST list all supported locales with their native names (e.g., "简体中文", "English"). The timezone selector MUST default to the browser's detected timezone and allow selection from a list of IANA timezones grouped by region.

#### Scenario: Language selection changes wizard language
- **WHEN** the user selects "English" in the language step
- **THEN** all subsequent wizard steps display in English immediately

#### Scenario: Timezone defaults to browser timezone
- **WHEN** the language step loads and the browser reports `Asia/Shanghai`
- **THEN** the timezone selector defaults to `Asia/Shanghai`

#### Scenario: Proceed to database step
- **WHEN** the user clicks "Next" on the language step
- **THEN** the wizard advances to the database selection step with the chosen language active

### Requirement: All install wizard text is translatable
All user-facing text in the install wizard (labels, placeholders, helper text, error messages, button text) SHALL use i18n translation keys from the `install` namespace. No hardcoded Chinese or English text in the install wizard components.

#### Scenario: Database step displays in selected language
- **WHEN** the user selected "English" in step 1 and is on the database step
- **THEN** "SQLite" description shows "Zero-config, suitable for small deployments" instead of "零配置，适合小型部署"
