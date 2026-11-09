# Feature Specification: Chroma UI and Theme Selection

**Feature Branch**: `master` (no feature branch created)

**Created**: 2026-10-29

**Status**: Draft

**Input**: Use [Chroma UI](https://github.com/githubixx/chroma-ui) for the UI, with [vdradmin-go](https://github.com/githubixx/vdradmin-go) as an implementation reference. Support its themes and provide a "Theme" dropdown in Configuration for selecting available themes.

## Clarifications

### Session 2026-10-29

- Q: Who should a saved theme selection apply to? → A: One administrator-selected theme applies to the whole deployment.
- Q: Should the Theme dropdown include a System option that follows each visitor's device light/dark setting? → A: Include System alongside bundled themes.
- Q: Which appearance should existing installations use when no theme has been saved? → A: System, based on each visitor's device.
- Q: If a saved theme is no longer available, should administrators have to choose a replacement before saving other configuration changes? → A: Require a valid replacement before saving any changes.
- Q: What minimum participant sample should the usability review require? → A: At least one administrator and one user; an administrator may participate in both groups.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Use the refreshed interface (Priority: P1)

A visitor or administrator uses Bookmarker through a consistent Chroma UI-based interface while continuing to perform the same tasks and respecting existing access restrictions.

**Why this priority**: The application-wide interface is the foundation for applying themes without losing existing functionality.

**Independent Test**: Open the application with an existing configuration and exercise navigation, sign-in, listing and searching bookmarks, and authorized bookmark and configuration operations at desktop and mobile widths.

**Acceptance Scenarios**:

1. **Given** an existing installation without a theme setting, **When** a visitor opens Bookmarker, **Then** the interface follows the visitor's device appearance and existing bookmark viewing and search remain usable.
2. **Given** an administrator is signed in, **When** they navigate to and use the existing add, edit, delete, capture, and configuration workflows, **Then** the workflows, feedback, and access controls continue to function.
3. **Given** a visitor or administrator uses a narrow screen or keyboard navigation, **When** they navigate the interface, **Then** navigation and controls remain visible, operable, and labeled.

---

### User Story 2 - Select and apply a theme (Priority: P1)

An administrator chooses an available theme in Configuration and saves it so everyone sees the chosen appearance.

**Why this priority**: Theme choice is the requested user-facing capability and makes the refreshed interface adaptable.

**Independent Test**: Sign in, select two different available themes in turn, save each, and verify the resulting appearance and selected value on the current page and after reopening the application.

**Acceptance Scenarios**:

1. **Given** an administrator opens Configuration, **When** the form is displayed, **Then** a labeled "Theme" dropdown lists all currently available bundled Chroma UI themes plus System and identifies the active selection.
2. **Given** a valid different theme is selected, **When** the administrator saves Configuration successfully, **Then** the selected appearance is applied to the whole current page without an application restart and the saved selection is shown when Configuration is reopened.
3. **Given** a theme has been saved, **When** another visitor loads or reloads a page or the application restarts, **Then** that page uses the saved deployment-wide theme.
4. **Given** System is saved, **When** a visitor's device uses a light or dark appearance or changes between them, **Then** the page follows that preference without changing the saved selection.

---

### User Story 3 - Recover safely from invalid changes (Priority: P2)

An administrator can recognize and correct an unavailable theme or failed configuration save without losing the last working appearance or other settings.

**Why this priority**: Invalid values and save failures must not make Bookmarker unusable or silently alter unrelated settings.

**Independent Test**: Submit an unavailable theme and simulate a failed configuration save, then verify the error, active appearance, and persisted settings.

**Acceptance Scenarios**:

1. **Given** an unavailable or tampered theme value is submitted, **When** the administrator saves, **Then** the save is rejected with a useful validation message and all previously saved settings remain unchanged.
2. **Given** a valid theme is selected but the configuration cannot be saved, **When** the save fails, **Then** the administrator sees an error and the prior saved theme remains active.
3. **Given** a previously saved theme is no longer available, **When** a page is opened, **Then** it remains usable with the default appearance, and Configuration identifies the unavailable saved selection and allows the administrator to replace it.

### Edge Cases

- An installation upgraded from a configuration without a theme uses the default without requiring an immediate configuration edit; saving another setting preserves the effective default.
- A previously saved selection absent from a newer catalog falls back visually without silently overwriting the saved value; configuration saves are blocked until an explicit valid replacement is chosen.
- A failed or invalid combined configuration submission changes neither the theme nor other saved settings.
- A theme saved from a form that also changes database settings takes effect immediately, while existing database restart requirements remain in force.
- Other open browser pages need not change in real time; they adopt the saved selection on their next full page load or reload.
- If theme artwork or external fonts are unavailable, content and controls remain readable and usable with fallback presentation.
- Unauthorized visitors cannot read or change Configuration, including its theme setting.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Bookmarker MUST use Chroma UI as the shared visual foundation for its web interface, with vdradmin-go available as a reference rather than as a replacement for Bookmarker's workflows or branding.
- **FR-002**: The shared appearance MUST cover the application shell, navigation, sign-in, bookmark list, add, edit, delete, capture and search workflows, Configuration, and their feedback, empty, and error states.
- **FR-003**: The refreshed interface MUST preserve existing bookmark, search, capture, authentication, authorization, configuration, navigation, and feedback behavior, including the distinction between visitor and administrator capabilities.
- **FR-004**: Configuration MUST provide an accessible control labeled "Theme" that lists each available bundled Chroma UI theme by a distinguishable name and also offers System. The options MUST reflect the bundled catalog actually available in the delivered application, rather than a separately maintained fixed list.
- **FR-005**: Configuration MUST display the saved valid selection and permit an authorized administrator to select and save a different available option using the existing configuration save workflow.
- **FR-006**: A successful theme save MUST apply the selected appearance to the entire current page without restarting the application; subsequent full page loads and reloads for all visitors MUST use that deployment-wide selection.
- **FR-007**: The selected theme MUST persist across application restarts. A configuration with no theme setting MUST remain valid and use System by default.
- **FR-008**: System MUST follow the visitor's device light/dark preference, including changes while the page is open, without altering the deployment-wide saved selection. This also applies when System is the default or fallback appearance.
- **FR-009**: An unrecognized or unavailable theme submitted as a configuration change MUST be rejected with an actionable error; no part of that configuration submission may become active or be persisted.
- **FR-010**: If a persisted theme is no longer available, Bookmarker MUST render using the default theme without overwriting the persisted selection, identify the unavailable selection in Configuration, and require an administrator to select a valid replacement before saving any configuration change.
- **FR-011**: A failed configuration save MUST leave the previously active and persisted configuration unchanged and inform the administrator that the change was not saved.
- **FR-012**: The interface MUST retain usable, labeled controls, visible keyboard focus, readable content and functional navigation on desktop and narrow mobile screens for every available theme and System mode.
- **FR-013**: Existing configuration fields, validation, redaction, safe persistence, and restart notifications MUST remain effective. Updating other settings without changing an available theme MUST preserve its saved selection.
- **FR-014**: Pages MUST remain readable and controls usable when optional theme artwork or external fonts cannot load.
- **FR-015**: Existing configuration interfaces other than the web form MUST continue to handle configurations with a theme without unintentionally clearing or corrupting the selection when modifying other fields; an unavailable saved theme MUST be replaced with a valid choice before any change is saved.

### Key Entities

- **Theme option**: A bundled Chroma UI theme with a stable identifier and distinguishable display name, or the System selection mode, which follows each visitor's device appearance.
- **Deployment configuration**: The shared collection of application settings, including the saved theme selection; it is managed by an authorized administrator and applies to all visitors.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In an acceptance test with at least one administrator, at least 90% of participating administrators can locate the Theme dropdown, choose an option, and save it within 60 seconds on their first attempt.
- **SC-002**: For 100% of available bundled themes and System, a successful save changes the entire current page's appearance and the saved selection remains effective after a reload and an application restart.
- **SC-003**: In regression acceptance tests, 100% of the existing primary journeys—sign-in, navigation, listing, search, add, edit, delete, capture, and configuration—remain completable for their authorized actors at desktop and mobile widths.
- **SC-004**: In tests of invalid selections and simulated persistence failures, 100% of attempts display an error and preserve the last valid configuration.
- **SC-005**: In a keyboard-only acceptance pass, 100% of primary navigation items and Configuration controls are reachable, visibly focused, labeled, and operable in each available theme and System mode.
- **SC-006**: In a review with at least one user, at least 80% of participating users report that the refreshed interface is readable and that navigation and controls are easy to identify across the themes they try.

## Assumptions

- Individual user preferences, preview-before-save, and automatic live updates to other open browser pages are out of scope.
- The delivered Chroma UI bundled catalog defines available themes. System is an additional selection mode, not a catalog theme. Custom theme authoring, installation, and local overrides are out of scope.
- Existing configuration authorization and security policies remain unchanged. Theme selection does not grant additional access.
- The Chroma UI project is the requested dependency and vdradmin-go is a reference for adapting it, not a requirement to reproduce its product-specific pages or features. Dependency selection and architecture are planning decisions subject to the project constitution.
- Primary desktop and mobile acceptance tests use representative supported browser viewport sizes; theme artwork and fonts are optional to basic operation.
