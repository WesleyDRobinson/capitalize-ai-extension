// IndexedDB schema via Dexie.js
import Dexie, { Table } from 'dexie';

export interface Conversation {
  id: string;
  tenantId: string;
  title: string;
  createdAt: Date;
  updatedAt: Date;
  messageCount?: number;
}

export interface Message {
  id: string;
  conversationId: string;
  role: 'user' | 'assistant' | 'system' | 'tool';
  content: string;
  sequence: number;
  createdAt: Date;
  model?: string;
  tokensIn?: number;
  tokensOut?: number;
}

export interface PendingMessage {
  id?: number;
  conversationId: string;
  content: string;
  status: 'pending' | 'sending' | 'failed';
  createdAt: Date;
  retryCount: number;
}

export interface SyncState {
  conversationId: string;
  lastSequence: number;
  lastSyncAt: Date;
  status: 'synced' | 'syncing' | 'stale';
}

class ConversationsDB extends Dexie {
  conversations!: Table<Conversation>;
  messages!: Table<Message>;
  pendingMessages!: Table<PendingMessage>;
  syncState!: Table<SyncState>;

  constructor() {
    super('ConversationsDB');
    this.version(1).stores({
      conversations: 'id, tenantId, updatedAt',
      messages: 'id, conversationId, sequence, createdAt',
      pendingMessages: '++id, conversationId, status',
      syncState: 'conversationId'
    });
  }
}

export const db = new ConversationsDB();

// Helper functions
export async function getConversationMessages(conversationId: string): Promise<Message[]> {
  return db.messages
    .where('conversationId')
    .equals(conversationId)
    .sortBy('sequence');
}

export async function getLastSequence(conversationId: string): Promise<number> {
  const syncState = await db.syncState.get(conversationId);
  return syncState?.lastSequence ?? 0;
}

export async function updateSyncState(
  conversationId: string,
  lastSequence: number,
  status: 'synced' | 'syncing' | 'stale'
): Promise<void> {
  await db.syncState.put({
    conversationId,
    lastSequence,
    lastSyncAt: new Date(),
    status
  });
}

export async function clearConversationData(conversationId: string): Promise<void> {
  await db.messages.where('conversationId').equals(conversationId).delete();
  await db.syncState.delete(conversationId);
  await db.pendingMessages.where('conversationId').equals(conversationId).delete();
}
