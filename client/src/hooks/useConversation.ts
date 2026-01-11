// hooks/useConversation.ts
import { useQuery } from '@tanstack/react-query';
import { useLiveQuery } from 'dexie-react-hooks';
import { db, getConversationMessages, getLastSequence, updateSyncState } from '../db';
import { listMessages } from '../api/messages';
import { useAppStore } from '../store/useAppStore';

export function useConversation(conversationId: string | null) {
  const { setSyncing } = useAppStore();

  // First, try to load from IndexedDB (instant)
  const localMessages = useLiveQuery(
    () => conversationId ? getConversationMessages(conversationId) : [],
    [conversationId],
    []
  );

  // Then, fetch from server to ensure sync
  const serverQuery = useQuery({
    queryKey: ['messages', conversationId],
    queryFn: async () => {
      if (!conversationId) return [];

      setSyncing(true);
      try {
        const afterSequence = await getLastSequence(conversationId);
        const response = await listMessages(conversationId, afterSequence);

        // Merge into IndexedDB
        if (response.messages.length > 0) {
          await db.messages.bulkPut(response.messages);
          await updateSyncState(
            conversationId,
            response.last_sequence,
            'synced'
          );
        }

        return response.messages;
      } finally {
        setSyncing(false);
      }
    },
    enabled: !!conversationId,
    staleTime: 5000, // Consider fresh for 5 seconds
    refetchOnWindowFocus: false
  });

  return {
    // Return local data immediately, server data will merge in
    messages: localMessages ?? [],
    isLoading: localMessages === undefined && serverQuery.isLoading,
    isSyncing: serverQuery.isFetching,
    error: serverQuery.error,
    refetch: serverQuery.refetch
  };
}

export function useConversations() {
  return useQuery({
    queryKey: ['conversations'],
    queryFn: async () => {
      const { listConversations } = await import('../api/conversations');
      const response = await listConversations();

      // Cache in IndexedDB
      await db.conversations.bulkPut(response.conversations);

      return response;
    },
    staleTime: 10000
  });
}
