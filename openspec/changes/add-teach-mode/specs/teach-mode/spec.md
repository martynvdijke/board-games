## ADDED Requirements

### Requirement: Teach script authoring
A per-game guide SHALL support an ordered teach script with a duration per step.

#### Scenario: Adding steps
- **WHEN** a user edits a game's guide
- **THEN** they can add, reorder, edit and remove teach steps, each with a title, body text and an estimated number of minutes

#### Scenario: Existing guides keep working
- **WHEN** a guide has no teach steps
- **THEN** it renders exactly as before and no teach entry point is offered

#### Scenario: Reordering
- **WHEN** a user reorders steps
- **THEN** the new order is persisted atomically and the total teach time is updated

### Requirement: Teach mode presentation
The system SHALL present the teach script as a stepwise view suitable for use at the table.

#### Scenario: Stepwise presentation
- **WHEN** a user opens teach mode for a game with steps
- **THEN** each step is shown with its body and photos, the current step is indicated, and the total remaining time is displayed

#### Scenario: Timer survives a reload
- **WHEN** the page is reloaded mid-teach
- **THEN** elapsed time is restored and the current step is preserved

#### Scenario: Reduced motion and screen readers
- **WHEN** the user prefers reduced motion or uses a screen reader
- **THEN** no motion-dependent feedback is required to use the view and remaining time is announced at a coarse interval rather than every second

#### Scenario: JavaScript unavailable
- **WHEN** the teach view is loaded without JavaScript
- **THEN** all steps and their durations remain readable and no control is unusable

### Requirement: Printable quick reference
The system SHALL provide a print stylesheet that produces a legible black-and-white quick reference from the same steps.

#### Scenario: Printing
- **WHEN** a user prints the teach reference
- **THEN** navigation and interactive controls are omitted, text renders black on white at readable size, and steps do not split awkwardly across pages

#### Scenario: Photos in print
- **WHEN** the guide contains photos
- **THEN** printing keeps photos legible or omits them consistently rather than rendering unreadable thumbnails

### Requirement: Per-game entry point
Each game SHALL have a stable entry point to its teach view usable from a printed code.

#### Scenario: QR code
- **WHEN** a user requests a QR code for a game's teach view
- **THEN** the code encodes the canonical absolute URL for that game built from the instance base URL and contains no credential or token

#### Scenario: Unauthenticated scan
- **WHEN** the teach URL is opened by someone not signed in
- **THEN** they are sent to sign in and the teach content is not disclosed

#### Scenario: Teach time recorded
- **WHEN** a teach run is completed
- **THEN** the elapsed minutes are stored for that game and used in preference to inferred estimates afterwards
