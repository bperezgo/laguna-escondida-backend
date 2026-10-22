# delivery-zones Specification

## Purpose
Defines the areas where the restaurant delivers, as polygons drawn by the staff, so the system can decide whether a customer's location is one the rider is able and allowed to reach before an order is accepted.

## Requirements

### Requirement: A delivery zone is a named polygon

A delivery zone SHALL have a name, a boundary given as a GeoJSON `Polygon` or `MultiPolygon` geometry (coordinates as `[longitude, latitude]`, as GeoJSON defines), and an active flag. A boundary SHALL be rejected with a validation error when it is not one of those two types, when a ring has fewer than four positions, when a ring's first and last positions differ, or when any coordinate is outside the longitude/latitude range. Holes (inner rings) SHALL be honored.

#### Scenario: A valid zone

- **WHEN** a zone "Centro" is created with a closed GeoJSON polygon
- **THEN** it is stored, active, and returned with its boundary unchanged

#### Scenario: An open ring

- **WHEN** a zone is created with a ring whose last position differs from its first
- **THEN** the request fails with a validation error and no zone is stored

#### Scenario: A coordinate out of range

- **WHEN** a zone is created with the position `[-74.08, 95]` (latitude 95)
- **THEN** the request fails with a validation error and no zone is stored

### Requirement: Containment decides whether a point is deliverable

A point SHALL be inside a zone when it lies within the zone's outer boundary and outside all its holes. When several active zones contain the point, the oldest-created one SHALL be the one recorded. Inactive and deleted zones SHALL never contain a point.

#### Scenario: Point in a hole

- **WHEN** a zone's polygon has a hole around a district the rider must not enter, and a point lies in that hole
- **THEN** the point is not inside the zone

#### Scenario: Inactive zone

- **WHEN** a point lies only inside a zone that has been deactivated
- **THEN** the point is not deliverable

### Requirement: Only admins and managers manage zones

Admins and managers SHALL be able to list, create, update (name, boundary, active flag) and delete zones. Deleting SHALL be a soft delete. Every other role, including `agent`, SHALL be refused with HTTP 403 on these operations. Checking whether a point is deliverable (next requirement) SHALL be available to any user allowed to create orders. Changing or deleting a zone SHALL NOT change orders already created in it.

#### Scenario: A manager redraws a zone

- **WHEN** a manager updates the boundary of zone "Centro"
- **THEN** later delivery orders are checked against the new boundary and existing orders keep their recorded zone

#### Scenario: The agent tries to edit a zone

- **WHEN** a user with only the `agent` role updates a zone
- **THEN** the request is refused with HTTP 403 and the zone is unchanged

### Requirement: A point can be checked without creating an order

The system SHALL answer, for a latitude and longitude, whether it lies in an active zone and which one, without creating anything.

#### Scenario: Checking a point

- **WHEN** a point inside zone "Centro" is checked
- **THEN** the answer is deliverable, zone "Centro"
