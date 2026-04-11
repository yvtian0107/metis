## MODIFIED Requirements

### Requirement: Login page text is translatable
All user-facing text on the login page SHALL use translation keys from the `auth` namespace. This includes: page title, subtitle, input labels, input placeholders, button labels, error messages, links (register, forgot password), and OAuth provider labels.

#### Scenario: Login page in English
- **WHEN** the active locale is `en`
- **THEN** the login page shows "Sign in to {appName}", "Enter your credentials to continue", "Username", "Password", "Continue", "Don't have an account? Create one"

#### Scenario: Login page in Chinese
- **WHEN** the active locale is `zh-CN`
- **THEN** the login page shows "登录到 {appName}", "输入你的账号继续", "用户名", "密码", "继续", "没有账号？创建账号"

### Requirement: Registration page text is translatable
All user-facing text on the registration page SHALL use translation keys from the `auth` namespace.

#### Scenario: Registration page in English
- **WHEN** the active locale is `en`
- **THEN** the page shows "Create Account", "Username", "Email (optional)", "Password", "Confirm Password", "Register", "Already have an account? Sign in"

### Requirement: Two-factor authentication text is translatable
All 2FA-related text (verification page, setup dialog, backup codes display) SHALL use translation keys from the `auth` namespace.

#### Scenario: 2FA verification page in English
- **WHEN** the active locale is `en` and user is on `/2fa`
- **THEN** the page shows "Two-Factor Authentication", "Enter the 6-digit code from your authenticator app", "Use recovery code" toggle

### Requirement: Change password dialog text is translatable
All text in the change password dialog SHALL use translation keys from the `auth` namespace.

#### Scenario: Change password dialog in English
- **WHEN** the active locale is `en` and the change password dialog opens
- **THEN** it shows "Change Password", "Current Password", "New Password", "Confirm New Password", "Save"

## ADDED Requirements

### Requirement: Language switcher on unauthenticated pages
The login, registration, and 2FA pages SHALL display a language switcher (e.g., a dropdown or toggle) that allows users to switch the UI language before authenticating. The selected locale MUST be persisted in localStorage so it survives page refreshes.

#### Scenario: Switch to English on login page
- **WHEN** a user selects "English" from the language switcher on the login page
- **THEN** the login page immediately re-renders in English
- **AND** the locale choice is saved to localStorage
- **AND** subsequent visits to the login page default to English
