// SSE Stream client with reconnection logic
import { fetchEventSource } from '@microsoft/fetch-event-source';
import { db, Message, updateSyncState } from '../db';
import { getAccessToken } from './auth';

interface TokenEvent {
  token: string;
  index: number;
}

interface MessageCompleteEvent {
  message: Message;
  sequence: number;
}

interface StreamOptions {
  onToken: (token: string, index: number) => void;
  onComplete: (message: Message) => void;
  onError: (error: Error) => void;
  onConnected?: () => void;
  onDisconnected?: () => void;
}

class ConversationStream {
  private controller: AbortController | null = null;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 5;
  private conversationId: string | null = null;

  async connect(
    conversationId: string,
    options: StreamOptions
  ): Promise<void> {
    this.controller = new AbortController();
    this.reconnectAttempts = 0;
    this.conversationId = conversationId;

    const { onToken, onComplete, onError, onConnected, onDisconnected } = options;

    const connect = async () => {
      try {
        await fetchEventSource(
          `/api/v1/conversations/${conversationId}/stream`,
          {
            signal: this.controller!.signal,
            headers: {
              'Authorization': `Bearer ${getAccessToken()}`,
            },

            onopen: async (response) => {
              if (response.ok) {
                this.reconnectAttempts = 0;
                console.log('SSE connection established');
                onConnected?.();
              } else {
                throw new Error(`SSE connection failed: ${response.status}`);
              }
            },

            onmessage: (event) => {
              switch (event.event) {
                case 'token': {
                  const data: TokenEvent = JSON.parse(event.data);
                  onToken(data.token, data.index);
                  break;
                }
                case 'message_complete': {
                  const data: MessageCompleteEvent = JSON.parse(event.data);
                  // Persist to IndexedDB
                  db.messages.put(data.message);
                  updateSyncState(conversationId, data.sequence, 'synced');
                  onComplete(data.message);
                  break;
                }
                case 'error': {
                  const data = JSON.parse(event.data);
                  onError(new Error(data.message));
                  break;
                }
                case 'heartbeat':
                  // Connection alive, no action needed
                  break;
                case 'connected':
                  console.log('Stream connected:', event.data);
                  break;
              }
            },

            onerror: (err) => {
              if (this.reconnectAttempts < this.maxReconnectAttempts) {
                this.reconnectAttempts++;
                const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000);
                console.log(`SSE reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`);
                setTimeout(() => this.handleReconnect(conversationId), delay);
              } else {
                onError(new Error('Max reconnection attempts reached'));
              }
              throw err;
            },

            onclose: () => {
              console.log('SSE connection closed');
              onDisconnected?.();
              this.syncMissedMessages(conversationId);
            }
          }
        );
      } catch (err) {
        if (err instanceof Error && err.name !== 'AbortError') {
          onError(err);
        }
      }
    };

    await connect();
  }

  private async handleReconnect(conversationId: string): Promise<void> {
    await this.syncMissedMessages(conversationId);
  }

  private async syncMissedMessages(conversationId: string): Promise<void> {
    try {
      const syncState = await db.syncState.get(conversationId);
      const lastSequence = syncState?.lastSequence ?? 0;

      const response = await fetch(
        `/api/v1/conversations/${conversationId}/messages?after_sequence=${lastSequence}&limit=100`,
        {
          headers: {
            'Authorization': `Bearer ${getAccessToken()}`,
          }
        }
      );

      if (!response.ok) {
        throw new Error(`Sync failed: ${response.status}`);
      }

      const { messages, last_sequence, has_more } = await response.json();

      // Bulk insert to IndexedDB
      if (messages.length > 0) {
        await db.messages.bulkPut(messages);
      }

      // Update sync state
      await updateSyncState(
        conversationId,
        last_sequence,
        has_more ? 'stale' : 'synced'
      );

      // If more messages exist, continue syncing
      if (has_more) {
        await this.syncMissedMessages(conversationId);
      }
    } catch (err) {
      console.error('Failed to sync messages:', err);
      await updateSyncState(conversationId, 0, 'stale');
    }
  }

  disconnect(): void {
    this.controller?.abort();
    this.controller = null;
    this.conversationId = null;
  }

  isConnected(): boolean {
    return this.controller !== null;
  }
}

export const conversationStream = new ConversationStream();

// Stream with message - sends a message and streams the response
export async function streamMessage(
  conversationId: string,
  content: string,
  model: string = 'claude-3-5-sonnet-20241022',
  options: StreamOptions
): Promise<void> {
  const controller = new AbortController();

  try {
    await fetchEventSource(
      `/api/v1/conversations/${conversationId}/stream`,
      {
        method: 'POST',
        signal: controller.signal,
        headers: {
          'Authorization': `Bearer ${getAccessToken()}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          content,
          model,
          stream: true
        }),

        onopen: async (response) => {
          if (!response.ok) {
            throw new Error(`Stream failed: ${response.status}`);
          }
          options.onConnected?.();
        },

        onmessage: (event) => {
          switch (event.event) {
            case 'token': {
              const data: TokenEvent = JSON.parse(event.data);
              options.onToken(data.token, data.index);
              break;
            }
            case 'message_complete': {
              const data: MessageCompleteEvent = JSON.parse(event.data);
              db.messages.put(data.message);
              updateSyncState(conversationId, data.sequence, 'synced');
              options.onComplete(data.message);
              break;
            }
            case 'user_message': {
              const message: Message = JSON.parse(event.data);
              db.messages.put(message);
              break;
            }
            case 'error': {
              const data = JSON.parse(event.data);
              options.onError(new Error(data.message));
              break;
            }
            case 'done':
              console.log('Stream completed');
              break;
          }
        },

        onerror: (err) => {
          options.onError(err instanceof Error ? err : new Error('Stream error'));
          throw err;
        },

        onclose: () => {
          options.onDisconnected?.();
        }
      }
    );
  } catch (err) {
    if (err instanceof Error && err.name !== 'AbortError') {
      options.onError(err);
    }
  }
}
