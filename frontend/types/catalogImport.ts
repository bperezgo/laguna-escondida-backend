// Mirrors ../backend/docs/api/catalog-import.md. Taxes are percentages (8 means 8 %).
import type { UnitOfMeasure } from "./product.ts";

export interface CatalogImportDefaults {
  vat?: number;
  ico?: number;
  taxes_format?: "percentage";
  unit_of_measure?: UnitOfMeasure;
  prep_area?: string;
  prep_priority?: number;
}

export interface CatalogImportExisting {
  ref: string;
  sku?: string;
  name?: string;
}

export interface CatalogImportPrep {
  area?: string;
  priority?: number;
}

export interface CatalogImportProductFields {
  ref: string;
  name: string;
  category: string;
  sku?: string;
  description?: string;
  unit_of_measure?: UnitOfMeasure;
  total_price_with_taxes?: number;
  vat?: number;
  ico?: number;
  taxes_format?: "percentage";
  prep?: CatalogImportPrep;
}

export interface CatalogImportIngredient extends CatalogImportProductFields {
  product_type?: "INGREDIENT" | "BOTH";
}

export type CatalogImportSellable = CatalogImportProductFields;

export interface CatalogImportRecipeLine {
  ingredient: string;
  quantity: number;
}

export interface CatalogImportChoiceItem {
  ingredient: string;
  quantity: number;
}

export interface CatalogImportChoice {
  name: string;
  default?: boolean;
  items?: CatalogImportChoiceItem[];
}

export interface CatalogImportSlot {
  name: string;
  choices: CatalogImportChoice[];
}

export interface CatalogImportPlate extends CatalogImportProductFields {
  recipe?: CatalogImportRecipeLine[];
  slots?: CatalogImportSlot[];
}

export interface CatalogImportStockCount {
  product: string;
  counted: number;
}

export interface CatalogImportDocument {
  defaults?: CatalogImportDefaults;
  existing?: CatalogImportExisting[];
  ingredients?: CatalogImportIngredient[];
  sellables?: CatalogImportSellable[];
  plates?: CatalogImportPlate[];
  opening_stock?: CatalogImportStockCount[];
}

export interface ApplyCatalogImportRequest extends CatalogImportDocument {
  expected_digest: string;
}

export type CatalogImportProblemCode =
  | "duplicate_ref"
  | "unknown_ref"
  | "existing_not_found"
  | "existing_ambiguous"
  | "name_conflict"
  | "sku_conflict"
  | "missing_field"
  | "invalid_field"
  | "invalid_unit"
  | "invalid_link_type"
  | "invalid_quantity"
  | "duplicate_item"
  | "invalid_slot"
  | "slot_default_count"
  | "stock_on_existing"
  | "limit_exceeded"
  | "digest_mismatch";

export interface CatalogImportProblem {
  path: string;
  code: CatalogImportProblemCode | string;
  message: string;
}

export interface CatalogImportGeneratedSku {
  ref: string;
  sku: string;
}

export interface CatalogImportSummary {
  products: number;
  ingredients: number;
  sellables: number;
  plates: number;
  existing: number;
  recipe_lines: number;
  option_slots: number;
  option_choices: number;
  preparation_areas: number;
  opening_stock: number;
  generated_skus: CatalogImportGeneratedSku[];
}

export interface CatalogImportCreatedProduct {
  id: string;
  sku: string;
}

export interface CatalogImportResult {
  valid: boolean;
  applied: boolean;
  problems: CatalogImportProblem[];
  summary: CatalogImportSummary;
  digest: string;
  products?: Record<string, CatalogImportCreatedProduct>;
}
