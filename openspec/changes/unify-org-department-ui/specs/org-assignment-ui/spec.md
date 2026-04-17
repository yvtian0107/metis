## REMOVED Requirements

### Requirement: Personnel assignment page with department-centric view
**Reason**: The standalone `/org/assignments` page is replaced by the department detail page (`/org/departments/:id`). All member management functionality (view members, add/remove, edit positions, view org info) is now available within the department detail page's member section.
**Migration**: Navigate to `/org/departments/:id` to manage members for a specific department. The `/org/assignments` route SHALL redirect to `/org/departments` for backward compatibility.

### Requirement: Add and remove members from a department
**Reason**: This functionality is moved to the department detail page's member section. The AddMemberSheet and removal flow are identical but triggered from the detail page instead of the standalone assignments page.
**Migration**: Use the "Add Member" button and member row actions on the department detail page at `/org/departments/:id`.

### Requirement: Toggle primary position in member list
**Reason**: Primary position management is preserved in the department detail page's member section via the EditPositionsSheet (edit positions action in member row dropdown).
**Migration**: Use "Edit Positions" action on member rows in the department detail page.
