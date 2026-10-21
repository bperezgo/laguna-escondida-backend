import { apiRequest } from './config';
import type {
  CreateDeliveryZoneRequest,
  DeliveryPointCheck,
  DeliveryZone,
  DeliveryZoneListResponse,
  UpdateDeliveryZoneRequest,
} from '@/types/deliveryZone';

export const deliveryZonesApi = {
  async getAll(): Promise<DeliveryZone[]> {
    const response = await apiRequest<DeliveryZoneListResponse>('/delivery-zones');
    return response.delivery_zones || [];
  },

  async create(zone: CreateDeliveryZoneRequest): Promise<DeliveryZone> {
    return apiRequest<DeliveryZone>('/delivery-zones', {
      method: 'POST',
      body: JSON.stringify(zone),
    });
  },

  async update(id: string, zone: UpdateDeliveryZoneRequest): Promise<DeliveryZone> {
    return apiRequest<DeliveryZone>(`/delivery-zones/${id}`, {
      method: 'PUT',
      body: JSON.stringify(zone),
    });
  },

  /**
   * Soft delete; orders already recorded in the zone keep it.
   */
  async delete(id: string): Promise<void> {
    return apiRequest<void>(`/delivery-zones/${id}`, {
      method: 'DELETE',
    });
  },

  async check(lat: number, lng: number): Promise<DeliveryPointCheck> {
    const query = new URLSearchParams({ lat: String(lat), lng: String(lng) });
    return apiRequest<DeliveryPointCheck>(`/delivery-zones/check?${query}`);
  },
};
