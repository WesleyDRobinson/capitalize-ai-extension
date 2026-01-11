// Zustand store for app state
import { create } from 'zustand';
import { Conversation, Message } from '../db';

interface StreamingState {
  isStreaming: boolean;
  streamingContent: string;
  streamingConversationId: string | null;
}

export type ToastType = 'error' | 'success' | 'info' | 'warning';

export interface Toast {
  id: string;
  type: ToastType;
  message: string;
  duration?: number;
}

interface AppState {
  // Auth
  isAuthenticated: boolean;
  setAuthenticated: (value: boolean) => void;

  // Conversations
  conversations: Conversation[];
  currentConversationId: string | null;
  setConversations: (conversations: Conversation[]) => void;
  addConversation: (conversation: Conversation) => void;
  updateConversation: (id: string, updates: Partial<Conversation>) => void;
  removeConversation: (id: string) => void;
  setCurrentConversation: (id: string | null) => void;

  // Messages
  messages: Message[];
  setMessages: (messages: Message[]) => void;
  addMessage: (message: Message) => void;
  updateMessage: (id: string, updates: Partial<Message>) => void;

  // Streaming
  streaming: StreamingState;
  startStreaming: (conversationId: string) => void;
  appendStreamingContent: (token: string) => void;
  stopStreaming: () => void;

  // UI State
  sidebarOpen: boolean;
  toggleSidebar: () => void;
  setSidebarOpen: (open: boolean) => void;

  // Connection
  isConnected: boolean;
  setConnected: (connected: boolean) => void;

  // Sync
  isSyncing: boolean;
  setSyncing: (syncing: boolean) => void;

  // Toasts/Notifications
  toasts: Toast[];
  addToast: (type: ToastType, message: string, duration?: number) => void;
  removeToast: (id: string) => void;
  clearToasts: () => void;
}

export const useAppStore = create<AppState>((set) => ({
  // Auth
  isAuthenticated: false,
  setAuthenticated: (value) => set({ isAuthenticated: value }),

  // Conversations
  conversations: [],
  currentConversationId: null,
  setConversations: (conversations) => set({ conversations }),
  addConversation: (conversation) =>
    set((state) => ({
      conversations: [conversation, ...state.conversations]
    })),
  updateConversation: (id, updates) =>
    set((state) => ({
      conversations: state.conversations.map((c) =>
        c.id === id ? { ...c, ...updates } : c
      )
    })),
  removeConversation: (id) =>
    set((state) => ({
      conversations: state.conversations.filter((c) => c.id !== id),
      currentConversationId:
        state.currentConversationId === id ? null : state.currentConversationId
    })),
  setCurrentConversation: (id) => set({ currentConversationId: id }),

  // Messages
  messages: [],
  setMessages: (messages) => set({ messages }),
  addMessage: (message) =>
    set((state) => ({
      messages: [...state.messages, message]
    })),
  updateMessage: (id, updates) =>
    set((state) => ({
      messages: state.messages.map((m) =>
        m.id === id ? { ...m, ...updates } : m
      )
    })),

  // Streaming
  streaming: {
    isStreaming: false,
    streamingContent: '',
    streamingConversationId: null
  },
  startStreaming: (conversationId) =>
    set({
      streaming: {
        isStreaming: true,
        streamingContent: '',
        streamingConversationId: conversationId
      }
    }),
  appendStreamingContent: (token) =>
    set((state) => ({
      streaming: {
        ...state.streaming,
        streamingContent: state.streaming.streamingContent + token
      }
    })),
  stopStreaming: () =>
    set({
      streaming: {
        isStreaming: false,
        streamingContent: '',
        streamingConversationId: null
      }
    }),

  // UI State
  sidebarOpen: true,
  toggleSidebar: () => set((state) => ({ sidebarOpen: !state.sidebarOpen })),
  setSidebarOpen: (open) => set({ sidebarOpen: open }),

  // Connection
  isConnected: true,
  setConnected: (connected) => set({ isConnected: connected }),

  // Sync
  isSyncing: false,
  setSyncing: (syncing) => set({ isSyncing: syncing }),

  // Toasts/Notifications
  toasts: [],
  addToast: (type, message, duration = 5000) => {
    const id = `toast_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
    set((state) => ({
      toasts: [...state.toasts, { id, type, message, duration }]
    }));
    // Auto-remove after duration
    if (duration > 0) {
      setTimeout(() => {
        set((state) => ({
          toasts: state.toasts.filter((t) => t.id !== id)
        }));
      }, duration);
    }
  },
  removeToast: (id) =>
    set((state) => ({
      toasts: state.toasts.filter((t) => t.id !== id)
    })),
  clearToasts: () => set({ toasts: [] })
}));
