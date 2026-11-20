import type {
  CourierListResponse,
  CourierQueue,
  DeliveryHistory,
  DeliveryListFilter,
  DeliveryOrder,
  DeliveryOrderListResponse,
  DeliveryStatus,
  DispatchDeliveryRequest,
  FailDeliveryRequest,
} from "../../types/delivery.ts";

/** Keeps the backend's status and code, and the order's current status on a 409. */
export class DeliveryApiError extends Error {
  status: number;
  code?: string;
  field?: string;
  currentStatus?: DeliveryStatus | null;

  constructor(
    message: string,
    status: number,
    code?: string,
    field?: string,
    currentStatus?: DeliveryStatus | null,
  ) {
    super(message);
    this.name = "DeliveryApiError";
    this.status = status;
    this.code = code;
    this.field = field;
    this.currentStatus = currentStatus;
  }
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`/api/deliveries${path}`, {
    ...options,
    headers: { "Content-Type": "application/json", ...options?.headers },
  });
  if (response.status === 401) {
    window.location.href = "/signin";
    throw new DeliveryApiError("Unauthorized", 401);
  }
  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    throw new DeliveryApiError(
      body.error || `HTTP error! status: ${response.status}`,
      response.status,
      body.code,
      body.field,
      body.current_status,
    );
  }
  return response.json();
}

function post<T>(path: string, body?: object): Promise<T> {
  return request<T>(path, { method: "POST", body: body ? JSON.stringify(body) : undefined });
}

export const deliveriesApi = {
  list(filter: DeliveryListFilter = {}): Promise<DeliveryOrderListResponse> {
    const query = new URLSearchParams();
    if (filter.status) query.set("status", filter.status);
    if (filter.order_type) query.set("order_type", filter.order_type);
    return request(`/orders${query.size > 0 ? `?${query}` : ""}`);
  },

  history(orderId: string): Promise<DeliveryHistory> {
    return request(`/orders/${encodeURIComponent(orderId)}/history`);
  },

  couriers(): Promise<CourierListResponse> {
    return request("/couriers");
  },

  mine(): Promise<CourierQueue> {
    return request("/mine");
  },

  /** A courier sends no body to claim the order for themselves. */
  dispatch(orderId: string, courier?: DispatchDeliveryRequest): Promise<DeliveryOrder> {
    return post(`/orders/${encodeURIComponent(orderId)}/dispatch`, courier);
  },

  deliver(orderId: string): Promise<DeliveryOrder> {
    return post(`/orders/${encodeURIComponent(orderId)}/deliver`);
  },

  fail(orderId: string, failure: FailDeliveryRequest): Promise<DeliveryOrder> {
    return post(`/orders/${encodeURIComponent(orderId)}/fail`, failure);
  },
};
