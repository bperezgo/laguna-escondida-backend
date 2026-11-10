import { apiRequest } from "./config";
import type {
  ProductIngredient,
  ProductIngredientWithProduct,
  ProductIngredientListResponse,
  AddIngredientRequest,
  UpdateIngredientRequest,
  PlateOptionSlot,
  PlateOptionSlotListResponse,
  ReplacePlateOptionSlotsRequest,
} from "@/types/product";

/**
 * Recipe (ingredient) management for COMPOSITE products.
 * Wraps the backend endpoints under /products/:id/ingredients.
 */
export const recipeApi = {
  /**
   * List a composite product's recipe (its ingredients, with the joined product).
   */
  async list(productId: string): Promise<ProductIngredientWithProduct[]> {
    const response = await apiRequest<ProductIngredientListResponse>(
      `/products/${productId}/ingredients`,
    );
    return response.ingredients || [];
  },

  /**
   * Add an ingredient to a composite product's recipe.
   */
  async add(
    productId: string,
    req: AddIngredientRequest,
  ): Promise<ProductIngredient> {
    return apiRequest<ProductIngredient>(
      `/products/${productId}/ingredients`,
      {
        method: "POST",
        body: JSON.stringify(req),
      },
    );
  },

  /**
   * Update an ingredient's quantity.
   */
  async update(
    productId: string,
    ingredientId: string,
    req: UpdateIngredientRequest,
  ): Promise<ProductIngredient> {
    return apiRequest<ProductIngredient>(
      `/products/${productId}/ingredients/${ingredientId}`,
      {
        method: "PUT",
        body: JSON.stringify(req),
      },
    );
  },

  /**
   * Remove an ingredient from a composite product's recipe.
   */
  async remove(productId: string, ingredientId: string): Promise<void> {
    return apiRequest<void>(
      `/products/${productId}/ingredients/${ingredientId}`,
      {
        method: "DELETE",
      },
    );
  },

  /**
   * List a composite plate's option slots (empty when it has none).
   */
  async listOptionSlots(productId: string): Promise<PlateOptionSlot[]> {
    const response = await apiRequest<PlateOptionSlotListResponse>(
      `/products/${productId}/option-slots`,
    );
    return response.slots || [];
  },

  /**
   * Replace the plate's whole slot list; returns the saved list. Cloud only.
   */
  async replaceOptionSlots(
    productId: string,
    req: ReplacePlateOptionSlotsRequest,
  ): Promise<PlateOptionSlot[]> {
    const response = await apiRequest<PlateOptionSlotListResponse>(
      `/products/${productId}/option-slots`,
      { method: "PUT", body: JSON.stringify(req) },
    );
    return response.slots || [];
  },
};
