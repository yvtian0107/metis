## ADDED Requirements

### Requirement: Install-time admin identity supports built-in smart approvals
Fresh installation SHALL assign the created default admin account an organization identity that makes the built-in smart ITSM services testable immediately after install. At minimum, the admin SHALL be assigned to the built-in department/position combination required to act in the seeded approval paths intended for manual verification.

#### Scenario: Fresh install admin can act on built-in smart approvals
- **WHEN** installation completes with the default admin account
- **THEN** that admin account SHALL already hold the built-in org identity needed to operate the intended seeded smart approval path(s)

#### Scenario: Install-time assignment is additive
- **WHEN** the install flow creates the admin account and applies the default org identity
- **THEN** the system SHALL add the required department/position assignment without weakening role-based permissions or mutating unrelated seed data

#### Scenario: Incremental sync does not overwrite existing admin org identity
- **WHEN** `seed.Sync()` runs on a subsequent startup after admins have been manually adjusted
- **THEN** the system SHALL NOT overwrite existing admin department/position assignments
