export interface TabAttentionState {
  count: number;
  baseTitle: string | null;
}

export function formatTabTitle(count: number, baseTitle: string): string {
  return count > 0 ? `(${count}) ${baseTitle}` : baseTitle;
}

export interface TabAttention {
  setBaseTitle(title: string): void;
  add(): void;
  reset(): void;
  getState(): TabAttentionState;
}

export function createTabAttention(
  render: (state: TabAttentionState) => void
): TabAttention {
  const state: TabAttentionState = { count: 0, baseTitle: null };
  const update = (next: Partial<TabAttentionState>) => {
    Object.assign(state, next);
    render({ ...state });
  };

  return {
    setBaseTitle: (title) => update({ baseTitle: title }),
    add: () => update({ count: state.count + 1 }),
    reset: () => {
      if (state.count !== 0) update({ count: 0 });
    },
    getState: () => ({ ...state }),
  };
}
