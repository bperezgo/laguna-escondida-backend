import { apiRequest } from "./config";
import type {
  ProductIngredient,
  ProductIngredientWithProduct,
  ProductIngredientListResponse,
  AddIngredientRequest,
  UpdateIngredientRequest,
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
};
