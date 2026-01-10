// hooks/useSendMessage.ts
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { db, Message, PendingMessage } from '../db';
import { streamMessage } from '../lib/stream';
import { useAppStore } from '../store/useAppStore';

export function useSendMessage(conversationId: string) {
  const queryClient = useQueryClient();
  const {
    addMessage,
    startStreaming,
    appendStreamingContent,
    stopStreaming
  } = useAppStore();

  return useMutation({
    mutationFn: async (content: string) => {
      // Add to pending queue immediately
      const pendingId = await db.pendingMessages.add({
        conversationId,
        content,
        status: 'pending',
        createdAt: new Date(),
        retryCount: 0
      });

      // Optimistically add user message to UI
      const optimisticUserMessage: Message = {
        id: `pending_user_${pendingId}`,
        conversationId,
        role: 'user',
        content,
        sequence: -1,
        createdAt: new Date()
      };

      addMessage(optimisticUserMessage);
      startStreaming(conversationId);

      try {
        // Update status to sending
        await db.pendingMessages.update(pendingId, { status: 'sending' });

        // Stream the response
        await streamMessage(conversationId, content, 'claude-3-5-sonnet-20241022', {
          onToken: (token) => {
            appendStreamingContent(token);
          },
          onComplete: (message) => {
            addMessage(message);
            stopStreaming();
          },
          onError: (error) => {
            console.error('Stream error:', error);
            stopStreaming();
            throw error;
          },
          onConnected: () => {
            console.log('Stream connected');
          },
          onDisconnected: () => {
            console.log('Stream disconnected');
          }
        });

        // Remove from pending on success
        await db.pendingMessages.delete(pendingId);

        return { pendingId };
      } catch (err) {
        // Mark for retry
        const pending = await db.pendingMessages.get(pendingId);
        if (pending) {
          await db.pendingMessages.update(pendingId, {
            status: 'failed',
            retryCount: pending.retryCount + 1
          });
        }
        throw err;
      }
    },
    onSuccess: () => {
      // Invalidate messages query to refetch
      queryClient.invalidateQueries({ queryKey: ['messages', conversationId] });
    },
    onError: (err) => {
      console.error('Failed to send message:', err);
      stopStreaming();
    }
  });
}

// Background retry for failed messages
export async function retryFailedMessages(): Promise<void> {
  const failedMessages = await db.pendingMessages
    .where('status')
    .equals('failed')
    .and((msg: PendingMessage) => msg.retryCount < 3)
    .toArray();

  for (const msg of failedMessages) {
    if (!msg.id) continue;

    try {
      await db.pendingMessages.update(msg.id, { status: 'sending' });

      await streamMessage(msg.conversationId, msg.content, 'claude-3-5-sonnet-20241022', {
        onToken: () => {},
        onComplete: async () => {
          await db.pendingMessages.delete(msg.id!);
        },
        onError: async () => {
          await db.pendingMessages.update(msg.id!, {
            status: 'failed',
            retryCount: msg.retryCount + 1
          });
        }
      });
    } catch {
      await db.pendingMessages.update(msg.id, {
        status: 'failed',
        retryCount: msg.retryCount + 1
      });
    }
  }
}
