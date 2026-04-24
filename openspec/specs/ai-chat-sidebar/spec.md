# ai-chat-sidebar Specification

## Purpose
TBD - created by archiving change chat-ui-polish. Update Purpose after archive.
## Requirements
### Requirement: Date-grouped session list
The session sidebar SHALL group conversations by date categories: "Today", "Yesterday", "Last 7 Days", "Last 30 Days", and "Older". Each group SHALL display a translated section header. Empty groups SHALL be hidden.

#### Scenario: Sessions grouped by date
- **WHEN** the sidebar loads sessions with varying creation dates
- **THEN** sessions SHALL be organized under date group headers in reverse chronological order within each group

#### Scenario: Empty date groups hidden
- **WHEN** there are no sessions from "Yesterday"
- **THEN** the "Yesterday" group header SHALL NOT be displayed

#### Scenario: Date group labels are translated
- **WHEN** the locale is `zh-CN`
- **THEN** group headers SHALL display "今天", "昨天", "最近 7 天", "最近 30 天", "更早"

### Requirement: Session rename
The system SHALL allow users to rename conversations. Double-clicking a session title in the sidebar SHALL enter inline edit mode with an input field. Pressing Enter SHALL save the new title via `PATCH /api/v1/ai/sessions/:sid`. Pressing Escape SHALL cancel editing.

#### Scenario: Enter rename mode
- **WHEN** user double-clicks a session title in the sidebar
- **THEN** the title text SHALL become an editable input field pre-filled with the current title

#### Scenario: Save renamed title
- **WHEN** user presses Enter in the rename input
- **THEN** the system SHALL call `PATCH /api/v1/ai/sessions/:sid` with the new title and update the sidebar display

#### Scenario: Cancel rename
- **WHEN** user presses Escape during renaming
- **THEN** the input SHALL revert to the original title in read-only display

### Requirement: Session delete confirmation
The system SHALL require confirmation before deleting a session from the sidebar. A confirmation popover or dialog SHALL appear with the session title and "Delete" / "Cancel" actions.

#### Scenario: Delete with confirmation
- **WHEN** user clicks the delete button on a session in the sidebar
- **THEN** a confirmation prompt SHALL appear before the session is deleted

#### Scenario: Cancel delete
- **WHEN** user clicks "Cancel" in the delete confirmation
- **THEN** the session SHALL NOT be deleted and the confirmation SHALL close

### Requirement: Session context menu
The system SHALL provide a context menu (via "⋯" button on hover) for each session in the sidebar. The menu SHALL include: Rename, Pin/Unpin, Export, and Delete actions.

#### Scenario: Context menu on hover
- **WHEN** user hovers over a session item in the sidebar
- **THEN** a "⋯" more button SHALL appear at the right side of the session item

#### Scenario: Context menu actions
- **WHEN** user clicks the "⋯" button
- **THEN** a dropdown menu SHALL appear with Rename, Pin/Unpin, and Delete actions

### Requirement: Pinned sessions section
The system SHALL support pinning sessions to the top of the sidebar. Pinned sessions SHALL appear in a dedicated "Pinned" section above the date-grouped list. Pin state SHALL be persisted via the session API.

#### Scenario: Pin a session
- **WHEN** user selects "Pin" from the context menu
- **THEN** the session SHALL move to the "Pinned" section at the top of the sidebar

#### Scenario: Unpin a session
- **WHEN** user selects "Unpin" from a pinned session's context menu
- **THEN** the session SHALL return to its date-grouped position in the list

### Requirement: Reusable chat session sidebar
The system SHALL provide a reusable Chat Workspace session sidebar that can be configured for AI Management chat and ITSM Service Desk. The sidebar SHALL support the same base interaction model, selected state, loading state, empty state, date grouping where enabled, and new-session action.

#### Scenario: AI chat sidebar uses reusable component
- **WHEN** a user opens AI Management chat with a current agent
- **THEN** the session sidebar SHALL be rendered by the reusable Chat Workspace session sidebar
- **AND** AI-specific actions such as rename, delete, pin, export, or collapse SHALL be configured through sidebar capabilities

#### Scenario: ITSM service desk sidebar uses reusable component
- **WHEN** a user opens ITSM Service Desk
- **THEN** the service desk session list SHALL be rendered by the reusable Chat Workspace session sidebar
- **AND** it SHALL use the same selected state and density as AI Management chat while preserving ITSM labels

### Requirement: Sidebar behavior changes in one place
Changes to shared sidebar interaction or visual treatment SHALL be implemented in the reusable Chat Workspace session sidebar and apply to all configured chat surfaces.

#### Scenario: Selected style updated
- **WHEN** the selected session style is changed in the shared sidebar
- **THEN** AI Management chat and ITSM Service Desk SHALL both reflect the updated selected state without page-specific style changes

