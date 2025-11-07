'use client';

import { useState, useMemo } from 'react';
import { TableSelector, ProductSearch, OrderItemCard } from '@/components/orders';
import type { Table, OrderItem } from '@/types/order';
import { createTable, createOrder, searchProducts } from '@/lib/api/orders';

// Mock product type for display
interface Product {
  ID: string;
  Name: string;
  Image?: string;
  Price: number;
  VAT: number;
  Category: string;
}

export default function OrdersPage() {
  const [selectedTable, setSelectedTable] = useState<Table | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [products, setProducts] = useState<Product[]>([]);
  const [orderItems, setOrderItems] = useState<Map<string, { product: Product; quantity: number; comment: string }>>(new Map());
  const [isLoading, setIsLoading] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);

  // Mock products for demonstration - will be replaced with API call
  const mockProducts: Product[] = [
    { ID: '1', Name: 'Caesar Salad', Price: 12.99, VAT: 10, Category: 'Salads', Image: undefined },
    { ID: '2', Name: 'Grilled Salmon', Price: 24.99, VAT: 10, Category: 'Main Course', Image: undefined },
    { ID: '3', Name: 'Chocolate Cake', Price: 8.99, VAT: 10, Category: 'Desserts', Image: undefined },
    { ID: '4', Name: 'Margherita Pizza', Price: 15.99, VAT: 10, Category: 'Pizza', Image: undefined },
    { ID: '5', Name: 'Chicken Burger', Price: 14.99, VAT: 10, Category: 'Burgers', Image: undefined },
    { ID: '6', Name: 'Fish Tacos', Price: 16.99, VAT: 10, Category: 'Mexican', Image: undefined },
  ];

  // Filter products based on search query
  const filteredProducts = useMemo(() => {
    if (!searchQuery.trim()) {
      return mockProducts;
    }
    const query = searchQuery.toLowerCase();
    return mockProducts.filter(product =>
      product.Name.toLowerCase().includes(query) ||
      product.Category.toLowerCase().includes(query)
    );
  }, [searchQuery]);

  const handleTableSelect = (table: Table) => {
    setSelectedTable(table);
    // Reset order when selecting a new table
    setOrderItems(new Map());
  };

  const handleCreateNewTable = async (tableNumber: number) => {
    try {
      // TODO: Replace with actual API call when backend is ready
      // const response = await createTable({ Number: tableNumber });
      // setSelectedTable(response.table);
      
      // Mock response for now
      const newTable: Table = {
        ID: `table-${tableNumber}`,
        Number: tableNumber,
        Status: 'available',
        CreatedAt: new Date().toISOString(),
        UpdatedAt: new Date().toISOString(),
      };
      setSelectedTable(newTable);
    } catch (error) {
      console.error('Error creating table:', error);
      throw error;
    }
  };

  const handleQuantityChange = (productId: string, delta: number) => {
    setOrderItems(prev => {
      const newMap = new Map(prev);
      const existing = newMap.get(productId);
      const product = mockProducts.find(p => p.ID === productId);
      
      if (!product) return prev;

      if (existing) {
        const newQuantity = Math.max(0, existing.quantity + delta);
        if (newQuantity === 0) {
          newMap.delete(productId);
        } else {
          newMap.set(productId, {
            ...existing,
            quantity: newQuantity,
          });
        }
      } else if (delta > 0) {
        newMap.set(productId, {
          product,
          quantity: 1,
          comment: '',
        });
      }
      
      return newMap;
    });
  };

  const handleCommentChange = (productId: string, comment: string) => {
    setOrderItems(prev => {
      const newMap = new Map(prev);
      const existing = newMap.get(productId);
      
      if (existing) {
        newMap.set(productId, {
          ...existing,
          comment,
        });
      }
      
      return newMap;
    });
  };

  const calculateTotal = () => {
    let total = 0;
    orderItems.forEach(({ product, quantity }) => {
      const unitPrice = product.Price + (product.Price * product.VAT / 100);
      total += unitPrice * quantity;
    });
    return total;
  };

  const handleSubmitOrder = async () => {
    if (!selectedTable) {
      alert('Please select a table first');
      return;
    }

    if (orderItems.size === 0) {
      alert('Please add at least one item to the order');
      return;
    }

    setIsSubmitting(true);
    try {
      const items: OrderItem[] = Array.from(orderItems.values()).map(({ product, quantity, comment }) => ({
        ProductID: product.ID,
        ProductName: product.Name,
        ProductImage: product.Image,
        Quantity: quantity,
        UnitPrice: product.Price,
        Comment: comment || undefined,
      }));

      // TODO: Replace with actual API call when backend is ready
      // const response = await createOrder({
      //   TableID: selectedTable.ID,
      //   Items: items,
      // });
      
      // Mock success for now
      console.log('Order would be created:', {
        TableID: selectedTable.ID,
        Items: items,
      });

      alert(`Order created successfully for Table ${selectedTable.Number}!`);
      
      // Reset order after successful submission
      setOrderItems(new Map());
    } catch (error) {
      console.error('Error creating order:', error);
      alert('Failed to create order. Please try again.');
    } finally {
      setIsSubmitting(false);
    }
  };

  // Get products that are in the order
  const orderedProducts = useMemo(() => {
    return Array.from(orderItems.values()).map(({ product, quantity, comment }) => ({
      product,
      quantity,
      comment,
    }));
  }, [orderItems]);

  return (
    <div style={{
      minHeight: '100vh',
      backgroundColor: '#f5f5f5',
      padding: '0.5rem',
    }}>
      <div style={{
        maxWidth: '1200px',
        margin: '0 auto',
      }}>
        <h1 style={{
          fontSize: 'clamp(1.5rem, 4vw, 2rem)',
          fontWeight: 'bold',
          marginBottom: '1rem',
          color: '#333',
          padding: '0.5rem',
        }}>
          Take Order
        </h1>

        {/* Table Selector */}
        <TableSelector
          selectedTable={selectedTable}
          onTableSelect={handleTableSelect}
          onCreateNewTable={handleCreateNewTable}
        />

        {selectedTable && (
          <>
            {/* Product Search */}
            <ProductSearch
              searchQuery={searchQuery}
              onSearchChange={setSearchQuery}
            />

            <div style={{
              display: 'grid',
              gridTemplateColumns: '1fr',
              gap: '1.5rem',
              marginTop: '1rem',
            }}>
              {/* Available Products */}
              <div>
                <h2 style={{
                  fontSize: '1.25rem',
                  fontWeight: 'bold',
                  marginBottom: '1rem',
                  color: '#333',
                }}>
                  Available Products
                </h2>
                <div style={{
                  display: 'grid',
                  gridTemplateColumns: 'repeat(auto-fill, minmax(min(280px, 100%), 1fr))',
                  gap: '0.75rem',
                }}>
                  {filteredProducts.map(product => {
                    const orderItem = orderItems.get(product.ID);
                    const quantity = orderItem?.quantity || 0;
                    
                    return (
                      <div
                        key={product.ID}
                        style={{
                          padding: '1rem',
                          backgroundColor: 'white',
                          border: '1px solid #e0e0e0',
                          borderRadius: '8px',
                          cursor: 'pointer',
                          transition: 'all 0.2s',
                          boxShadow: quantity > 0 ? '0 4px 8px rgba(0,123,255,0.2)' : '0 2px 4px rgba(0,0,0,0.1)',
                        }}
                        onClick={() => handleQuantityChange(product.ID, 1)}
                        onMouseEnter={(e) => {
                          e.currentTarget.style.transform = 'translateY(-2px)';
                          e.currentTarget.style.boxShadow = '0 4px 12px rgba(0,0,0,0.15)';
                        }}
                        onMouseLeave={(e) => {
                          e.currentTarget.style.transform = 'translateY(0)';
                          e.currentTarget.style.boxShadow = quantity > 0 ? '0 4px 8px rgba(0,123,255,0.2)' : '0 2px 4px rgba(0,0,0,0.1)';
                        }}
                      >
                        <div style={{
                          width: '100%',
                          height: '150px',
                          borderRadius: '8px',
                          backgroundColor: '#f8f9fa',
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'center',
                          marginBottom: '0.75rem',
                          overflow: 'hidden',
                          border: '1px solid #e0e0e0',
                        }}>
                          {product.Image ? (
                            <img
                              src={product.Image}
                              alt={product.Name}
                              style={{
                                width: '100%',
                                height: '100%',
                                objectFit: 'cover',
                              }}
                            />
                          ) : (
                            <span style={{ color: '#6c757d', fontSize: '3rem' }}>🍽️</span>
                          )}
                        </div>
                        <h3 style={{
                          margin: '0 0 0.5rem 0',
                          fontSize: '1rem',
                          fontWeight: 'bold',
                          color: '#333',
                        }}>
                          {product.Name}
                        </h3>
                        <p style={{
                          margin: '0 0 0.5rem 0',
                          color: '#666',
                          fontSize: '0.875rem',
                        }}>
                          {product.Category}
                        </p>
                        <div style={{
                          display: 'flex',
                          justifyContent: 'space-between',
                          alignItems: 'center',
                        }}>
                          <span style={{
                            fontSize: '1.1rem',
                            fontWeight: 'bold',
                            color: '#28a745',
                          }}>
                            ${(product.Price + (product.Price * product.VAT / 100)).toFixed(2)}
                          </span>
                          {quantity > 0 && (
                            <span style={{
                              backgroundColor: '#007bff',
                              color: 'white',
                              padding: '0.25rem 0.75rem',
                              borderRadius: '12px',
                              fontSize: '0.875rem',
                              fontWeight: 'bold',
                            }}>
                              {quantity} in order
                            </span>
                          )}
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>

              {/* Order Items */}
              {orderedProducts.length > 0 && (
                <div style={{
                  position: 'sticky',
                  bottom: 0,
                  backgroundColor: 'white',
                  borderTop: '2px solid #e0e0e0',
                  padding: '1rem',
                  borderRadius: '8px 8px 0 0',
                  boxShadow: '0 -2px 8px rgba(0,0,0,0.1)',
                }}>
                  <h2 style={{
                    fontSize: '1.25rem',
                    fontWeight: 'bold',
                    marginBottom: '1rem',
                    color: '#333',
                  }}>
                    Current Order
                  </h2>
                  <div style={{
                    maxHeight: '400px',
                    overflowY: 'auto',
                    marginBottom: '1rem',
                  }}>
                    {orderedProducts.map(({ product, quantity, comment }) => (
                      <OrderItemCard
                        key={product.ID}
                        product={product}
                        quantity={quantity}
                        comment={comment}
                        onQuantityChange={handleQuantityChange}
                        onCommentChange={handleCommentChange}
                      />
                    ))}
                  </div>
                  <div style={{
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '1rem',
                    paddingTop: '1rem',
                    borderTop: '2px solid #e0e0e0',
                    marginTop: '1rem',
                  }}>
                    <div style={{
                      display: 'flex',
                      justifyContent: 'space-between',
                      alignItems: 'center',
                      flexWrap: 'wrap',
                      gap: '0.5rem',
                    }}>
                      <div>
                        <p style={{
                          margin: 0,
                          fontSize: '0.875rem',
                          color: '#666',
                        }}>
                          Total Items: {Array.from(orderItems.values()).reduce((sum, item) => sum + item.quantity, 0)}
                        </p>
                        <p style={{
                          margin: '0.5rem 0 0 0',
                          fontSize: 'clamp(1.25rem, 4vw, 1.5rem)',
                          fontWeight: 'bold',
                          color: '#333',
                        }}>
                          Total: ${calculateTotal().toFixed(2)}
                        </p>
                      </div>
                    </div>
                    <button
                      onClick={handleSubmitOrder}
                      disabled={isSubmitting || orderedProducts.length === 0}
                      style={{
                        padding: '1rem',
                        fontSize: 'clamp(1rem, 3vw, 1.1rem)',
                        fontWeight: 'bold',
                        backgroundColor: isSubmitting || orderedProducts.length === 0 ? '#6c757d' : '#28a745',
                        color: 'white',
                        border: 'none',
                        borderRadius: '8px',
                        cursor: isSubmitting || orderedProducts.length === 0 ? 'not-allowed' : 'pointer',
                        transition: 'background-color 0.2s',
                        width: '100%',
                      }}
                      onMouseEnter={(e) => {
                        if (!isSubmitting && orderedProducts.length > 0) {
                          e.currentTarget.style.backgroundColor = '#218838';
                        }
                      }}
                      onMouseLeave={(e) => {
                        if (!isSubmitting && orderedProducts.length > 0) {
                          e.currentTarget.style.backgroundColor = '#28a745';
                        }
                      }}
                    >
                      {isSubmitting ? 'Creating Order...' : 'Create Order'}
                    </button>
                  </div>
                </div>
              )}
            </div>
          </>
        )}

        {!selectedTable && (
          <div style={{
            textAlign: 'center',
            padding: '3rem',
            color: '#666',
            fontSize: '1.1rem',
          }}>
            Please select or create a table to start taking orders
          </div>
        )}
      </div>
    </div>
  );
}

