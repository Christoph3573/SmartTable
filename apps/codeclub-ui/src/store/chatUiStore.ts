import { create } from "zustand";

/**
 * Tracks which chat channel is currently open on screen. The global
 * WebSocket listener (mounted once in AppLayout) reads this to know whether
 * an incoming live message should also mark the channel as read, since that
 * listener has no view into ChatPage's local state.
 */
type ChatUiState = {
  activeChannelId?: number;
  setActiveChannelId: (id: number | undefined) => void;
};

export const useChatUiStore = create<ChatUiState>((set) => ({
  activeChannelId: undefined,
  setActiveChannelId: (id) => set({ activeChannelId: id }),
}));
