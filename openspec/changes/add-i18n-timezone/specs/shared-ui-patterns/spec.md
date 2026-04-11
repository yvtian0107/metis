## MODIFIED Requirements

### Requirement: Empty table states are translatable
Empty table state messages (icon + title + description) SHALL use translation keys instead of hardcoded Chinese. Each list page's empty state uses its own namespace (e.g., `t('users.empty.title')`, `t('users.empty.description')`).

#### Scenario: Users empty state in English
- **WHEN** the users page has no data and locale is `en`
- **THEN** the empty state shows "No Users" and "Click 'New User' to add the first user"

#### Scenario: Users empty state in Chinese
- **WHEN** the users page has no data and locale is `zh-CN`
- **THEN** the empty state shows "暂无用户" and "点击「新建用户」添加第一个用户"

## ADDED Requirements

### Requirement: Common UI vocabulary namespace
The `common` translation namespace SHALL contain shared UI vocabulary used across multiple pages: button labels (save, cancel, delete, edit, create, search, confirm, close, enable, disable), status words (active, inactive, loading), confirmation dialog text (delete confirmation template), pagination labels, and form validation messages.

#### Scenario: Save button uses common namespace
- **WHEN** any page renders a save button
- **THEN** it uses `t('common:save')` which resolves to "保存" (zh-CN) or "Save" (en)

#### Scenario: Delete confirmation uses common namespace with interpolation
- **WHEN** a delete confirmation dialog shows for user "admin"
- **THEN** it uses `t('common:deleteConfirm', { name: 'admin' })` which resolves to "确定要删除 \"admin\" 吗？此操作不可撤销。" or "Are you sure you want to delete \"admin\"? This action cannot be undone."

### Requirement: Loading and saving state labels are translatable
All loading indicators (e.g., "保存中...", "加载中...", "删除中...", "登录中...") SHALL use translation keys from the `common` namespace.

#### Scenario: Saving state in English
- **WHEN** a form is submitting and locale is `en`
- **THEN** the button shows "Saving..." instead of "保存中..."
