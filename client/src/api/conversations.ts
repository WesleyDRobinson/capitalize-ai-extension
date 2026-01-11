// Conversation API calls
import { api } from './client';
import { Conversation } from '../db';

interface CreateConversationRequest {
  title: string;
  metadata?: Record<string, string>;
}

interface UpdateConversationRequest {
  title?: string;
  metadata?: Record<string, string>;
}

interface ListConversationsResponse {
  conversations: Conversation[];
  total: number;
  has_more: boolean;
  next_cursor?: string;
}

export async function createConversation(
  data: CreateConversationRequest
): Promise<Conversation> {
  return api.post<Conversation>('/conversations', data);
}

export async function listConversations(
  limit: number = 20,
  offset: number = 0
): Promise<ListConversationsResponse> {
  return api.get<ListConversationsResponse>(
    `/conversations?limit=${limit}&offset=${offset}`
  );
}

export async function getConversation(id: string): Promise<Conversation> {
  return api.get<Conversation>(`/conversations/${id}`);
}

export async function updateConversation(
  id: string,
  data: UpdateConversationRequest
): Promise<Conversation> {
  return api.put<Conversation>(`/conversations/${id}`, data);
}

export async function deleteConversation(id: string): Promise<void> {
  return api.delete(`/conversations/${id}`);
}
