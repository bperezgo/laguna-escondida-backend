export type CommandStatus = "pending" | "completed" | "cancelled";

export interface CommandItem {
  id: string;
  product_id: string;
  product_name: string;
  quantity: number;
  notes?: string;
}

export interface OpenBillCreator {
  id: string;
  user_name: string;
}

export interface Command {
  id: string;
  open_bill_id: string;
  temporal_identifier: string;
  created_by?: OpenBillCreator;
  area: string;
  status: CommandStatus;
  items: CommandItem[];
  created_at: string;
  updated_at: string;
}
