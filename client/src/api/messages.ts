// Message API calls
import { api } from './client';
import { Message } from '../db';

interface SendMessageRequest {
  content: string;
  model?: string;
  stream?: boolean;
}

interface SendMessageResponse {
  message: Message;
  sequence: number;
}

interface ListMessagesResponse {
  messages: Message[];
  has_more: boolean;
  last_sequence: number;
  stream_active: boolean;
}

export async function sendMessage(
  conversationId: string,
  data: SendMessageRequest
): Promise<SendMessageResponse> {
  return api.post<SendMessageResponse>(
    `/conversations/${conversationId}/messages`,
    data
  );
}

export async function listMessages(
  conversationId: string,
  afterSequence: number = 0,
  limit: number = 50
): Promise<ListMessagesResponse> {
  return api.get<ListMessagesResponse>(
    `/conversations/${conversationId}/messages?after_sequence=${afterSequence}&limit=${limit}`
  );
}
