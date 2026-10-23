## ADDED Requirements

### Requirement: The overview SHALL summarise the user's shelf

The shelf overview SHALL show the number of owned games, games with zero plays ("shelf of shame"), plays recorded in the current year, and wishlist items for the authenticated user. Statistics SHALL update after sync, play logging, and collection edits.

#### Scenario: Fresh shelf
- **WHEN** a user with no synced games opens the overview
- **THEN** the page SHALL show zeroed statistics and a friendly empty state with a sync call to action

#### Scenario: Populated shelf
- **WHEN** a user has synced owned games and logged plays
- **THEN** the overview SHALL show accurate owned, unplayed, plays-this-year, and wishlist counts

### Requirement: Games SHALL be searchable and filterable

The shelf SHALL provide full-text search over game names, descriptions, designers, publishers, categories, and mechanics. It SHALL support filters for player count, maximum playtime, weight range, category, mechanic, and collection status (owned, wishlist, want to play, unplayed, favorite). Filters SHALL combine with search and with each other.

#### Scenario: Title search
- **WHEN** a user types a partial game title
- **THEN** matching games SHALL appear without a full page reload
- **AND** the result count SHALL be announced to assistive technology

#### Scenario: Player-count filter
- **WHEN** a user filters for games that support exactly 5 players
- **THEN** only games whose minimum and maximum player counts include 5 SHALL be listed

#### Scenario: Combined filters
- **WHEN** a user applies a category, a maximum playtime, and a search term
- **THEN** results SHALL satisfy every active condition

#### Scenario: Clear filters
- **WHEN** a user clears all filters
- **THEN** the full shelf SHALL be restored and all filter controls reset

#### Scenario: No results
- **WHEN** no game matches the active search and filters
- **THEN** the page SHALL show a no-results state with a clear-filters action
- **AND** the layout SHALL NOT collapse or jump

### Requirement: The shelf SHALL support sorting, pagination, and view density

Users SHALL be able to sort by name, year published, personal rating, BGG rating, weight, recently added, and most played. Results SHALL be paginated with a stable page size. Users SHALL be able to switch between a cover grid and a compact list, and the chosen density SHALL persist.

#### Scenario: Sort by weight
- **WHEN** a user selects "Weight: light to heavy"
- **THEN** games SHALL be ordered by ascending weight with unweighted games last

#### Scenario: Pagination
- **WHEN** more games match than fit on one page
- **THEN** the page SHALL offer navigation to further pages
- **AND** the current page SHALL remain after a reload

#### Scenario: Density persistence
- **WHEN** a user switches to the compact list view and reloads
- **THEN** the compact view SHALL remain selected

### Requirement: View state SHALL be represented in the URL

Search, filters, sort, density, and page SHALL be encoded in the query string so that views are shareable, reload-safe, and navigable with browser back and forward.

#### Scenario: Share a filtered view
- **WHEN** a user copies the URL of an active filtered view and opens it in another tab
- **THEN** the same search, filters, sort, density, and page SHALL be applied

#### Scenario: Back navigation
- **WHEN** a user changes a filter and then presses the browser back button
- **THEN** the previous filter state SHALL be restored

### Requirement: Each game SHALL have a detail page

The game detail page SHALL show the cover, title, year, description, player counts, playtime, minimum age, weight, BGG rank and rating, categories, mechanics, designers, publishers, a link to BoardGameGeek, the user's collection status, personal rating, notes, and play history.

#### Scenario: Game with metadata
- **WHEN** a user opens a synced game
- **THEN** all available metadata SHALL be rendered with readable labels
- **AND** missing fields SHALL be omitted without leaving broken layout

#### Scenario: BGG link
- **WHEN** a user activates the BoardGameGeek link
- **THEN** the BGG page for that game SHALL open in a new tab with safe link attributes

#### Scenario: Unknown game
- **WHEN** a user requests a game identifier that does not exist or is not on their shelf
- **THEN** the system SHALL return `404 Not Found`

### Requirement: Users SHALL edit personal collection state

Users SHALL be able to mark a game as owned, wishlist, want to play, preordered, for trade, or favorite; set a personal rating from 1 to 10 in half-point steps; write a free-text note; and record acquisition date, price paid, location, and condition. Personal changes SHALL persist and SHALL NOT modify BoardGameGeek.

#### Scenario: Favorite a game
- **WHEN** a user marks a game as favorite
- **THEN** the favorite state SHALL persist and the game SHALL appear in the favorite filter

#### Scenario: Set a rating
- **WHEN** a user selects a half-point rating
- **THEN** the rating SHALL be saved and reflected on the card and detail page

#### Scenario: Local-only change
- **WHEN** a user edits personal collection state
- **THEN** the system SHALL NOT attempt to write to BoardGameGeek
- **AND** the UI SHALL describe the change as local

### Requirement: The system SHALL present a shelf of shame

The shelf of shame SHALL list owned, non-expansion games with zero combined BGG and local plays. It SHALL show the total count and highlight the game that has been owned the longest without a recorded play.

#### Scenario: Unplayed filter
- **WHEN** a user selects the "Unplayed" filter
- **THEN** only owned, non-expansion games with zero plays SHALL be listed

#### Scenario: Longest-waiting game
- **WHEN** multiple unplayed games have acquisition dates
- **THEN** the overview SHALL highlight the one with the earliest acquisition date as "longest on the shelf"

#### Scenario: No unplayed games
- **WHEN** every owned game has at least one play
- **THEN** the shelf of shame SHALL show a celebratory empty state

### Requirement: The interface SHALL follow the warm editorial design system

The interface SHALL use the documented parchment/felt-green palette, Newsreader display type, DM Sans UI type, cover-forward cards, and consistent spacing, radii, and motion tokens. It SHALL support light and dark themes, respect `prefers-reduced-motion`, and remain usable from 320px to wide desktop without horizontal scrolling.

#### Scenario: Dark mode
- **WHEN** a user selects dark mode
- **THEN** every page SHALL use the dark palette
- **AND** the choice SHALL persist across sessions

#### Scenario: System theme
- **WHEN** a user has not chosen a theme and their operating system prefers dark
- **THEN** the application SHALL render in dark mode

#### Scenario: Reduced motion
- **WHEN** the operating system requests reduced motion
- **THEN** non-essential transitions SHALL be disabled

#### Scenario: Mobile layout
- **WHEN** the overview is viewed at 360px width
- **THEN** the grid SHALL reflow without horizontal scrolling
- **AND** primary actions SHALL remain reachable

### Requirement: The interface SHALL be accessible

Pages SHALL provide visible focus styles, semantic landmarks, labelled controls, alternative text for covers, keyboard-operable actions, and WCAG AA contrast. Actions available on hover SHALL also be available on keyboard focus or in the game detail page. Dynamic result updates SHALL be announced through an `aria-live` region.

#### Scenario: Keyboard navigation
- **WHEN** a user navigates the overview with only a keyboard
- **THEN** search, filters, cards, and view controls SHALL be operable with visible focus

#### Scenario: Screen-reader result count
- **WHEN** search results change
- **THEN** an assistive technology SHALL be informed of the new result count

#### Scenario: Cover alternative text
- **WHEN** a cover image is rendered
- **THEN** its alternative text SHALL identify the game
