# Delivery Zones API

Define the areas where the restaurant delivers. A `delivery` order is accepted only when its pin falls inside an **active** zone (see [Orders → Order types](./orders.md#order-types-dine-in-takeaway-and-delivery)). No active zone means no delivery is accepted.

**Cloud only.** These routes exist only in the cloud binary; on the edge binary they return `404`.

## Endpoints

| Method | Endpoint                         | Description                          | Permission               |
| ------ | -------------------------------- | ------------------------------------ | ------------------------ |
| GET    | `/api/delivery-zones`            | List zones (active and inactive)     | `delivery-zones:read`    |
| POST   | `/api/delivery-zones`            | Create a zone                        | `delivery-zones:manage`  |
| PUT    | `/api/delivery-zones/:id`        | Update name, boundary or active flag | `delivery-zones:manage`  |
| DELETE | `/api/delivery-zones/:id`        | Delete (soft) a zone                 | `delivery-zones:manage`  |
| GET    | `/api/delivery-zones/check`      | Is a point deliverable, and where?   | `orders:create`          |

`delivery-zones:read` and `delivery-zones:manage` are granted to **admin** and **manager**. Every other role, including `agent`, gets `403` on list/create/update/delete. Checking a point is open to anyone who can create orders.

## Boundary format (GeoJSON)

`boundary` is a GeoJSON **geometry** object, either a `Polygon` or a `MultiPolygon`:

```json
{
  "type": "Polygon",
  "coordinates": [
    [[-74.10, 4.58], [-74.06, 4.58], [-74.06, 4.62], [-74.10, 4.62], [-74.10, 4.58]],
    [[-74.085, 4.595], [-74.075, 4.595], [-74.075, 4.605], [-74.085, 4.605], [-74.085, 4.595]]
  ]
}
```

- The first ring is the outer boundary; any further ring is a **hole** (an area inside the zone the rider must not enter). A point in a hole is not deliverable.
- A `MultiPolygon` is several such polygons; a point inside any of them (and outside its holes) is deliverable.
- Every ring needs **at least 4 positions** and must be **closed**: its last position equals its first.

> ⚠️ **Positions are `[longitude, latitude]`**, the GeoJSON order — the opposite of how maps usually show them. Bogotá is about `[-74.08, 4.60]`, not `[4.60, -74.08]`. Out-of-range values (latitude beyond ±90, longitude beyond ±180) are rejected, but a swapped pair is often still in range and silently puts the zone somewhere else on Earth, refusing every delivery. **After uploading a zone, always check it with the restaurant's own location** (see [Check a Point](#check-a-point)).

### Drawing a zone in geojson.io

1. Open <https://geojson.io>, find the area, and draw it with the polygon tool (click the corners, then click the first corner to close it).
2. To cut out a district, draw it as a second polygon and move its coordinates into the first polygon's `coordinates` array as an extra ring.
3. In the JSON panel on the right, copy the `geometry` object of the feature (the `{"type": "Polygon", "coordinates": ...}` part) — **not** the whole `FeatureCollection`.
4. Send it as `boundary` in `POST /api/delivery-zones`, then check the restaurant's own pin.

Zones drawn in Google My Maps or QGIS work too: export to KML, convert to GeoJSON, and take the geometry.

---

## List Zones

`GET /api/delivery-zones`

Returns every non-deleted zone, oldest first.

### Example Response (200 OK)

```json
{
  "delivery_zones": [
    {
      "id": "019a0f00-1111-7000-8000-00000000c3e7",
      "name": "Centro",
      "boundary": {
        "type": "Polygon",
        "coordinates": [[[-74.10, 4.58], [-74.06, 4.58], [-74.06, 4.62], [-74.10, 4.62], [-74.10, 4.58]]]
      },
      "active": true,
      "created_at": "2026-10-15T09:00:00Z",
      "updated_at": "2026-10-15T09:00:00Z"
    }
  ]
}
```

---

## Create Zone

`POST /api/delivery-zones`

### Request Body

| Field    | Type    | Required | Description |
| -------- | ------- | -------- | ----------- |
| name     | string  | Yes      | 1–100 characters |
| boundary | object  | Yes      | GeoJSON `Polygon` or `MultiPolygon` geometry, `[lng, lat]` positions |
| active   | boolean | No       | Defaults to `true` |

### Example Request

```json
{
  "name": "Centro",
  "boundary": {
    "type": "Polygon",
    "coordinates": [[[-74.10, 4.58], [-74.06, 4.58], [-74.06, 4.62], [-74.10, 4.62], [-74.10, 4.58]]]
  }
}
```

### Example Response (201 Created)

The created zone, same shape as a list item.

### Error Responses

**400 Bad Request** — invalid name or boundary

```json
{
  "error": "boundary must be a GeoJSON Polygon or MultiPolygon with [lng, lat] positions: a ring's first and last positions must be equal",
  "code": "DELIVERY_ZONE_INVALID_BOUNDARY"
}
```

| `code`                           | When |
| -------------------------------- | ---- |
| `DELIVERY_ZONE_INVALID_NAME`     | Name empty or longer than 100 characters |
| `DELIVERY_ZONE_INVALID_BOUNDARY` | Not a `Polygon`/`MultiPolygon`, a ring with fewer than 4 positions, an open ring, or a coordinate out of range |

**403 Forbidden** — the caller lacks `delivery-zones:manage`

---

## Update Zone

`PUT /api/delivery-zones/:id`

Every field is optional; only sent fields change. Redrawing or deactivating a zone affects **later** delivery orders only: existing orders keep the zone they were accepted in. Deactivating every zone stops new deliveries without a deploy.

### Example Request

```json
{
  "active": false
}
```

### Example Response (200 OK)

The updated zone.

### Error Responses

**400 Bad Request** — invalid name or boundary (same codes as Create)

**404 Not Found** — the zone does not exist or was deleted

```json
{
  "error": "Delivery zone not found"
}
```

---

## Delete Zone

`DELETE /api/delivery-zones/:id`

Soft-deletes the zone. Orders already recorded in it are unchanged.

### Example Response (200 OK)

```json
{
  "message": "Delivery zone deleted successfully"
}
```

**404 Not Found** — the zone does not exist or was already deleted

---

## Check a Point

`GET /api/delivery-zones/check?lat=<latitude>&lng=<longitude>`

Answers whether a point is inside an active zone, without creating anything. When several active zones contain it, the oldest one is reported — the same one an order would record.

### Query Parameters

| Param | Type   | Required | Description |
| ----- | ------ | -------- | ----------- |
| lat   | number | Yes      | Latitude, decimal degrees in [-90, 90] |
| lng   | number | Yes      | Longitude, decimal degrees in [-180, 180] |

### Example Response (200 OK, deliverable)

```json
{
  "deliverable": true,
  "zone_id": "019a0f00-1111-7000-8000-00000000c3e7",
  "zone_name": "Centro"
}
```

### Example Response (200 OK, not deliverable)

```json
{
  "deliverable": false
}
```

### Error Responses

**400 Bad Request** — `lat`/`lng` missing, not numbers, or out of range

```json
{
  "error": "lat and lng query parameters must be decimal degrees"
}
```
