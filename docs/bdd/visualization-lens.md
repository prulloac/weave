# BDD Test Cases: Visualization Lens

## Feature: Physics-based graph visualization

### Scenario: User navigates to the lens page and sees the graph

**Given** the user has a mounted OKF bundle with concepts
**When** the user navigates to `/lens`
**Then** the page title "Visualization Lens" is visible
**And** a canvas element is rendered

### Scenario: User sees nodes colored by concept type

**Given** the user is on the lens page
**When** the graph renders
**Then** the legend displays the type-to-color mapping
**And** at least one legend entry is visible

### Scenario: User sees edges represented in the legend

**Given** the user is on the lens page
**When** the graph renders
**Then** the legend shows a "Link" edge style
**And** the legend shows a "Related" edge style

### Scenario: User can drag a node

**Given** the user is on the lens page with a rendered graph
**When** the user presses and holds on the canvas center
**And** drags to a new position
**Then** the interaction completes without error

### Scenario: User can reheat the simulation

**Given** the user is on the lens page with a frozen simulation
**When** the user clicks the "Reheat" button
**Then** the button is visible and clickable

### Scenario: Legend shows all node types from the bundle

**Given** the user is on the lens page
**When** the legend renders
**Then** at least one node type entry is displayed
