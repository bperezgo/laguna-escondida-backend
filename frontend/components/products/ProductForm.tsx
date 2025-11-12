'use client';

import { useState, useEffect } from 'react';
import type { Product, CreateProductRequest, UpdateProductRequest } from '@/types/product';

interface ProductFormProps {
  product?: Product | null;
  onSubmit: (data: CreateProductRequest | UpdateProductRequest) => Promise<void>;
  onCancel: () => void;
  isLoading?: boolean;
}

export default function ProductForm({ product, onSubmit, onCancel, isLoading = false }: ProductFormProps) {
  const [formData, setFormData] = useState({
    name: product?.name || '',
    category: product?.category || '',
    price: product?.unit_price?.toString() || '',
    vat: product?.vat?.toString() || '',
    ico: product?.ico?.toString() || '',
    description: product?.description || '',
    brand: product?.brand || '',
    model: product?.model || '',
    sku: product?.sku || '',
    total_price_with_taxes: product?.total_price_with_taxes?.toString() || '',
  });

  const [errors, setErrors] = useState<Record<string, string>>({});

  useEffect(() => {
    if (product) {
      setFormData({
        name: product.name || '',
        category: product.category || '',
        price: product.unit_price?.toString() || '',
        vat: product.vat?.toString() || '',
        ico: product.ico?.toString() || '',
        description: product.description || '',
        brand: product.brand || '',
        model: product.model || '',
        sku: product.sku || '',
        total_price_with_taxes: product.total_price_with_taxes?.toString() || '',
      });
    }
  }, [product]);

  const validate = (): boolean => {
    const newErrors: Record<string, string> = {};

    // Name: required, min=1, max=255
    if (!formData.name.trim()) {
      newErrors.name = 'Product name is required';
    } else if (formData.name.length > 255) {
      newErrors.name = 'Product name must be 255 characters or less';
    }

    // Category: required, min=1, max=100
    if (!formData.category.trim()) {
      newErrors.category = 'Category is required';
    } else if (formData.category.length > 100) {
      newErrors.category = 'Category must be 100 characters or less';
    }

    // VAT: required, gte=0 (as string)
    if (!formData.vat.trim()) {
      newErrors.vat = 'VAT is required';
    } else {
      const vat = parseFloat(formData.vat);
      if (isNaN(vat) || vat < 0) {
        newErrors.vat = 'VAT must be a valid number greater than or equal to 0';
      }
    }

    // ICO: required, gte=0 (as string)
    if (!formData.ico.trim()) {
      newErrors.ico = 'ICO is required';
    } else {
      const ico = parseFloat(formData.ico);
      if (isNaN(ico) || ico < 0) {
        newErrors.ico = 'ICO must be a valid number greater than or equal to 0';
      }
    }

    // SKU: required, min=1, max=255
    if (!formData.sku.trim()) {
      newErrors.sku = 'SKU is required';
    } else if (formData.sku.length > 255) {
      newErrors.sku = 'SKU must be 255 characters or less';
    }

    // TotalPriceWithTaxes: required, gt=0 (as string)
    if (!formData.total_price_with_taxes.trim()) {
      newErrors.total_price_with_taxes = 'Total price with taxes is required';
    } else {
      const totalPriceWithTaxes = parseFloat(formData.total_price_with_taxes);
      if (isNaN(totalPriceWithTaxes) || totalPriceWithTaxes <= 0) {
        newErrors.total_price_with_taxes = 'Total price with taxes must be a valid number greater than 0';
      }
    }

    // Price: required for updates, gt=0
    if (product) {
      if (!formData.price.trim()) {
        newErrors.price = 'Price is required';
      } else {
        const price = parseFloat(formData.price);
        if (isNaN(price) || price <= 0) {
          newErrors.price = 'Price must be a valid number greater than 0';
        }
      }
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!validate()) {
      return;
    }

    // Build submit data
    if (product) {
      // Update request - all fields are required
      const submitData: UpdateProductRequest = {
        name: formData.name.trim(),
        category: formData.category.trim(),
        price: parseFloat(formData.price),
        vat: formData.vat.trim(),
        ico: formData.ico.trim(),
        taxes_format: 'percentage',
        sku: formData.sku.trim(),
        total_price_with_taxes: formData.total_price_with_taxes.trim(),
      };

      // Add optional fields only if they have values
      if (formData.description.trim()) {
        submitData.description = formData.description.trim();
      }
      if (formData.brand.trim()) {
        submitData.brand = formData.brand.trim();
      }
      if (formData.model.trim()) {
        submitData.model = formData.model.trim();
      }

      await onSubmit(submitData);
    } else {
      // Create request - all fields are required
      const submitData: CreateProductRequest = {
        name: formData.name.trim(),
        category: formData.category.trim(),
        vat: formData.vat.trim(),
        ico: formData.ico.trim(),
        taxes_format: 'percentage',
        sku: formData.sku.trim(),
        total_price_with_taxes: formData.total_price_with_taxes.trim(),
      };

      // Add optional fields only if they have values
      if (formData.description.trim()) {
        submitData.description = formData.description.trim();
      }
      if (formData.brand.trim()) {
        submitData.brand = formData.brand.trim();
      }
      if (formData.model.trim()) {
        submitData.model = formData.model.trim();
      }

      await onSubmit(submitData);
    }
  };

  const handleChange = (field: string, value: string) => {
    setFormData(prev => ({ ...prev, [field]: value }));
    // Clear error when user starts typing
    if (errors[field]) {
      setErrors(prev => {
        const newErrors = { ...prev };
        delete newErrors[field];
        return newErrors;
      });
    }
  };

  return (
    <div style={{
      position: 'fixed',
      top: 0,
      left: 0,
      right: 0,
      bottom: 0,
      backgroundColor: 'rgba(0, 0, 0, 0.5)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      zIndex: 1000,
      padding: '1rem',
    }}>
      <div style={{
        backgroundColor: 'white',
        borderRadius: '8px',
        padding: '2rem',
        maxWidth: '500px',
        width: '100%',
        maxHeight: '90vh',
        overflowY: 'auto',
        boxShadow: '0 4px 6px rgba(0, 0, 0, 0.1)',
      }}>
        <h2 style={{ marginTop: 0, marginBottom: '1.5rem', fontSize: '1.5rem', fontWeight: 'bold' }}>
          {product ? 'Edit Product' : 'Create New Product'}
        </h2>

        <form onSubmit={handleSubmit}>
          <div style={{ marginBottom: '1.5rem' }}>
            <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333' }}>
              Product Name *
            </label>
            <input
              type="text"
              value={formData.name}
              onChange={(e) => handleChange('name', e.target.value)}
              style={{
                width: '100%',
                padding: '0.75rem',
                border: `1px solid ${errors.name ? '#dc3545' : '#ced4da'}`,
                borderRadius: '4px',
                fontSize: '1rem',
                boxSizing: 'border-box',
              }}
              placeholder="Enter product name"
              maxLength={255}
            />
            {errors.name && (
              <p style={{ margin: '0.25rem 0 0 0', color: '#dc3545', fontSize: '0.875rem' }}>
                {errors.name}
              </p>
            )}
          </div>

          <div style={{ marginBottom: '1.5rem' }}>
            <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333' }}>
              Category *
            </label>
            <input
              type="text"
              value={formData.category}
              onChange={(e) => handleChange('category', e.target.value)}
              style={{
                width: '100%',
                padding: '0.75rem',
                border: `1px solid ${errors.category ? '#dc3545' : '#ced4da'}`,
                borderRadius: '4px',
                fontSize: '1rem',
                boxSizing: 'border-box',
              }}
              placeholder="Enter category"
              maxLength={100}
            />
            {errors.category && (
              <p style={{ margin: '0.25rem 0 0 0', color: '#dc3545', fontSize: '0.875rem' }}>
                {errors.category}
              </p>
            )}
          </div>

          <div style={{ marginBottom: '1.5rem' }}>
            <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333' }}>
              SKU *
            </label>
            <input
              type="text"
              value={formData.sku}
              onChange={(e) => handleChange('sku', e.target.value)}
              style={{
                width: '100%',
                padding: '0.75rem',
                border: `1px solid ${errors.sku ? '#dc3545' : '#ced4da'}`,
                borderRadius: '4px',
                fontSize: '1rem',
                boxSizing: 'border-box',
              }}
              placeholder="Enter SKU"
              maxLength={255}
            />
            {errors.sku && (
              <p style={{ margin: '0.25rem 0 0 0', color: '#dc3545', fontSize: '0.875rem' }}>
                {errors.sku}
              </p>
            )}
          </div>

          {product && (
            <div style={{ marginBottom: '1.5rem' }}>
              <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333' }}>
                Price *
              </label>
              <input
                type="number"
                step="0.01"
                min="0.01"
                value={formData.price}
                onChange={(e) => handleChange('price', e.target.value)}
                style={{
                  width: '100%',
                  padding: '0.75rem',
                  border: `1px solid ${errors.price ? '#dc3545' : '#ced4da'}`,
                  borderRadius: '4px',
                  fontSize: '1rem',
                  boxSizing: 'border-box',
                }}
                placeholder="0.00"
              />
              {errors.price && (
                <p style={{ margin: '0.25rem 0 0 0', color: '#dc3545', fontSize: '0.875rem' }}>
                  {errors.price}
                </p>
              )}
            </div>
          )}

          <div style={{ marginBottom: '1.5rem' }}>
            <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333' }}>
              Total Price with Taxes *
            </label>
            <input
              type="text"
              value={formData.total_price_with_taxes}
              onChange={(e) => handleChange('total_price_with_taxes', e.target.value)}
              style={{
                width: '100%',
                padding: '0.75rem',
                border: `1px solid ${errors.total_price_with_taxes ? '#dc3545' : '#ced4da'}`,
                borderRadius: '4px',
                fontSize: '1rem',
                boxSizing: 'border-box',
              }}
              placeholder="0.00"
            />
            {errors.total_price_with_taxes && (
              <p style={{ margin: '0.25rem 0 0 0', color: '#dc3545', fontSize: '0.875rem' }}>
                {errors.total_price_with_taxes}
              </p>
            )}
          </div>

          <div style={{ marginBottom: '1.5rem' }}>
            <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333' }}>
              VAT *
            </label>
            <input
              type="text"
              value={formData.vat}
              onChange={(e) => handleChange('vat', e.target.value)}
              style={{
                width: '100%',
                padding: '0.75rem',
                border: `1px solid ${errors.vat ? '#dc3545' : '#ced4da'}`,
                borderRadius: '4px',
                fontSize: '1rem',
                boxSizing: 'border-box',
              }}
              placeholder="0.00"
            />
            {errors.vat && (
              <p style={{ margin: '0.25rem 0 0 0', color: '#dc3545', fontSize: '0.875rem' }}>
                {errors.vat}
              </p>
            )}
          </div>

          <div style={{ marginBottom: '1.5rem' }}>
            <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333' }}>
              ICO *
            </label>
            <input
              type="text"
              value={formData.ico}
              onChange={(e) => handleChange('ico', e.target.value)}
              style={{
                width: '100%',
                padding: '0.75rem',
                border: `1px solid ${errors.ico ? '#dc3545' : '#ced4da'}`,
                borderRadius: '4px',
                fontSize: '1rem',
                boxSizing: 'border-box',
              }}
              placeholder="0.00"
            />
            {errors.ico && (
              <p style={{ margin: '0.25rem 0 0 0', color: '#dc3545', fontSize: '0.875rem' }}>
                {errors.ico}
              </p>
            )}
          </div>

          <div style={{ marginBottom: '1.5rem' }}>
            <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333' }}>
              Description
            </label>
            <textarea
              value={formData.description}
              onChange={(e) => handleChange('description', e.target.value)}
              style={{
                width: '100%',
                padding: '0.75rem',
                border: `1px solid ${errors.description ? '#dc3545' : '#ced4da'}`,
                borderRadius: '4px',
                fontSize: '1rem',
                boxSizing: 'border-box',
                minHeight: '80px',
                resize: 'vertical',
              }}
              placeholder="Enter product description (optional)"
            />
            {errors.description && (
              <p style={{ margin: '0.25rem 0 0 0', color: '#dc3545', fontSize: '0.875rem' }}>
                {errors.description}
              </p>
            )}
          </div>

          <div style={{ marginBottom: '1.5rem' }}>
            <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333' }}>
              Brand
            </label>
            <input
              type="text"
              value={formData.brand}
              onChange={(e) => handleChange('brand', e.target.value)}
              style={{
                width: '100%',
                padding: '0.75rem',
                border: `1px solid ${errors.brand ? '#dc3545' : '#ced4da'}`,
                borderRadius: '4px',
                fontSize: '1rem',
                boxSizing: 'border-box',
              }}
              placeholder="Enter brand (optional)"
            />
            {errors.brand && (
              <p style={{ margin: '0.25rem 0 0 0', color: '#dc3545', fontSize: '0.875rem' }}>
                {errors.brand}
              </p>
            )}
          </div>

          <div style={{ marginBottom: '1.5rem' }}>
            <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333' }}>
              Model
            </label>
            <input
              type="text"
              value={formData.model}
              onChange={(e) => handleChange('model', e.target.value)}
              style={{
                width: '100%',
                padding: '0.75rem',
                border: `1px solid ${errors.model ? '#dc3545' : '#ced4da'}`,
                borderRadius: '4px',
                fontSize: '1rem',
                boxSizing: 'border-box',
              }}
              placeholder="Enter model (optional)"
            />
            {errors.model && (
              <p style={{ margin: '0.25rem 0 0 0', color: '#dc3545', fontSize: '0.875rem' }}>
                {errors.model}
              </p>
            )}
          </div>

          <div style={{ display: 'flex', gap: '1rem', justifyContent: 'flex-end' }}>
            <button
              type="button"
              onClick={onCancel}
              disabled={isLoading}
              style={{
                padding: '0.75rem 1.5rem',
                backgroundColor: '#6c757d',
                color: 'white',
                border: 'none',
                borderRadius: '4px',
                cursor: isLoading ? 'not-allowed' : 'pointer',
                fontSize: '1rem',
                fontWeight: '500',
                opacity: isLoading ? 0.6 : 1,
              }}
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isLoading}
              style={{
                padding: '0.75rem 1.5rem',
                backgroundColor: '#28a745',
                color: 'white',
                border: 'none',
                borderRadius: '4px',
                cursor: isLoading ? 'not-allowed' : 'pointer',
                fontSize: '1rem',
                fontWeight: '500',
                opacity: isLoading ? 0.6 : 1,
              }}
            >
              {isLoading ? 'Saving...' : (product ? 'Update' : 'Create')}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

