'use client';

import { useState, useEffect } from 'react';
import type { CreateElectronicInvoiceRequest, ElectronicInvoicePaymentCode, DocumentType, InvoiceItem, InvoiceAllowance, InvoiceTax } from '@/types/invoice';
import type { Product } from '@/types/product';
import { productsApi } from '@/lib/api/products';

interface InvoiceFormProps {
  onSubmit: (data: CreateElectronicInvoiceRequest) => Promise<void>;
  onCancel: () => void;
  isLoading?: boolean;
}

const PAYMENT_CODES: { value: ElectronicInvoicePaymentCode; label: string }[] = [
  { value: 'credit_card', label: 'Credit Card' },
  { value: 'debit_card', label: 'Debit Card' },
  { value: 'cash', label: 'Cash' },
  { value: 'transfer_debit_bank', label: 'Transfer Debit Bank' },
  { value: 'transfer_credit_bank', label: 'Transfer Credit Bank' },
  { value: 'transfer_debit_interbank', label: 'Transfer Debit Interbank' },
];

const DOCUMENT_TYPES: { value: DocumentType; label: string }[] = [
  { value: 'CC', label: 'CC - National Identification Number' },
  { value: 'NIT', label: 'NIT' },
];

export default function InvoiceForm({ onSubmit, onCancel, isLoading = false }: InvoiceFormProps) {
  const [formData, setFormData] = useState({
    consecutive: '',
    issue_date: '',
    issue_time: '',
    payment_code: 'cash' as ElectronicInvoicePaymentCode,
    customer: {
      id: '',
      document_type: 'CC' as DocumentType,
      name: '',
      email: '',
    },
    amounts: {
      totalAmount: '',
      discountAmount: '',
      taxAmount: '',
      payAmount: '',
    },
    items: [] as InvoiceItem[],
  });

  const [errors, setErrors] = useState<Record<string, string>>({});
  const [products, setProducts] = useState<Product[]>([]);
  const [productsLoading, setProductsLoading] = useState(false);
  const [itemProductSearch, setItemProductSearch] = useState<Record<number, string>>({});
  const [itemSelectedProduct, setItemSelectedProduct] = useState<Record<number, Product | null>>({});
  const [itemDropdownOpen, setItemDropdownOpen] = useState<Record<number, boolean>>({});

  // Fetch products on mount
  useEffect(() => {
    const fetchProducts = async () => {
      setProductsLoading(true);
      try {
        const fetchedProducts = await productsApi.getAll();
        setProducts(fetchedProducts);
      } catch (error) {
        console.error('Error fetching products:', error);
      } finally {
        setProductsLoading(false);
      }
    };

    fetchProducts();
  }, []);

  // Filter products based on search query for a specific item
  const getFilteredProducts = (itemIndex: number): Product[] => {
    const searchQuery = itemProductSearch[itemIndex] || '';
    if (!searchQuery.trim()) {
      return products;
    }
    const query = searchQuery.toLowerCase();
    return products.filter(product =>
      product.name.toLowerCase().includes(query) ||
      product.sku.toLowerCase().includes(query) ||
      (product.description && product.description.toLowerCase().includes(query)) ||
      (product.brand && product.brand.toLowerCase().includes(query)) ||
      (product.model && product.model.toLowerCase().includes(query))
    );
  };

  // Handle product selection for an item
  const handleProductSelect = (itemIndex: number, product: Product) => {
    setItemSelectedProduct(prev => ({
      ...prev,
      [itemIndex]: product,
    }));
    setItemProductSearch(prev => ({
      ...prev,
      [itemIndex]: product.name,
    }));
    setItemDropdownOpen(prev => ({
      ...prev,
      [itemIndex]: false,
    }));

    // Auto-populate item fields from product
    updateItem(itemIndex, 'description', product.description || product.name);
    updateItem(itemIndex, 'unitPrice', product.unit_price.toString());
    updateItem(itemIndex, 'brand', product.brand || '');
    updateItem(itemIndex, 'model', product.model || '');
    updateItem(itemIndex, 'code', product.sku);
    
    // Recalculate total if quantity is already set
    const currentItem = formData.items[itemIndex];
    if (currentItem.quantity) {
      calculateItemTotal(itemIndex);
    }
  };

  // Handle clearing product selection for an item
  const handleClearProduct = (itemIndex: number) => {
    setItemSelectedProduct(prev => {
      const newState = { ...prev };
      delete newState[itemIndex];
      return newState;
    });
    setItemProductSearch(prev => ({
      ...prev,
      [itemIndex]: '',
    }));
    // Clear auto-populated fields
    updateItem(itemIndex, 'description', '');
    updateItem(itemIndex, 'unitPrice', '');
    updateItem(itemIndex, 'brand', '');
    updateItem(itemIndex, 'model', '');
    updateItem(itemIndex, 'code', '');
    updateItem(itemIndex, 'total', '');
  };

  const validate = (): boolean => {
    const newErrors: Record<string, string> = {};

    // Validate consecutive
    const consecutive = parseInt(formData.consecutive);
    if (isNaN(consecutive) || consecutive < 0) {
      newErrors.consecutive = 'Consecutive must be a valid positive number';
    }

    // Validate dates
    if (!formData.issue_date.trim()) {
      newErrors.issue_date = 'Issue date is required';
    }
    if (!formData.issue_time.trim()) {
      newErrors.issue_time = 'Issue time is required';
    }

    // Validate customer (optional - if any field is filled, all are required)
    const hasCustomerData = 
      formData.customer.id.trim() || 
      formData.customer.name.trim() || 
      formData.customer.email.trim();
    
    if (hasCustomerData) {
      if (!formData.customer.id.trim()) {
        newErrors['customer.id'] = 'Document number is required when customer information is provided';
      }
      if (!formData.customer.name.trim()) {
        newErrors['customer.name'] = 'Customer name is required when customer information is provided';
      }
      if (!formData.customer.email.trim()) {
        newErrors['customer.email'] = 'Customer email is required when customer information is provided';
      } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(formData.customer.email)) {
        newErrors['customer.email'] = 'Invalid email format';
      }
    }

    // Validate amounts
    if (!formData.amounts.totalAmount.trim()) {
      newErrors['amounts.totalAmount'] = 'Total amount is required';
    }
    if (!formData.amounts.payAmount.trim()) {
      newErrors['amounts.payAmount'] = 'Pay amount is required';
    }

    // Validate items
    if (formData.items.length === 0) {
      newErrors.items = 'At least one item is required';
    }

    formData.items.forEach((item, index) => {
      if (!item.quantity.trim()) {
        newErrors[`items.${index}.quantity`] = 'Quantity is required';
      }
      if (!item.unitPrice.trim()) {
        newErrors[`items.${index}.unitPrice`] = 'Unit price is required';
      }
      if (!item.description.trim()) {
        newErrors[`items.${index}.description`] = 'Description is required';
      }
    });

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!validate()) {
      return;
    }

    // Only include customer if at least one field is filled
    const hasCustomerData = 
      formData.customer.id.trim() || 
      formData.customer.name.trim() || 
      formData.customer.email.trim();

    const submitData: CreateElectronicInvoiceRequest = {
      consecutive: parseInt(formData.consecutive),
      issue_date: formData.issue_date.trim(),
      issue_time: formData.issue_time.trim(),
      payment_code: formData.payment_code,
      ...(hasCustomerData && {
        customer: {
          id: formData.customer.id.trim(),
          document_type: formData.customer.document_type,
          name: formData.customer.name.trim(),
          email: formData.customer.email.trim(),
        },
      }),
      amounts: {
        totalAmount: formData.amounts.totalAmount.trim(),
        discountAmount: formData.amounts.discountAmount.trim() || '0',
        taxAmount: formData.amounts.taxAmount.trim() || '0',
        payAmount: formData.amounts.payAmount.trim(),
      },
      items: formData.items.map(item => ({
        quantity: item.quantity.trim(),
        unitPrice: item.unitPrice.trim(),
        total: item.total.trim(),
        description: item.description.trim(),
        brand: item.brand.trim(),
        model: item.model.trim(),
        code: item.code.trim(),
        allowance: item.allowance || [],
        taxes: item.taxes || [],
      })),
    };

    await onSubmit(submitData);
  };

  const handleChange = (field: string, value: string) => {
    const keys = field.split('.');
    if (keys.length === 1) {
      setFormData(prev => ({ ...prev, [field]: value }));
    } else if (keys.length === 2) {
      setFormData(prev => ({
        ...prev,
        [keys[0]]: {
          ...prev[keys[0] as keyof typeof prev] as any,
          [keys[1]]: value,
        },
      }));
    } else if (keys.length === 3 && keys[0] === 'amounts') {
      setFormData(prev => ({
        ...prev,
        amounts: {
          ...prev.amounts,
          [keys[2]]: value,
        },
      }));
    }
    // Clear error when user starts typing
    if (errors[field]) {
      setErrors(prev => {
        const newErrors = { ...prev };
        delete newErrors[field];
        return newErrors;
      });
    }
  };

  const addItem = () => {
    const newIndex = formData.items.length;
    setFormData(prev => ({
      ...prev,
      items: [
        ...prev.items,
        {
          quantity: '',
          unitPrice: '',
          total: '',
          description: '',
          brand: '',
          model: '',
          code: '',
        },
      ],
    }));
    // Initialize product search state for new item
    setItemProductSearch(prev => ({
      ...prev,
      [newIndex]: '',
    }));
    setItemSelectedProduct(prev => ({
      ...prev,
      [newIndex]: null,
    }));
  };

  const removeItem = (index: number) => {
    setFormData(prev => ({
      ...prev,
      items: prev.items.filter((_, i) => i !== index),
    }));
    // Clean up product search state
    setItemProductSearch(prev => {
      const newState = { ...prev };
      delete newState[index];
      // Reindex remaining items
      const reindexed: Record<number, string> = {};
      Object.keys(newState).forEach(key => {
        const oldIndex = parseInt(key);
        if (oldIndex > index) {
          reindexed[oldIndex - 1] = newState[oldIndex];
        } else if (oldIndex < index) {
          reindexed[oldIndex] = newState[oldIndex];
        }
      });
      return reindexed;
    });
    setItemSelectedProduct(prev => {
      const newState = { ...prev };
      delete newState[index];
      // Reindex remaining items
      const reindexed: Record<number, Product | null> = {};
      Object.keys(newState).forEach(key => {
        const oldIndex = parseInt(key);
        if (oldIndex > index) {
          reindexed[oldIndex - 1] = newState[oldIndex];
        } else if (oldIndex < index) {
          reindexed[oldIndex] = newState[oldIndex];
        }
      });
      return reindexed;
    });
  };

  const updateItem = (index: number, field: keyof InvoiceItem, value: string) => {
    setFormData(prev => ({
      ...prev,
      items: prev.items.map((item, i) =>
        i === index ? { ...item, [field]: value } : item
      ),
    }));
    // Clear error
    const errorKey = `items.${index}.${field}`;
    if (errors[errorKey]) {
      setErrors(prev => {
        const newErrors = { ...prev };
        delete newErrors[errorKey];
        return newErrors;
      });
    }
  };

  const calculateItemTotal = (index: number) => {
    const item = formData.items[index];
    const quantity = parseFloat(item.quantity) || 0;
    const unitPrice = parseFloat(item.unitPrice) || 0;
    const total = (quantity * unitPrice).toFixed(2);
    updateItem(index, 'total', total);
  };

  const getCurrentDateTime = () => {
    const now = new Date();
    return {
      date: now.toISOString().split('T')[0],
      time: now.toTimeString().split(' ')[0].substring(0, 5),
    };
  };

  const setCurrentDateTime = () => {
    const { date, time } = getCurrentDateTime();
    setFormData(prev => ({
      ...prev,
      issue_date: date,
      issue_time: time,
    }));
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
      overflowY: 'auto',
    }}>
      <div style={{
        backgroundColor: 'white',
        borderRadius: '8px',
        padding: '2rem',
        maxWidth: '900px',
        width: '100%',
        maxHeight: '95vh',
        overflowY: 'auto',
        boxShadow: '0 4px 6px rgba(0, 0, 0, 0.1)',
        margin: 'auto',
      }}>
        <h2 style={{ marginTop: 0, marginBottom: '1.5rem', fontSize: '1.5rem', fontWeight: 'bold' }}>
          Create Electronic Invoice
        </h2>

        <form onSubmit={handleSubmit}>
          {/* Invoice Details Section */}
          <div style={{ marginBottom: '2rem', paddingBottom: '1.5rem', borderBottom: '1px solid #e0e0e0' }}>
            <h3 style={{ marginTop: 0, marginBottom: '1rem', fontSize: '1.2rem', fontWeight: '600' }}>
              Invoice Details
            </h3>
            
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '1rem', marginBottom: '1rem' }}>
              <div>
                <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333' }}>
                  Consecutive *
                </label>
                <input
                  type="number"
                  min="0"
                  value={formData.consecutive}
                  onChange={(e) => handleChange('consecutive', e.target.value)}
                  style={{
                    width: '100%',
                    padding: '0.75rem',
                    border: `1px solid ${errors.consecutive ? '#dc3545' : '#ced4da'}`,
                    borderRadius: '4px',
                    fontSize: '1rem',
                    boxSizing: 'border-box',
                  }}
                />
                {errors.consecutive && (
                  <p style={{ margin: '0.25rem 0 0 0', color: '#dc3545', fontSize: '0.875rem' }}>
                    {errors.consecutive}
                  </p>
                )}
              </div>

              <div>
                <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333' }}>
                  Issue Date *
                </label>
                <div style={{ display: 'flex', gap: '0.5rem' }}>
                  <input
                    type="date"
                    value={formData.issue_date}
                    onChange={(e) => handleChange('issue_date', e.target.value)}
                    style={{
                      flex: 1,
                      padding: '0.75rem',
                      border: `1px solid ${errors.issue_date ? '#dc3545' : '#ced4da'}`,
                      borderRadius: '4px',
                      fontSize: '1rem',
                      boxSizing: 'border-box',
                    }}
                  />
                  <button
                    type="button"
                    onClick={setCurrentDateTime}
                    style={{
                      padding: '0.75rem',
                      backgroundColor: '#6c757d',
                      color: 'white',
                      border: 'none',
                      borderRadius: '4px',
                      cursor: 'pointer',
                      fontSize: '0.875rem',
                    }}
                    title="Set to current date/time"
                  >
                    Now
                  </button>
                </div>
                {errors.issue_date && (
                  <p style={{ margin: '0.25rem 0 0 0', color: '#dc3545', fontSize: '0.875rem' }}>
                    {errors.issue_date}
                  </p>
                )}
              </div>

              <div>
                <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333' }}>
                  Issue Time *
                </label>
                <input
                  type="time"
                  value={formData.issue_time}
                  onChange={(e) => handleChange('issue_time', e.target.value)}
                  style={{
                    width: '100%',
                    padding: '0.75rem',
                    border: `1px solid ${errors.issue_time ? '#dc3545' : '#ced4da'}`,
                    borderRadius: '4px',
                    fontSize: '1rem',
                    boxSizing: 'border-box',
                  }}
                />
                {errors.issue_time && (
                  <p style={{ margin: '0.25rem 0 0 0', color: '#dc3545', fontSize: '0.875rem' }}>
                    {errors.issue_time}
                  </p>
                )}
              </div>
            </div>

            <div>
              <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333' }}>
                Payment Code *
              </label>
              <select
                value={formData.payment_code}
                onChange={(e) => handleChange('payment_code', e.target.value as ElectronicInvoicePaymentCode)}
                style={{
                  width: '100%',
                  padding: '0.75rem',
                  border: '1px solid #ced4da',
                  borderRadius: '4px',
                  fontSize: '1rem',
                  boxSizing: 'border-box',
                }}
              >
                {PAYMENT_CODES.map(code => (
                  <option key={code.value} value={code.value}>{code.label}</option>
                ))}
              </select>
            </div>
          </div>

          {/* Customer Section */}
          <div style={{ marginBottom: '2rem', paddingBottom: '1.5rem', borderBottom: '1px solid #e0e0e0' }}>
            <h3 style={{ marginTop: 0, marginBottom: '1rem', fontSize: '1.2rem', fontWeight: '600' }}>
              Customer Information <span style={{ fontSize: '0.875rem', fontWeight: 'normal', color: '#666' }}>(Optional)</span>
            </h3>
            
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', marginBottom: '1rem' }}>
              <div>
                <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333' }}>
                  Document Number
                </label>
                <input
                  type="text"
                  value={formData.customer.id}
                  onChange={(e) => handleChange('customer.id', e.target.value)}
                  style={{
                    width: '100%',
                    padding: '0.75rem',
                    border: `1px solid ${errors['customer.id'] ? '#dc3545' : '#ced4da'}`,
                    borderRadius: '4px',
                    fontSize: '1rem',
                    boxSizing: 'border-box',
                  }}
                  placeholder="Enter document number"
                />
                {errors['customer.id'] && (
                  <p style={{ margin: '0.25rem 0 0 0', color: '#dc3545', fontSize: '0.875rem' }}>
                    {errors['customer.id']}
                  </p>
                )}
              </div>

              <div>
                <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333' }}>
                  Document Type
                </label>
                <select
                  value={formData.customer.document_type}
                  onChange={(e) => handleChange('customer.document_type', e.target.value as DocumentType)}
                  style={{
                    width: '100%',
                    padding: '0.75rem',
                    border: '1px solid #ced4da',
                    borderRadius: '4px',
                    fontSize: '1rem',
                    boxSizing: 'border-box',
                  }}
                >
                  {DOCUMENT_TYPES.map(type => (
                  <option key={type.value} value={type.value}>{type.label}</option>
                ))}
                </select>
              </div>
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
              <div>
                <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333' }}>
                  Name
                </label>
                <input
                  type="text"
                  value={formData.customer.name}
                  onChange={(e) => handleChange('customer.name', e.target.value)}
                  style={{
                    width: '100%',
                    padding: '0.75rem',
                    border: `1px solid ${errors['customer.name'] ? '#dc3545' : '#ced4da'}`,
                    borderRadius: '4px',
                    fontSize: '1rem',
                    boxSizing: 'border-box',
                  }}
                  placeholder="Enter customer name"
                />
                {errors['customer.name'] && (
                  <p style={{ margin: '0.25rem 0 0 0', color: '#dc3545', fontSize: '0.875rem' }}>
                    {errors['customer.name']}
                  </p>
                )}
              </div>

              <div>
                <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333' }}>
                  Email
                </label>
                <input
                  type="email"
                  value={formData.customer.email}
                  onChange={(e) => handleChange('customer.email', e.target.value)}
                  style={{
                    width: '100%',
                    padding: '0.75rem',
                    border: `1px solid ${errors['customer.email'] ? '#dc3545' : '#ced4da'}`,
                    borderRadius: '4px',
                    fontSize: '1rem',
                    boxSizing: 'border-box',
                  }}
                  placeholder="Enter customer email"
                />
                {errors['customer.email'] && (
                  <p style={{ margin: '0.25rem 0 0 0', color: '#dc3545', fontSize: '0.875rem' }}>
                    {errors['customer.email']}
                  </p>
                )}
              </div>
            </div>
          </div>

          {/* Items Section */}
          <div style={{ marginBottom: '2rem', paddingBottom: '1.5rem', borderBottom: '1px solid #e0e0e0' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
              <h3 style={{ margin: 0, fontSize: '1.2rem', fontWeight: '600' }}>
                Invoice Items
              </h3>
              <button
                type="button"
                onClick={addItem}
                style={{
                  padding: '0.5rem 1rem',
                  backgroundColor: '#007bff',
                  color: 'white',
                  border: 'none',
                  borderRadius: '4px',
                  cursor: 'pointer',
                  fontSize: '0.875rem',
                  fontWeight: '500',
                }}
              >
                + Add Item
              </button>
            </div>
            {errors.items && (
              <p style={{ margin: '0 0 1rem 0', color: '#dc3545', fontSize: '0.875rem' }}>
                {errors.items}
              </p>
            )}
            {formData.items.map((item, index) => (
              <div key={index} style={{
                border: '1px solid #e0e0e0',
                borderRadius: '4px',
                padding: '1rem',
                marginBottom: '1rem',
                backgroundColor: '#f9f9f9',
              }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
                  <h4 style={{ margin: 0, fontSize: '1rem', fontWeight: '600' }}>Item {index + 1}</h4>
                  <button
                    type="button"
                    onClick={() => removeItem(index)}
                    style={{
                      padding: '0.25rem 0.75rem',
                      backgroundColor: '#dc3545',
                      color: 'white',
                      border: 'none',
                      borderRadius: '4px',
                      cursor: 'pointer',
                      fontSize: '0.875rem',
                    }}
                  >
                    Remove
                  </button>
                </div>
                
                {/* Product Selector */}
                <div style={{ marginBottom: '1rem', position: 'relative' }}>
                  <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333', fontSize: '0.875rem' }}>
                    Select Product
                  </label>
                  <div style={{ position: 'relative' }}>
                    <input
                      type="text"
                      value={itemProductSearch[index] || ''}
                      onChange={(e) => {
                        setItemProductSearch(prev => ({
                          ...prev,
                          [index]: e.target.value,
                        }));
                      }}
                      onFocus={() => {
                        // Ensure search is visible when focused
                        if (!itemProductSearch[index]) {
                          setItemProductSearch(prev => ({
                            ...prev,
                            [index]: '',
                          }));
                        }
                      }}
                      placeholder={productsLoading ? 'Loading products...' : 'Search and select a product'}
                      disabled={productsLoading}
                      style={{
                        width: '100%',
                        padding: '0.5rem',
                        border: '1px solid #ced4da',
                        borderRadius: '4px',
                        fontSize: '0.875rem',
                        boxSizing: 'border-box',
                      }}
                    />
                    {itemProductSearch[index] !== undefined && itemProductSearch[index] !== '' && getFilteredProducts(index).length > 0 && (
                      <div style={{
                        position: 'absolute',
                        top: '100%',
                        left: 0,
                        right: 0,
                        backgroundColor: 'white',
                        border: '1px solid #ced4da',
                        borderRadius: '4px',
                        maxHeight: '200px',
                        overflowY: 'auto',
                        zIndex: 1000,
                        marginTop: '0.25rem',
                        boxShadow: '0 2px 8px rgba(0,0,0,0.15)',
                      }}>
                        {getFilteredProducts(index).map((product) => (
                          <div
                            key={product.id}
                            onClick={() => handleProductSelect(index, product)}
                            style={{
                              padding: '0.75rem',
                              cursor: 'pointer',
                              borderBottom: '1px solid #f0f0f0',
                              transition: 'background-color 0.2s',
                            }}
                            onMouseEnter={(e) => {
                              e.currentTarget.style.backgroundColor = '#f8f9fa';
                            }}
                            onMouseLeave={(e) => {
                              e.currentTarget.style.backgroundColor = 'white';
                            }}
                          >
                            <div style={{ fontWeight: '500', fontSize: '0.875rem', marginBottom: '0.25rem' }}>
                              {product.name}
                            </div>
                            <div style={{ fontSize: '0.75rem', color: '#666' }}>
                              SKU: {product.sku} | Price: ${product.unit_price.toFixed(2)}
                              {product.description && ` | ${product.description.substring(0, 50)}${product.description.length > 50 ? '...' : ''}`}
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                  
                  {/* Display selected product information */}
                  {itemSelectedProduct[index] && (
                    <div style={{
                      marginTop: '0.75rem',
                      padding: '0.75rem',
                      backgroundColor: '#e7f3ff',
                      borderRadius: '4px',
                      border: '1px solid #b3d9ff',
                    }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.5rem' }}>
                        <div style={{ fontSize: '0.875rem', fontWeight: '500', color: '#0066cc' }}>
                          Selected Product:
                        </div>
                        <button
                          type="button"
                          onClick={() => handleClearProduct(index)}
                          style={{
                            padding: '0.25rem 0.5rem',
                            backgroundColor: '#dc3545',
                            color: 'white',
                            border: 'none',
                            borderRadius: '4px',
                            cursor: 'pointer',
                            fontSize: '0.75rem',
                          }}
                          title="Clear product selection"
                        >
                          Clear
                        </button>
                      </div>
                      <div style={{ fontSize: '0.875rem', lineHeight: '1.6' }}>
                        <div><strong>Name:</strong> {itemSelectedProduct[index]!.name}</div>
                        <div><strong>SKU:</strong> {itemSelectedProduct[index]!.sku}</div>
                        <div><strong>Unit Price:</strong> ${itemSelectedProduct[index]!.unit_price.toFixed(2)}</div>
                        {itemSelectedProduct[index]!.description && (
                          <div><strong>Description:</strong> {itemSelectedProduct[index]!.description}</div>
                        )}
                        {itemSelectedProduct[index]!.brand && (
                          <div><strong>Brand:</strong> {itemSelectedProduct[index]!.brand}</div>
                        )}
                        {itemSelectedProduct[index]!.model && (
                          <div><strong>Model:</strong> {itemSelectedProduct[index]!.model}</div>
                        )}
                        <div><strong>Category:</strong> {itemSelectedProduct[index]!.category}</div>
                        <div><strong>VAT:</strong> {itemSelectedProduct[index]!.vat}%</div>
                      </div>
                    </div>
                  )}
                </div>

                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '1rem', marginBottom: '1rem' }}>
                  <div>
                    <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333', fontSize: '0.875rem' }}>
                      Quantity *
                    </label>
                    <input
                      type="text"
                      value={item.quantity}
                      onChange={(e) => {
                        updateItem(index, 'quantity', e.target.value);
                        calculateItemTotal(index);
                      }}
                      style={{
                        width: '100%',
                        padding: '0.5rem',
                        border: `1px solid ${errors[`items.${index}.quantity`] ? '#dc3545' : '#ced4da'}`,
                        borderRadius: '4px',
                        fontSize: '0.875rem',
                        boxSizing: 'border-box',
                      }}
                      placeholder="0"
                    />
                    {errors[`items.${index}.quantity`] && (
                      <p style={{ margin: '0.25rem 0 0 0', color: '#dc3545', fontSize: '0.75rem' }}>
                        {errors[`items.${index}.quantity`]}
                      </p>
                    )}
                  </div>
                  <div>
                    <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333', fontSize: '0.875rem' }}>
                      Unit Price * {itemSelectedProduct[index] && <span style={{ fontSize: '0.75rem', color: '#666', fontWeight: 'normal' }}>(from product)</span>}
                    </label>
                    <input
                      type="text"
                      value={item.unitPrice}
                      onChange={(e) => {
                        updateItem(index, 'unitPrice', e.target.value);
                        calculateItemTotal(index);
                      }}
                      readOnly={!!itemSelectedProduct[index]}
                      style={{
                        width: '100%',
                        padding: '0.5rem',
                        border: `1px solid ${errors[`items.${index}.unitPrice`] ? '#dc3545' : '#ced4da'}`,
                        borderRadius: '4px',
                        fontSize: '0.875rem',
                        boxSizing: 'border-box',
                        backgroundColor: itemSelectedProduct[index] ? '#e9ecef' : 'white',
                      }}
                      placeholder="0.00"
                    />
                    {errors[`items.${index}.unitPrice`] && (
                      <p style={{ margin: '0.25rem 0 0 0', color: '#dc3545', fontSize: '0.75rem' }}>
                        {errors[`items.${index}.unitPrice`]}
                      </p>
                    )}
                  </div>
                  <div>
                    <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333', fontSize: '0.875rem' }}>
                      Total
                    </label>
                    <input
                      type="text"
                      value={item.total}
                      readOnly
                      style={{
                        width: '100%',
                        padding: '0.5rem',
                        border: '1px solid #ced4da',
                        borderRadius: '4px',
                        fontSize: '0.875rem',
                        boxSizing: 'border-box',
                        backgroundColor: '#e9ecef',
                      }}
                    />
                  </div>
                </div>
                <div style={{ marginBottom: '1rem' }}>
                  <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333', fontSize: '0.875rem' }}>
                    Description * {itemSelectedProduct[index] && <span style={{ fontSize: '0.75rem', color: '#666', fontWeight: 'normal' }}>(from product)</span>}
                  </label>
                  <input
                    type="text"
                    value={item.description}
                    onChange={(e) => updateItem(index, 'description', e.target.value)}
                    readOnly={!!itemSelectedProduct[index]}
                    style={{
                      width: '100%',
                      padding: '0.5rem',
                      border: `1px solid ${errors[`items.${index}.description`] ? '#dc3545' : '#ced4da'}`,
                      borderRadius: '4px',
                      fontSize: '0.875rem',
                      boxSizing: 'border-box',
                      backgroundColor: itemSelectedProduct[index] ? '#e9ecef' : 'white',
                    }}
                    placeholder="Enter item description"
                  />
                  {errors[`items.${index}.description`] && (
                    <p style={{ margin: '0.25rem 0 0 0', color: '#dc3545', fontSize: '0.75rem' }}>
                      {errors[`items.${index}.description`]}
                    </p>
                  )}
                </div>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '1rem' }}>
                  <div>
                    <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333', fontSize: '0.875rem' }}>
                      Brand {itemSelectedProduct[index] && <span style={{ fontSize: '0.75rem', color: '#666', fontWeight: 'normal' }}>(from product)</span>}
                    </label>
                    <input
                      type="text"
                      value={item.brand}
                      onChange={(e) => updateItem(index, 'brand', e.target.value)}
                      readOnly={!!itemSelectedProduct[index]}
                      style={{
                        width: '100%',
                        padding: '0.5rem',
                        border: '1px solid #ced4da',
                        borderRadius: '4px',
                        fontSize: '0.875rem',
                        boxSizing: 'border-box',
                        backgroundColor: itemSelectedProduct[index] ? '#e9ecef' : 'white',
                      }}
                      placeholder="Enter brand"
                    />
                  </div>
                  <div>
                    <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333', fontSize: '0.875rem' }}>
                      Model {itemSelectedProduct[index] && <span style={{ fontSize: '0.75rem', color: '#666', fontWeight: 'normal' }}>(from product)</span>}
                    </label>
                    <input
                      type="text"
                      value={item.model}
                      onChange={(e) => updateItem(index, 'model', e.target.value)}
                      readOnly={!!itemSelectedProduct[index]}
                      style={{
                        width: '100%',
                        padding: '0.5rem',
                        border: '1px solid #ced4da',
                        borderRadius: '4px',
                        fontSize: '0.875rem',
                        boxSizing: 'border-box',
                        backgroundColor: itemSelectedProduct[index] ? '#e9ecef' : 'white',
                      }}
                      placeholder="Enter model"
                    />
                  </div>
                  <div>
                    <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333', fontSize: '0.875rem' }}>
                      Code {itemSelectedProduct[index] && <span style={{ fontSize: '0.75rem', color: '#666', fontWeight: 'normal' }}>(from product)</span>}
                    </label>
                    <input
                      type="text"
                      value={item.code}
                      onChange={(e) => updateItem(index, 'code', e.target.value)}
                      readOnly={!!itemSelectedProduct[index]}
                      style={{
                        width: '100%',
                        padding: '0.5rem',
                        border: '1px solid #ced4da',
                        borderRadius: '4px',
                        fontSize: '0.875rem',
                        boxSizing: 'border-box',
                        backgroundColor: itemSelectedProduct[index] ? '#e9ecef' : 'white',
                      }}
                      placeholder="Enter code"
                    />
                  </div>
                </div>
              </div>
            ))}
          </div>

          {/* Amounts Section */}
          <div style={{ marginBottom: '2rem' }}>
            <h3 style={{ marginTop: 0, marginBottom: '1rem', fontSize: '1.2rem', fontWeight: '600' }}>
              Invoice Amounts
            </h3>
            
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
              <div>
                <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333' }}>
                  Total Amount *
                </label>
                <input
                  type="text"
                  value={formData.amounts.totalAmount}
                  onChange={(e) => handleChange('amounts.totalAmount', e.target.value)}
                  style={{
                    width: '100%',
                    padding: '0.75rem',
                    border: `1px solid ${errors['amounts.totalAmount'] ? '#dc3545' : '#ced4da'}`,
                    borderRadius: '4px',
                    fontSize: '1rem',
                    boxSizing: 'border-box',
                  }}
                  placeholder="0.00"
                />
                {errors['amounts.totalAmount'] && (
                  <p style={{ margin: '0.25rem 0 0 0', color: '#dc3545', fontSize: '0.875rem' }}>
                    {errors['amounts.totalAmount']}
                  </p>
                )}
              </div>

              <div>
                <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333' }}>
                  Discount Amount
                </label>
                <input
                  type="text"
                  value={formData.amounts.discountAmount}
                  onChange={(e) => handleChange('amounts.discountAmount', e.target.value)}
                  style={{
                    width: '100%',
                    padding: '0.75rem',
                    border: '1px solid #ced4da',
                    borderRadius: '4px',
                    fontSize: '1rem',
                    boxSizing: 'border-box',
                  }}
                  placeholder="0.00"
                />
              </div>

              <div>
                <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333' }}>
                  Tax Amount
                </label>
                <input
                  type="text"
                  value={formData.amounts.taxAmount}
                  onChange={(e) => handleChange('amounts.taxAmount', e.target.value)}
                  style={{
                    width: '100%',
                    padding: '0.75rem',
                    border: '1px solid #ced4da',
                    borderRadius: '4px',
                    fontSize: '1rem',
                    boxSizing: 'border-box',
                  }}
                  placeholder="0.00"
                />
              </div>

              <div>
                <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: '500', color: '#333' }}>
                  Pay Amount *
                </label>
                <input
                  type="text"
                  value={formData.amounts.payAmount}
                  onChange={(e) => handleChange('amounts.payAmount', e.target.value)}
                  style={{
                    width: '100%',
                    padding: '0.75rem',
                    border: `1px solid ${errors['amounts.payAmount'] ? '#dc3545' : '#ced4da'}`,
                    borderRadius: '4px',
                    fontSize: '1rem',
                    boxSizing: 'border-box',
                  }}
                  placeholder="0.00"
                />
                {errors['amounts.payAmount'] && (
                  <p style={{ margin: '0.25rem 0 0 0', color: '#dc3545', fontSize: '0.875rem' }}>
                    {errors['amounts.payAmount']}
                  </p>
                )}
              </div>
            </div>
          </div>

          {/* Form Actions */}
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
              {isLoading ? 'Creating...' : 'Create Invoice'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

