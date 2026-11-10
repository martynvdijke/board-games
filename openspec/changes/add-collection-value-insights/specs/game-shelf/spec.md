## ADDED Requirements

### Requirement: Pipeline navigation
The shelf SHALL offer the acquisition pipeline as a distinct view reachable from the overview.

#### Scenario: Reachable from the shelf
- **WHEN** a signed-in user is on the collection overview
- **THEN** a link to the pipeline view is present and keyboard reachable

#### Scenario: Existing wishlist filter still works
- **WHEN** a user follows an existing filtered URL such as `?state=wishlist`
- **THEN** the collection renders the same filtered result as before the pipeline existed

### Requirement: Value-aware presentation
The game detail page and the collection list SHALL surface acquisition fields and allow sorting by cost metrics.

#### Scenario: Detail page fields
- **WHEN** a game detail page is rendered
- **THEN** acquisition date, price paid, location and condition are shown when recorded, and an edit affordance is present

#### Scenario: Sorting by cost
- **WHEN** a user sorts the collection by cost per play or cost per hour
- **THEN** games with a defined value are ordered by that value and games without one appear last

#### Scenario: Price entry
- **WHEN** a user saves a price typed as a decimal amount
- **THEN** it is stored as integer minor units and re-rendered as the same decimal amount
