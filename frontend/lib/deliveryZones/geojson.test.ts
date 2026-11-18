import { test } from "node:test";
import assert from "node:assert/strict";
import {
  parseBoundaryText,
  parseBoundary,
  describeBoundary,
  geojsonIoUrl,
} from "./geojson.ts";
import type { Position, ZoneBoundary } from "@/types/deliveryZone";

const square: Position[] = [
  [-74.1, 4.58],
  [-74.06, 4.58],
  [-74.06, 4.62],
  [-74.1, 4.62],
  [-74.1, 4.58],
];
const hole: Position[] = [
  [-74.085, 4.595],
  [-74.075, 4.595],
  [-74.075, 4.605],
  [-74.085, 4.605],
  [-74.085, 4.595],
];

test("accepts a bare Polygon geometry", () => {
  const result = parseBoundaryText(
    JSON.stringify({ type: "Polygon", coordinates: [square, hole] }),
  );
  assert.deepEqual(result, {
    ok: true,
    boundary: { type: "Polygon", coordinates: [square, hole] },
  });
});

test("unwraps a geojson.io FeatureCollection with one polygon", () => {
  const result = parseBoundary({
    type: "FeatureCollection",
    features: [
      {
        type: "Feature",
        properties: {},
        geometry: { type: "Polygon", coordinates: [square] },
      },
    ],
  });
  assert.equal(result.ok, true);
  assert.equal(result.ok && result.boundary.type, "Polygon");
});

test("merges several features into a MultiPolygon", () => {
  const result = parseBoundary({
    type: "FeatureCollection",
    features: [
      { type: "Feature", geometry: { type: "Polygon", coordinates: [square] } },
      {
        type: "Feature",
        geometry: { type: "MultiPolygon", coordinates: [[hole]] },
      },
    ],
  });
  assert.deepEqual(result, {
    ok: true,
    boundary: { type: "MultiPolygon", coordinates: [[square], [hole]] },
  });
});

test("drops altitude from positions", () => {
  const withAltitude = square.map(([lng, lat]) => [lng, lat, 2600]);
  const result = parseBoundary({ type: "Polygon", coordinates: [withAltitude] });
  assert.deepEqual(result, {
    ok: true,
    boundary: { type: "Polygon", coordinates: [square] },
  });
});

test("rejects what the backend would reject", () => {
  const cases: [string, unknown][] = [
    ["invalid JSON", "{nope"],
    ["a line", { type: "LineString", coordinates: square }],
    ["a point feature", { type: "Feature", geometry: { type: "Point", coordinates: [-74, 4] } }],
    ["an open ring", { type: "Polygon", coordinates: [square.slice(0, 4)] }],
    ["a short ring", { type: "Polygon", coordinates: [[square[0], square[1], square[0]]] }],
    ["out of range", { type: "Polygon", coordinates: [[[4, -200], [5, -200], [5, -199], [4, -200]]] }],
    ["empty collection", { type: "FeatureCollection", features: [] }],
  ];
  for (const [name, input] of cases) {
    const result =
      typeof input === "string" ? parseBoundaryText(input) : parseBoundary(input);
    assert.equal(result.ok, false, name);
  }
});

test("describes areas and holes", () => {
  const multi: ZoneBoundary = {
    type: "MultiPolygon",
    coordinates: [[square, hole], [square]],
  };
  assert.equal(describeBoundary({ type: "Polygon", coordinates: [square] }), "1 área");
  assert.equal(describeBoundary(multi), "2 áreas · 1 exclusión");
});

test("geojson.io URL carries the zones in the fragment", () => {
  const url = geojsonIoUrl([
    { name: "Centro", boundary: { type: "Polygon", coordinates: [square] }, active: false },
  ]);
  const prefix = "https://geojson.io/#data=data:application/json,";
  assert.ok(url.startsWith(prefix));
  const collection = JSON.parse(decodeURIComponent(url.slice(prefix.length)));
  assert.equal(collection.features[0].properties.name, "Centro");
  assert.equal(collection.features[0].properties.active, false);
  assert.deepEqual(collection.features[0].geometry.coordinates, [square]);
});
