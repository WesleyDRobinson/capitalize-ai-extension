# Product Requirements Document

## Conversational AI Messaging Platform

### NATS JetStream + Go + React Architecture

**Version:** 1.0  
**Date:** January 2026  
**Classification:** CONFIDENTIAL

-----

## Table of Contents

1. [Executive Summary](#1-executive-summary)
1. [Problem Statement](#2-problem-statement)
1. [Goals and Non-Goals](#3-goals-and-non-goals)
1. [System Architecture](#4-system-architecture)
1. [Data Models](#5-data-models)
1. [API Specification](#6-api-specification)
1. [Client Implementation](#7-client-implementation)
1. [Infrastructure and Deployment](#8-infrastructure-and-deployment)
1. [Security Considerations](#9-security-considerations)
1. [Observability](#10-observability)
1. [Milestones and Phases](#11-milestones-and-phases)
1. [Team Structure](#12-team-structure)
1. [Risk Assessment](#13-risk-assessment)
1. [Success Metrics](#14-success-metrics)
1. [Appendix: Technology Decisions](#appendix-a-technology-decisions)

-----

## 1. Executive Summary

This document defines the requirements for a real-time conversational AI platform that persists LLM-generated messages using NATS JetStream as the primary storage and streaming backbone. The system enables low-latency token streaming to clients, durable message persistence, conversation replay, and future batch processing capabilities.

### 1.1 Product Vision

Build a production-grade messaging infrastructure that treats conversation data as a first-class citizen. By leveraging NATS JetStream, we achieve both real-time streaming performance and durable persistence in a single, operationally simple system. This foundation enables AI-powered applications with full conversation history, audit trails, and analytics capabilities.

### 1.2 Key Outcomes

- Sub-100ms token delivery latency from LLM to client UI
- Complete conversation replay on client reconnection
- Durable message storage with 1-year retention
- Offline-capable client with IndexedDB synchronization
- Foundation for batch analytics, billing, and RAG indexing

### 1.3 Technical Stack

|Layer              |Technology              |Deployment           |
|-------------------|------------------------|---------------------|
|Backend API        |Go 1.22+                |Northflank           |
|Message Broker     |NATS JetStream 2.10+    |Vultr VPS (dedicated)|
|Frontend           |React 18 + TypeScript   |Vercel               |
|Client Storage     |Dexie.js (IndexedDB)    |Browser              |
|Real-time Transport|Server-Sent Events (SSE)|HTTP/2               |
|LLM Integration    |Anthropic/OpenAI APIs   |External             |

-----

## 2. Problem Statement

### 2.1 Current Challenges

Building conversational AI applications requires solving multiple interconnected problems that traditional architectures handle poorly:

#### 2.1.1 Latency vs. Durability Tradeoff

LLMs generate tokens at 30-100 tokens/second. Users expect immediate feedback, but persisting each token individually creates unacceptable write amplification. Traditional databases force a choice between real-time UX and durable storage.

#### 2.1.2 Connection Fragility

Mobile and web clients frequently disconnect. When a user loses connectivity mid-generation, they lose context. Reconnection typically means restarting the entire request, wasting compute and degrading UX.

#### 2.1.3 Conversation State Management

Conversations span sessions, devices, and time. Client-side state quickly diverges from server state. Without a unified source of truth, features like search, analytics, and context windowing become fragile.

#### 2.1.4 Operational Complexity

Running separate systems for real-time messaging (Redis Pub/Sub, WebSockets), persistent storage (Postgres), and event streaming (Kafka) creates operational burden disproportionate to Series A scale.

### 2.2 Solution Approach

NATS JetStream provides persistence-enabled messaging that unifies these concerns. Messages are durable by default, replay is a first-class primitive, and the operational footprint is minimal. Combined with client-side IndexedDB caching via Dexie.js, we achieve resilient, offline-capable conversations without infrastructure sprawl.

-----

## 3. Goals and Non-Goals

### 3.1 Goals

|Goal                       |Success Criteria                                |Priority|
|---------------------------|------------------------------------------------|--------|
|Real-time token streaming  |P95 latency < 100ms from LLM to UI render       |P0      |
|Durable message persistence|Zero message loss under normal operations       |P0      |
|Conversation replay        |Full history available within 500ms on reconnect|P0      |
|Offline resilience         |Client functional with cached data when offline |P1      |
|Multi-tenant isolation     |Complete data separation by tenant              |P1      |
|Audit trail                |Immutable record of all messages with metadata  |P1      |
|Batch processing foundation|Consumer patterns for analytics pipelines       |P2      |
|Horizontal scalability     |Linear scaling to 10K concurrent conversations  |P2      |

### 3.2 Non-Goals (Year 1)

- Multi-region deployment and global distribution
- End-to-end encryption at message level
- Real-time collaborative editing within messages
- Token-level persistence (individual tokens stored separately)
- Voice/audio message support

-----

## 4. System Architecture

### 4.1 High-Level Overview

The system consists of four primary components: the React client application, the Go API server, NATS JetStream for messaging and persistence, and external LLM providers.

```
┌─────────────────┐     SSE (tokens)      ┌─────────────────┐
│                 │◄─────────────────────►│                 │
│   React Client  │                       │    Go API       │
│   (Vercel)      │     REST (messages)   │   (Northflank)  │
│                 │◄─────────────────────►│                 │
└────────┬────────┘                       └────────┬────────┘
         │                                         │
         │ IndexedDB                               │ NATS TCP (TLS)
         ▼                                         ▼
┌─────────────────┐                       ┌─────────────────┐
│   Dexie.js      │                       │  NATS JetStream │
│   (Browser)     │                       │  (Vultr VPS)    │
└─────────────────┘                       └────────┬────────┘
                                                   │
                                                   │ HTTP API
                                                   ▼
                                          ┌─────────────────┐
                                          │  LLM Providers  │
                                          │ (Anthropic/OAI) │
                                          └─────────────────┘
```

### 4.2 Data Flow Patterns

#### 4.2.1 Message Send Flow

When a user sends a message:

1. Client posts message to Go API
1. API publishes user message to JetStream
1. API initiates streaming request to LLM provider
1. Tokens stream to client via SSE (not persisted individually)
1. On stream completion, full assistant message published to JetStream
1. Client stores message in IndexedDB via Dexie.js

#### 4.2.2 Reconnection Flow

When a client reconnects after disconnection:

1. Client checks IndexedDB for last known message sequence
1. Client requests replay from API with sequence cursor
1. API creates ephemeral JetStream consumer from cursor
1. Missed messages returned to client
1. Client merges with IndexedDB, continues with live SSE

### 4.3 NATS JetStream Configuration

#### 4.3.1 Stream Design

A single stream handles all conversation data, with subject-based filtering for tenant and conversation isolation:

```
Stream: CONVERSATIONS
Subjects: conv.{tenant_id}.{conversation_id}.msg.>
          conv.{tenant_id}.{conversation_id}.event.>

Subject Hierarchy:
  conv.acme.c_abc123.msg.user       - User messages
  conv.acme.c_abc123.msg.assistant  - Assistant messages  
  conv.acme.c_abc123.msg.system     - System messages
  conv.acme.c_abc123.msg.tool       - Tool call results
  conv.acme.c_abc123.event.error    - Error events
  conv.acme.c_abc123.event.cancel   - Cancellation events
```

#### 4.3.2 Stream Configuration

```go
StreamConfig{
    Name:        "CONVERSATIONS",
    Retention:   jetstream.LimitsPolicy,
    MaxAge:      365 * 24 * time.Hour,  // 1 year
    MaxBytes:    100 * 1024 * 1024 * 1024, // 100GB
    Storage:     jetstream.FileStorage,
    Replicas:    1,  // Increase to 3 for HA
    Compression: jetstream.S2Compression,
    DenyDelete:  true,   // Audit compliance
    DenyPurge:   true,   // Audit compliance
}
```

#### 4.3.3 Consumer Patterns

|Consumer Type|Use Case           |Configuration                                    |
|-------------|-------------------|-------------------------------------------------|
|Ephemeral    |Conversation replay|FilterSubject, DeliverAll, AckNone               |
|Durable      |Batch analytics    |Durable name, AckExplicit, MaxAckPending         |
|Durable      |Billing processor  |Durable name, FilterSubject on assistant messages|
|Durable      |RAG indexer        |Durable name, DeliverNew for incremental indexing|

-----

## 5. Data Models

### 5.1 Core Message Schema

```go
type ConversationMessage struct {
    // Identity
    ID             string    `json:"id"`               // UUIDv7
    ConversationID string    `json:"conversation_id"`  // UUIDv7
    TenantID       string    `json:"tenant_id"`
    
    // Content
    Role           string    `json:"role"`             // user|assistant|system|tool
    Content        string    `json:"content"`
    
    // LLM Metadata (nullable for non-assistant messages)
    Model          *string   `json:"model,omitempty"`
    TokensIn       *int      `json:"tokens_in,omitempty"`
    TokensOut      *int      `json:"tokens_out,omitempty"`
    LatencyMs      *int64    `json:"latency_ms,omitempty"`
    StopReason     *string   `json:"stop_reason,omitempty"`
    
    // Timestamps
    CreatedAt      time.Time `json:"created_at"`
    StreamStarted  *time.Time`json:"stream_started,omitempty"`
    StreamEnded    *time.Time`json:"stream_ended,omitempty"`
    
    // JetStream Metadata (populated on read)
    Sequence       uint64    `json:"sequence,omitempty"`
}
```

### 5.2 Conversation Metadata

```go
type Conversation struct {
    ID          string            `json:"id"`
    TenantID    string            `json:"tenant_id"`
    Title       string            `json:"title"`
    CreatedAt   time.Time         `json:"created_at"`
    UpdatedAt   time.Time         `json:"updated_at"`
    Metadata    map[string]string `json:"metadata,omitempty"`
    
    // Computed (not stored in NATS)
    MessageCount int              `json:"message_count,omitempty"`
    LastMessage  *ConversationMessage `json:"last_message,omitempty"`
}
```

### 5.3 Event Schema

```go
type ConversationEvent struct {
    ID             string         `json:"id"`
    ConversationID string         `json:"conversation_id"`
    Type           string         `json:"type"`    // error|cancel|rate_limit|timeout
    Reason         string         `json:"reason"`
    Metadata       map[string]any `json:"metadata,omitempty"`
    CreatedAt      time.Time      `json:"created_at"`
}
```

### 5.4 Client-Side Schema (Dexie.js)

```typescript
// IndexedDB schema via Dexie.js
import Dexie, { Table } from 'dexie';

interface Conversation {
  id: string;
  tenantId: string;
  title: string;
  createdAt: Date;
  updatedAt: Date;
}

interface Message {
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

interface PendingMessage {
  id?: number;
  conversationId: string;
  content: string;
  status: 'pending' | 'sending' | 'failed';
  createdAt: Date;
  retryCount: number;
}

interface SyncState {
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
```

-----

## 6. API Specification

### 6.1 REST Endpoints

#### 6.1.1 Conversations

|Method|Path                              |Description                          |
|------|----------------------------------|-------------------------------------|
|POST  |/api/v1/conversations             |Create new conversation              |
|GET   |/api/v1/conversations             |List conversations (paginated)       |
|GET   |/api/v1/conversations/:id         |Get conversation with recent messages|
|DELETE|/api/v1/conversations/:id         |Soft delete conversation             |
|GET   |/api/v1/conversations/:id/messages|Get messages (with replay support)   |
|POST  |/api/v1/conversations/:id/messages|Send message, initiate LLM stream    |

#### 6.1.2 Message Replay Endpoint

```http
GET /api/v1/conversations/:id/messages?after_sequence=123&limit=100

Response:
{
  "messages": [
    {
      "id": "msg_abc123",
      "conversation_id": "conv_xyz",
      "role": "assistant",
      "content": "Hello! How can I help you today?",
      "sequence": 124,
      "created_at": "2026-01-10T12:00:00Z",
      "model": "claude-3-sonnet",
      "tokens_out": 12
    }
  ],
  "has_more": true,
  "last_sequence": 223,
  "stream_active": false
}
```

### 6.2 SSE Streaming Endpoint

```http
GET /api/v1/conversations/:id/stream
Accept: text/event-stream

Event Types:

event: token
data: {"token": "Hello", "index": 0}

event: token
data: {"token": " there", "index": 1}

event: message_complete
data: {"message": {...}, "sequence": 456}

event: error
data: {"code": "rate_limit", "message": "Rate limit exceeded", "retry_after": 30}

event: heartbeat
data: {"timestamp": "2026-01-10T12:00:00Z"}
```

### 6.3 Send Message Request

```http
POST /api/v1/conversations/:id/messages
Content-Type: application/json

{
  "content": "What is the capital of France?",
  "model": "claude-3-sonnet",
  "stream": true
}

Response (if stream: false):
{
  "message": {...},
  "sequence": 457
}

Response (if stream: true):
HTTP 202 Accepted
X-Stream-URL: /api/v1/conversations/:id/stream
```

### 6.4 Authentication

All endpoints require Bearer token authentication. Tokens are JWTs containing tenant_id and user_id claims. The API validates tokens and enforces tenant isolation on all NATS subject operations.

```http
Authorization: Bearer <jwt_token>

JWT Claims:
{
  "sub": "user_abc123",
  "tenant_id": "tenant_xyz",
  "exp": 1736500000,
  "iat": 1736400000,
  "scope": ["conversations:read", "conversations:write"]
}
```

-----

## 7. Client Implementation

### 7.1 Technology Stack

- **React 18** with TypeScript for UI components
- **@microsoft/fetch-event-source** for SSE with POST support and reconnection
- **Dexie.js** for IndexedDB with TypeScript support
- **TanStack Query** for server state management and caching
- **Zustand** for client state management

### 7.2 SSE Connection Management

```typescript
import { fetchEventSource } from '@microsoft/fetch-event-source';
import { db } from './db';

interface TokenEvent {
  token: string;
  index: number;
}

interface MessageCompleteEvent {
  message: Message;
  sequence: number;
}

class ConversationStream {
  private controller: AbortController | null = null;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 5;
  
  async connect(
    conversationId: string,
    onToken: (token: string, index: number) => void,
    onComplete: (message: Message) => void,
    onError: (error: Error) => void
  ) {
    this.controller = new AbortController();
    this.reconnectAttempts = 0;
    
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
                  db.syncState.put({
                    conversationId,
                    lastSequence: data.sequence,
                    lastSyncAt: new Date(),
                    status: 'synced'
                  });
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
              throw err; // Required to trigger reconnection
            },
            
            onclose: () => {
              console.log('SSE connection closed');
              // Sync any missed messages
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
  
  private async handleReconnect(conversationId: string) {
    // First sync missed messages, then reconnect
    await this.syncMissedMessages(conversationId);
  }
  
  private async syncMissedMessages(conversationId: string) {
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
      await db.messages.bulkPut(messages);
      
      // Update sync state
      await db.syncState.put({
        conversationId,
        lastSequence: last_sequence,
        lastSyncAt: new Date(),
        status: has_more ? 'stale' : 'synced'
      });
      
      // If more messages exist, continue syncing
      if (has_more) {
        await this.syncMissedMessages(conversationId);
      }
    } catch (err) {
      console.error('Failed to sync messages:', err);
      await db.syncState.update(conversationId, { status: 'stale' });
    }
  }
  
  disconnect() {
    this.controller?.abort();
    this.controller = null;
  }
}

export const conversationStream = new ConversationStream();
```

### 7.3 Offline Support with Pending Message Queue

```typescript
// hooks/useSendMessage.ts
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { db } from './db';

export function useSendMessage(conversationId: string) {
  const queryClient = useQueryClient();
  
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
      
      // Optimistically add to UI
      const optimisticMessage = {
        id: `pending_${pendingId}`,
        conversationId,
        role: 'user' as const,
        content,
        sequence: -1,
        createdAt: new Date()
      };
      
      queryClient.setQueryData(
        ['messages', conversationId],
        (old: Message[] = []) => [...old, optimisticMessage]
      );
      
      try {
        // Update status to sending
        await db.pendingMessages.update(pendingId, { status: 'sending' });
        
        const response = await fetch(
          `/api/v1/conversations/${conversationId}/messages`,
          {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
              'Authorization': `Bearer ${getAccessToken()}`
            },
            body: JSON.stringify({ content, stream: true })
          }
        );
        
        if (!response.ok) {
          throw new Error(`Send failed: ${response.status}`);
        }
        
        // Remove from pending on success
        await db.pendingMessages.delete(pendingId);
        
        // The actual message will arrive via SSE
        return { pendingId };
      } catch (err) {
        // Mark for retry
        await db.pendingMessages.update(pendingId, {
          status: 'failed',
          retryCount: (await db.pendingMessages.get(pendingId))!.retryCount + 1
        });
        throw err;
      }
    },
    onError: (err) => {
      console.error('Failed to send message:', err);
    }
  });
}

// Background retry for failed messages
export async function retryFailedMessages() {
  const failedMessages = await db.pendingMessages
    .where('status')
    .equals('failed')
    .and(msg => msg.retryCount < 3)
    .toArray();
  
  for (const msg of failedMessages) {
    try {
      await db.pendingMessages.update(msg.id!, { status: 'sending' });
      
      const response = await fetch(
        `/api/v1/conversations/${msg.conversationId}/messages`,
        {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${getAccessToken()}`
          },
          body: JSON.stringify({ content: msg.content, stream: true })
        }
      );
      
      if (response.ok) {
        await db.pendingMessages.delete(msg.id!);
      } else {
        throw new Error(`Retry failed: ${response.status}`);
      }
    } catch (err) {
      await db.pendingMessages.update(msg.id!, {
        status: 'failed',
        retryCount: msg.retryCount + 1
      });
    }
  }
}
```

### 7.4 React Hook for Conversation Loading

```typescript
// hooks/useConversation.ts
import { useQuery } from '@tanstack/react-query';
import { useLiveQuery } from 'dexie-react-hooks';
import { db } from './db';

export function useConversation(conversationId: string) {
  // First, try to load from IndexedDB (instant)
  const localMessages = useLiveQuery(
    () => db.messages
      .where('conversationId')
      .equals(conversationId)
      .sortBy('sequence'),
    [conversationId]
  );
  
  // Then, fetch from server to ensure sync
  const serverQuery = useQuery({
    queryKey: ['messages', conversationId],
    queryFn: async () => {
      const syncState = await db.syncState.get(conversationId);
      const afterSequence = syncState?.lastSequence ?? 0;
      
      const response = await fetch(
        `/api/v1/conversations/${conversationId}/messages?after_sequence=${afterSequence}`,
        {
          headers: {
            'Authorization': `Bearer ${getAccessToken()}`
          }
        }
      );
      
      if (!response.ok) {
        throw new Error(`Fetch failed: ${response.status}`);
      }
      
      const { messages, last_sequence } = await response.json();
      
      // Merge into IndexedDB
      if (messages.length > 0) {
        await db.messages.bulkPut(messages);
        await db.syncState.put({
          conversationId,
          lastSequence: last_sequence,
          lastSyncAt: new Date(),
          status: 'synced'
        });
      }
      
      return messages;
    },
    staleTime: 5000, // Consider fresh for 5 seconds
  });
  
  return {
    // Return local data immediately, server data will merge in
    messages: localMessages ?? [],
    isLoading: localMessages === undefined && serverQuery.isLoading,
    isSyncing: serverQuery.isFetching,
    error: serverQuery.error
  };
}
```

-----

## 8. Infrastructure and Deployment

### 8.1 Deployment Architecture Overview

The infrastructure follows a hybrid model optimized for Series A constraints:

- **Stateless services (Go API)** → Northflank (auto-scaling, zero-downtime deploys)
- **Stateful services (NATS JetStream)** → Vultr VPS (persistent disk, predictable performance)
- **Static assets (React client)** → Vercel (edge CDN, instant deploys)

```
┌─────────────────────────────────────────────────────────────────────┐
│                           Vercel Edge                                │
│                    (React Client - Global CDN)                       │
└─────────────────────────┬───────────────────────────────────────────┘
                          │ HTTPS
                          ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         Northflank                                   │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                  │
│  │   API Pod   │  │   API Pod   │  │   API Pod   │  (auto-scale)    │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘                  │
│         └────────────────┼────────────────┘                          │
└──────────────────────────┼──────────────────────────────────────────┘
                           │ NATS TCP (TLS)
                           ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      Vultr VPS (Dedicated)                           │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │                    NATS JetStream                            │    │
│  │                                                              │    │
│  │   ┌──────────────────────────────────────────────────────┐  │    │
│  │   │              /var/lib/nats/jetstream                  │  │    │
│  │   │                 (NVMe Block Storage)                  │  │    │
│  │   └──────────────────────────────────────────────────────┘  │    │
│  └─────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────┘
```

**Why this split:**

|Concern|Northflank           |Vultr VPS                                             |
|-------|---------------------|------------------------------------------------------|
|Scaling|Auto-scale on demand |Manual (but NATS handles 100K+ msg/sec on single node)|
|State  |Ephemeral containers |Persistent disk survives reboots                      |
|Deploys|Zero-downtime rolling|Requires careful orchestration                        |
|Cost   |Pay per use          |Fixed monthly ($24-96/mo for right-sized instance)    |
|Ops    |Managed              |SSH access, you own it                                |

For a stateful message broker with durability requirements, a dedicated VPS eliminates the complexity of persistent volumes in container orchestrators and the risk of data loss during pod rescheduling.

-----

### 8.2 Vultr VPS Setup (NATS JetStream)

#### 8.2.1 Recommended Instance Sizing

|Stage                   |Vultr Plan           |vCPU|RAM |Storage   |Monthly Cost|
|------------------------|---------------------|----|----|----------|------------|
|**Personal / Prototype**|Regular Cloud Compute|1   |1GB |25GB SSD  |**$5**      |
|Development             |Regular Cloud Compute|1   |2GB |55GB SSD  |$10         |
|Staging / Early Prod    |High Frequency       |2   |4GB |128GB NVMe|$24         |
|Production              |High Frequency       |4   |8GB |256GB NVMe|$48         |
|Scale (if needed)       |High Performance     |8   |32GB|512GB NVMe|$192        |

**Starting minimal:** NATS is written in Go and is remarkably efficient. A $5 instance with 1 vCPU / 1GB RAM handles thousands of messages per second — more than sufficient for personal use, prototyping, or early customer validation. Constrain `max_memory_store` to 256MB and let JetStream’s file storage do the work.

**Storage math:** A typical conversation message is ~2-4KB with metadata. At 25GB storage, you’re looking at 6-12 million messages before disk pressure — years of personal archive or months of moderate multi-user traffic.

**Upgrade path:** The architecture is identical at every tier. When you need more headroom, resize the instance or migrate to High Frequency for NVMe performance. No code changes required.

**Note:** High Frequency instances use NVMe which matters for write-heavy workloads at scale. For personal/prototype use, regular SSD is fine.

#### 8.2.2 Initial Server Setup

```bash
#!/bin/bash
# setup-nats-server.sh
# Run on fresh Ubuntu 24.04 LTS Vultr instance

set -euo pipefail

# System updates
apt update && apt upgrade -y
apt install -y ufw fail2ban curl jq

# Create nats user
useradd -r -s /bin/false nats

# Download NATS Server
NATS_VERSION="2.10.24"
curl -L "https://github.com/nats-io/nats-server/releases/download/v${NATS_VERSION}/nats-server-v${NATS_VERSION}-linux-amd64.tar.gz" | tar xz
mv nats-server-v${NATS_VERSION}-linux-amd64/nats-server /usr/local/bin/
chmod +x /usr/local/bin/nats-server

# Create directories
mkdir -p /etc/nats
mkdir -p /var/lib/nats/jetstream
mkdir -p /var/log/nats
chown -R nats:nats /var/lib/nats /var/log/nats

# Install NATS CLI for administration
curl -L "https://github.com/nats-io/natscli/releases/download/v0.1.5/nats-0.1.5-linux-amd64.tar.gz" | tar xz
mv nats-0.1.5-linux-amd64/nats /usr/local/bin/

echo "NATS installed. Configure /etc/nats/nats.conf next."
```

#### 8.2.3 NATS Configuration

```conf
# /etc/nats/nats.conf

server_name: nats-prod-1

# Network
host: 0.0.0.0
port: 4222
http_port: 8222  # Monitoring (bind to localhost in production)

# TLS for client connections (required for production)
tls {
  cert_file: "/etc/nats/server-cert.pem"
  key_file: "/etc/nats/server-key.pem"
  ca_file: "/etc/nats/ca.pem"
  verify: true
  timeout: 2
}

# JetStream configuration
jetstream {
  store_dir: "/var/lib/nats/jetstream"
  
  # Memory for caching (not persistence)
  # Adjust based on instance size:
  #   $5 (1GB RAM):  256MB
  #   $10 (2GB RAM): 512MB  
  #   $24+ (4GB+ RAM): 1GB+
  max_memory_store: 256MB
  
  # Disk storage limit (leave headroom for OS)
  #   25GB disk: 20GB
  #   55GB disk: 45GB
  #   128GB+ disk: 100GB
  max_file_store: 20GB
  
  # Sync writes to disk (stronger durability, slight perf impact)
  # Default is 2 minutes; 1 minute is good balance
  sync_interval: "1m"
}

# Connection limits (scale with RAM)
max_connections: 1000
max_payload: 8MB
max_pending: 16MB

# Timeouts
ping_interval: "2m"
ping_max: 2
write_deadline: "10s"

# Logging
debug: false
trace: false
logtime: true
log_file: "/var/log/nats/nats.log"

# Authorization (simple token for API servers)
authorization {
  token: "${NATS_AUTH_TOKEN}"
}
```

**Minimal instance adjustments ($5 tier):**

- `max_memory_store: 256MB` — leaves ~700MB for OS and NATS process
- `max_file_store: 20GB` — leaves 5GB headroom on 25GB disk
- `max_connections: 1000` — still plenty for personal use
- `max_pending: 16MB` — reduced buffer size

These settings are conservative. Monitor with `nats server report jetstream` and adjust upward as needed.

#### 8.2.4 Systemd Service

```ini
# /etc/systemd/system/nats.service

[Unit]
Description=NATS JetStream Server
After=network.target
Documentation=https://docs.nats.io

[Service]
Type=simple
User=nats
Group=nats
ExecStart=/usr/local/bin/nats-server -c /etc/nats/nats.conf
ExecReload=/bin/kill -HUP $MAINPID
Restart=always
RestartSec=5
LimitNOFILE=65536

# Environment file for secrets
EnvironmentFile=/etc/nats/nats.env

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/nats /var/log/nats

[Install]
WantedBy=multi-user.target
```

```bash
# /etc/nats/nats.env
NATS_AUTH_TOKEN=your-secure-token-here
```

```bash
# Enable and start
systemctl daemon-reload
systemctl enable nats
systemctl start nats

# Verify
nats server info --server nats://localhost:4222
```

#### 8.2.5 Firewall Configuration

```bash
# UFW rules for NATS server
ufw default deny incoming
ufw default allow outgoing

# SSH (restrict to your IP in production)
ufw allow 22/tcp

# NATS client connections (restrict to Northflank IPs)
# Get Northflank egress IPs from their dashboard
ufw allow from 1.2.3.4/32 to any port 4222 proto tcp  # Northflank IP 1
ufw allow from 5.6.7.8/32 to any port 4222 proto tcp  # Northflank IP 2

# Monitoring (localhost only, accessed via SSH tunnel)
# ufw allow from 127.0.0.1 to any port 8222

ufw enable
```

#### 8.2.6 TLS Certificate Setup

```bash
# Generate self-signed certs for development
# For production, use Let's Encrypt or your CA

mkdir -p /etc/nats/certs
cd /etc/nats/certs

# CA
openssl genrsa -out ca-key.pem 4096
openssl req -x509 -new -nodes -key ca-key.pem -sha256 -days 3650 \
  -out ca.pem -subj "/CN=NATS CA"

# Server cert
openssl genrsa -out server-key.pem 4096
openssl req -new -key server-key.pem -out server.csr \
  -subj "/CN=nats.yourapp.com"

cat > server-ext.cnf << EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = nats.yourapp.com
DNS.2 = localhost
IP.1 = YOUR_VULTR_IP
EOF

openssl x509 -req -in server.csr -CA ca.pem -CAkey ca-key.pem \
  -CAcreateserial -out server-cert.pem -days 825 -sha256 \
  -extfile server-ext.cnf

# Client cert (for API servers)
openssl genrsa -out client-key.pem 4096
openssl req -new -key client-key.pem -out client.csr \
  -subj "/CN=api-client"
openssl x509 -req -in client.csr -CA ca.pem -CAkey ca-key.pem \
  -CAcreateserial -out client-cert.pem -days 825 -sha256

# Set permissions
chown -R nats:nats /etc/nats/certs
chmod 600 /etc/nats/certs/*-key.pem
```

#### 8.2.7 Stream Initialization Script

```bash
#!/bin/bash
# init-streams.sh
# Run once to create JetStream streams

NATS_URL="nats://localhost:4222"
NATS_CREDS="--user admin --password ${NATS_ADMIN_PASSWORD}"

# Create the conversations stream
nats stream add CONVERSATIONS \
  --server "$NATS_URL" \
  $NATS_CREDS \
  --subjects "conv.>" \
  --storage file \
  --replicas 1 \
  --retention limits \
  --max-age 365d \
  --max-bytes 100GB \
  --max-msg-size 8MB \
  --discard old \
  --dupe-window 2m \
  --deny-delete \
  --deny-purge \
  --compression s2 \
  --description "All conversation messages and events"

# Verify
nats stream info CONVERSATIONS --server "$NATS_URL" $NATS_CREDS
```

-----

### 8.3 Northflank Configuration (Go API)

#### 8.3.1 Go API Service

```yaml
# northflank.yaml
apiVersion: v1
kind: CombinedService
metadata:
  name: api
spec:
  deployment:
    instances: 2
    internal:
      cpu: 1000m
      memory: 2Gi
    strategy:
      type: RollingUpdate
      rollingUpdate:
        maxUnavailable: 1
  ports:
    - name: http
      internalPort: 8080
      protocol: HTTP
      public: true
      domains:
        - api.yourapp.com
  healthChecks:
    liveness:
      type: http
      path: /health
      port: 8080
      initialDelaySeconds: 10
      periodSeconds: 10
    readiness:
      type: http
      path: /ready
      port: 8080
      initialDelaySeconds: 5
      periodSeconds: 5
  runtimeEnvironment:
    # NATS connection to Vultr VPS
    - name: NATS_URL
      value: tls://nats.yourapp.com:4222
    - name: NATS_CA_FILE
      fromSecret: nats-certs
      key: ca.pem
    - name: NATS_CERT_FILE
      fromSecret: nats-certs
      key: client-cert.pem
    - name: NATS_KEY_FILE
      fromSecret: nats-certs
      key: client-key.pem
    - name: JWT_SECRET
      fromSecret: jwt-secret
      key: secret
    - name: ANTHROPIC_API_KEY
      fromSecret: llm-keys
      key: anthropic
    - name: LOG_LEVEL
      value: info
  build:
    type: dockerfile
    dockerfile: Dockerfile
    context: .
```

#### 8.3.2 Go NATS Client Configuration

```go
// internal/nats/client.go
package nats

import (
    "crypto/tls"
    "crypto/x509"
    "os"
    "time"

    "github.com/nats-io/nats.go"
    "github.com/nats-io/nats.go/jetstream"
)

type Config struct {
    URL      string
    CAFile   string
    CertFile string
    KeyFile  string
}

func Connect(cfg Config) (*nats.Conn, jetstream.JetStream, error) {
    // Load CA cert
    caCert, err := os.ReadFile(cfg.CAFile)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to read CA file: %w", err)
    }
    caCertPool := x509.NewCertPool()
    caCertPool.AppendCertsFromPEM(caCert)

    // Load client cert
    cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to load client cert: %w", err)
    }

    tlsConfig := &tls.Config{
        RootCAs:      caCertPool,
        Certificates: []tls.Certificate{cert},
        MinVersion:   tls.VersionTLS12,
    }

    // Connect with reconnection handling
    nc, err := nats.Connect(
        cfg.URL,
        nats.Secure(tlsConfig),
        nats.MaxReconnects(-1),  // Infinite reconnects
        nats.ReconnectWait(2*time.Second),
        nats.ReconnectBufSize(8*1024*1024),  // 8MB buffer during reconnect
        nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
            log.Warn("NATS disconnected", "error", err)
        }),
        nats.ReconnectHandler(func(nc *nats.Conn) {
            log.Info("NATS reconnected", "url", nc.ConnectedUrl())
        }),
        nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
            log.Error("NATS error", "error", err)
        }),
    )
    if err != nil {
        return nil, nil, fmt.Errorf("failed to connect to NATS: %w", err)
    }

    // Create JetStream context
    js, err := jetstream.New(nc)
    if err != nil {
        nc.Close()
        return nil, nil, fmt.Errorf("failed to create JetStream context: %w", err)
    }

    return nc, js, nil
}
```

-----

### 8.4 Vercel Configuration (React Client)

```json
// vercel.json
{
  "framework": "vite",
  "buildCommand": "npm run build",
  "outputDirectory": "dist",
  "rewrites": [
    {
      "source": "/api/:path*",
      "destination": "https://api.yourapp.com/api/:path*"
    }
  ],
  "headers": [
    {
      "source": "/(.*)",
      "headers": [
        { "key": "X-Content-Type-Options", "value": "nosniff" },
        { "key": "X-Frame-Options", "value": "DENY" },
        { "key": "X-XSS-Protection", "value": "1; mode=block" },
        { "key": "Referrer-Policy", "value": "strict-origin-when-cross-origin" }
      ]
    },
    {
      "source": "/api/:path*",
      "headers": [
        { "key": "Cache-Control", "value": "no-store" }
      ]
    }
  ],
  "regions": ["iad1"]
}
```

-----

### 8.5 Environment Separation

|Environment             |NATS Deployment         |API Instances|Data Retention|Notes                              |
|------------------------|------------------------|-------------|--------------|-----------------------------------|
|**Personal / Prototype**|$5 Vultr (1 vCPU / 1GB) |1            |365 days      |Start here                         |
|Development             |$10 Vultr (1 vCPU / 2GB)|1            |30 days       |Team dev environment               |
|Staging                 |$24 Vultr (2 vCPU / 4GB)|2            |30 days       |Mirror production config           |
|Production              |$48 Vultr (4 vCPU / 8GB)|3+           |365 days      |Single node sufficient for Series A|
|Scale (future)          |3x Vultr cluster        |5+           |365 days      |When you need HA                   |

**Cost Comparison:**

|Scenario              |Container Platform (PVC)|Vultr VPS               |
|----------------------|------------------------|------------------------|
|Minimal viable        |~$40-60/mo              |**$5/mo**               |
|NATS + 100GB storage  |~$80-120/mo             |$48/mo                  |
|Data persistence      |Complex (PVC management)|Native (survives reboot)|
|Network performance   |Variable                |Consistent              |
|Operational complexity|Higher                  |Lower                   |

-----

### 8.6 Backup and Disaster Recovery

#### 8.6.1 Automated Backups

```bash
#!/bin/bash
# /opt/scripts/backup-nats.sh
# Run via cron: 0 */6 * * * /opt/scripts/backup-nats.sh

set -euo pipefail

BACKUP_DIR="/var/backups/nats"
NATS_DATA="/var/lib/nats/jetstream"
RETENTION_DAYS=7
DATE=$(date +%Y%m%d_%H%M%S)

mkdir -p "$BACKUP_DIR"

# Stop writes temporarily for consistent snapshot
# (JetStream handles this gracefully)
systemctl stop nats

# Create backup
tar -czf "$BACKUP_DIR/jetstream_$DATE.tar.gz" -C /var/lib/nats jetstream

# Restart NATS
systemctl start nats

# Upload to object storage (Vultr Object Storage or S3)
# aws s3 cp "$BACKUP_DIR/jetstream_$DATE.tar.gz" s3://your-bucket/nats-backups/

# Clean old backups
find "$BACKUP_DIR" -name "jetstream_*.tar.gz" -mtime +$RETENTION_DAYS -delete

echo "Backup completed: jetstream_$DATE.tar.gz"
```

#### 8.6.2 Vultr Snapshots

```bash
# Weekly automated snapshots via Vultr API
# Add to cron or use Vultr's scheduled snapshot feature

VULTR_API_KEY="your-api-key"
INSTANCE_ID="your-instance-id"

curl -X POST "https://api.vultr.com/v2/instances/${INSTANCE_ID}/snapshots" \
  -H "Authorization: Bearer ${VULTR_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"description": "Weekly NATS snapshot"}'
```

#### 8.6.3 Recovery Procedure

```bash
# Disaster recovery steps

# 1. Provision new Vultr instance (same region/spec)
# 2. Restore from snapshot OR:

# Restore from tar backup
systemctl stop nats
rm -rf /var/lib/nats/jetstream/*
tar -xzf /var/backups/nats/jetstream_YYYYMMDD_HHMMSS.tar.gz -C /var/lib/nats
chown -R nats:nats /var/lib/nats
systemctl start nats

# 3. Verify stream integrity
nats stream info CONVERSATIONS

# 4. Update DNS / Northflank env vars if IP changed
# 5. Verify API connectivity
```

-----

### 8.7 Monitoring the Vultr Instance

#### 8.7.1 NATS Prometheus Exporter

```bash
# Install Prometheus NATS Exporter
curl -L "https://github.com/nats-io/prometheus-nats-exporter/releases/download/v0.15.0/prometheus-nats-exporter-v0.15.0-linux-amd64.tar.gz" | tar xz
mv prometheus-nats-exporter-v0.15.0-linux-amd64/prometheus-nats-exporter /usr/local/bin/
```

```ini
# /etc/systemd/system/nats-exporter.service
[Unit]
Description=Prometheus NATS Exporter
After=nats.service

[Service]
Type=simple
ExecStart=/usr/local/bin/prometheus-nats-exporter -varz -jsz=all -connz -routez http://localhost:8222
Restart=always

[Install]
WantedBy=multi-user.target
```

#### 8.7.2 Basic Health Check Script

```bash
#!/bin/bash
# /opt/scripts/health-check.sh
# Called by external monitoring (UptimeRobot, Northflank health check, etc.)

NATS_URL="http://localhost:8222"

# Check NATS is responding
if ! curl -sf "$NATS_URL/healthz" > /dev/null; then
  echo "NATS health check failed"
  exit 1
fi

# Check JetStream is enabled
JS_STATUS=$(curl -sf "$NATS_URL/jsz" | jq -r '.config.store_dir')
if [ -z "$JS_STATUS" ]; then
  echo "JetStream not available"
  exit 1
fi

# Check disk space (alert if < 20% free)
DISK_FREE=$(df /var/lib/nats | awk 'NR==2 {print 100 - $5}' | tr -d '%')
if [ "$DISK_FREE" -lt 20 ]; then
  echo "Low disk space: ${DISK_FREE}% free"
  exit 1
fi

echo "OK"
exit 0
```

-----

### 8.8 CI/CD Pipeline

```yaml
# .github/workflows/deploy.yml
name: Deploy

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    services:
      nats:
        image: nats:2.10-alpine
        ports:
          - 4222:4222
        options: --health-cmd "wget -q --spider http://localhost:8222/healthz || exit 1" --health-interval 5s
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      
      - name: Run tests
        run: go test -v -race -coverprofile=coverage.out ./...
        env:
          NATS_URL: nats://localhost:4222
      
      - name: Upload coverage
        uses: codecov/codecov-action@v4

  deploy-api:
    needs: test
    if: github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Deploy to Northflank
        uses: northflank/deploy-action@v1
        with:
          api-token: ${{ secrets.NORTHFLANK_API_TOKEN }}
          project-id: ${{ secrets.NORTHFLANK_PROJECT_ID }}
          service-id: api
          
  deploy-client:
    needs: test
    if: github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Deploy to Vercel
        uses: amondnet/vercel-action@v25
        with:
          vercel-token: ${{ secrets.VERCEL_TOKEN }}
          vercel-org-id: ${{ secrets.VERCEL_ORG_ID }}
          vercel-project-id: ${{ secrets.VERCEL_PROJECT_ID }}
          vercel-args: '--prod'

  # Optional: NATS config deployment
  deploy-nats-config:
    needs: test
    if: github.ref == 'refs/heads/main' && contains(github.event.head_commit.modified, 'nats/')
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Deploy NATS config
        uses: appleboy/ssh-action@v1.0.3
        with:
          host: ${{ secrets.VULTR_NATS_HOST }}
          username: deploy
          key: ${{ secrets.VULTR_SSH_KEY }}
          script: |
            sudo cp /home/deploy/nats.conf /etc/nats/nats.conf
            sudo systemctl reload nats
```

-----

## 9. Security Considerations

### 9.1 Authentication and Authorization

- **JWT-based authentication** with short-lived access tokens (15 min) and refresh tokens (7 days)
- **Tenant isolation** enforced at API layer; all NATS subjects include tenant_id
- **Conversation ownership** validated on every request
- **Rate limiting** per user (60 req/min) and per tenant (1000 req/min)
- **Scope-based permissions** in JWT claims for fine-grained access control

### 9.2 Data Protection

- **TLS 1.3 required** for all connections (API, NATS, LLM providers)
- **Encryption at rest** for JetStream file storage (Northflank managed volumes)
- **IndexedDB data** scoped to origin; no cross-site access possible
- **LLM API keys** stored in secrets manager, never exposed to client
- **PII handling**: No sensitive data logged; conversation content encrypted in transit

### 9.3 Input Validation

```go
// middleware/validation.go
func ValidateMessageContent(content string) error {
    if len(content) == 0 {
        return errors.New("content cannot be empty")
    }
    if len(content) > 100000 { // ~100KB limit
        return errors.New("content exceeds maximum length")
    }
    if !utf8.ValidString(content) {
        return errors.New("content must be valid UTF-8")
    }
    return nil
}

func ValidateConversationID(id string) error {
    // UUIDv7 format validation
    if _, err := uuid.Parse(id); err != nil {
        return errors.New("invalid conversation ID format")
    }
    return nil
}
```

### 9.4 Audit and Compliance

- **Immutable message log** via DenyDelete/DenyPurge on JetStream stream
- **All API requests logged** with user_id, tenant_id, action, timestamp, duration
- **JetStream advisories** captured for system-level audit trail
- **Data retention policy** enforced by MaxAge configuration
- **GDPR considerations**: Soft delete marks conversations for exclusion, hard delete requires manual intervention with audit record

### 9.5 Security Headers

```go
// middleware/security.go
func SecurityHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("X-XSS-Protection", "1; mode=block")
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
        w.Header().Set("Content-Security-Policy", "default-src 'self'")
        w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        next.ServeHTTP(w, r)
    })
}
```

-----

## 10. Observability

### 10.1 Metrics

|Metric                        |Type     |Labels                   |Alert Threshold|
|------------------------------|---------|-------------------------|---------------|
|`api_request_duration_seconds`|Histogram|method, path, status     |P99 > 500ms    |
|`llm_stream_duration_seconds` |Histogram|model, status            |P99 > 60s      |
|`llm_tokens_total`            |Counter  |model, direction (in/out)|N/A (billing)  |
|`nats_stream_messages`        |Gauge    |stream                   |N/A            |
|`nats_stream_bytes`           |Gauge    |stream                   |> 80GB         |
|`nats_consumer_pending`       |Gauge    |stream, consumer         |> 10000        |
|`sse_connections_active`      |Gauge    |-                        |> 5000         |
|`client_sync_failures_total`  |Counter  |reason                   |> 100/min      |

### 10.2 Prometheus Instrumentation

```go
// metrics/metrics.go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    RequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "api_request_duration_seconds",
            Help:    "HTTP request duration in seconds",
            Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
        },
        []string{"method", "path", "status"},
    )
    
    LLMStreamDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "llm_stream_duration_seconds",
            Help:    "LLM streaming response duration",
            Buckets: []float64{1, 2, 5, 10, 20, 30, 45, 60, 90, 120},
        },
        []string{"model", "status"},
    )
    
    LLMTokensTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "llm_tokens_total",
            Help: "Total LLM tokens processed",
        },
        []string{"model", "direction"},
    )
    
    SSEConnectionsActive = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "sse_connections_active",
            Help: "Number of active SSE connections",
        },
    )
)
```

### 10.3 Structured Logging

```go
// logger/logger.go
package logger

import (
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

func NewLogger(level string) (*zap.Logger, error) {
    config := zap.Config{
        Level:       zap.NewAtomicLevelAt(parseLevel(level)),
        Development: false,
        Encoding:    "json",
        EncoderConfig: zapcore.EncoderConfig{
            TimeKey:        "ts",
            LevelKey:       "level",
            NameKey:        "logger",
            CallerKey:      "caller",
            MessageKey:     "msg",
            StacktraceKey:  "stacktrace",
            LineEnding:     zapcore.DefaultLineEnding,
            EncodeLevel:    zapcore.LowercaseLevelEncoder,
            EncodeTime:     zapcore.ISO8601TimeEncoder,
            EncodeDuration: zapcore.SecondsDurationEncoder,
            EncodeCaller:   zapcore.ShortCallerEncoder,
        },
        OutputPaths:      []string{"stdout"},
        ErrorOutputPaths: []string{"stderr"},
    }
    return config.Build()
}

// Example log output:
// {
//   "level": "info",
//   "ts": "2026-01-10T12:00:00.000Z",
//   "caller": "handler/messages.go:45",
//   "msg": "message_sent",
//   "correlation_id": "req_abc123",
//   "tenant_id": "tenant_xyz",
//   "user_id": "user_456",
//   "conversation_id": "conv_789",
//   "duration_ms": 45,
//   "tokens_out": 128
// }
```

### 10.4 Distributed Tracing

```go
// tracing/tracing.go
package tracing

import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
    "go.opentelemetry.io/otel/sdk/resource"
    "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

func InitTracer(serviceName, endpoint string) (*trace.TracerProvider, error) {
    exporter, err := otlptracehttp.New(
        context.Background(),
        otlptracehttp.WithEndpoint(endpoint),
        otlptracehttp.WithInsecure(),
    )
    if err != nil {
        return nil, err
    }
    
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceName(serviceName),
        )),
    )
    
    otel.SetTracerProvider(tp)
    return tp, nil
}
```

### 10.5 Alerting Rules

```yaml
# alerts.yaml
groups:
  - name: api
    rules:
      - alert: HighErrorRate
        expr: sum(rate(api_request_duration_seconds_count{status=~"5.."}[5m])) / sum(rate(api_request_duration_seconds_count[5m])) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "API error rate above 5%"
          
      - alert: HighLatency
        expr: histogram_quantile(0.99, rate(api_request_duration_seconds_bucket[5m])) > 0.5
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "P99 latency above 500ms"
          
  - name: nats
    rules:
      - alert: NATSUnavailable
        expr: up{job="nats"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "NATS server is down"
          
      - alert: HighConsumerLag
        expr: nats_consumer_pending > 10000
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "JetStream consumer has high pending messages"
```

-----

## 11. Milestones and Phases

### Phase 1: Foundation (Months 1-3)

**Objective:** Establish core infrastructure and basic message flow

**Deliverables:**

- Go API scaffolding with authentication middleware
- NATS JetStream deployment on Vultr VPS with TLS and stream configuration
- Basic conversation CRUD operations
- LLM integration with streaming response handling
- React client with basic chat UI
- SSE token streaming to client

**Success Criteria:** End-to-end message flow working in staging environment

**Key Risks:**

- NATS-to-Northflank networking (TLS, firewall rules)
- SSE connection handling edge cases

-----

### Phase 2: Persistence and Replay (Months 4-5)

**Objective:** Implement durable storage and reconnection handling

**Deliverables:**

- Message persistence to JetStream on stream completion
- Conversation replay API endpoint
- Dexie.js IndexedDB integration in client
- Automatic reconnection and sync logic with @microsoft/fetch-event-source
- Sync state management and conflict resolution

**Success Criteria:** Client maintains state across disconnections with < 500ms sync time

**Key Risks:**

- IndexedDB storage quota management
- Race conditions in sync logic

-----

### Phase 3: Multi-tenancy and Security (Months 6-7)

**Objective:** Production-ready security and tenant isolation

**Deliverables:**

- JWT authentication with refresh token flow
- Tenant isolation at NATS subject level
- Rate limiting per user and tenant
- Audit logging implementation
- Security review and penetration testing

**Success Criteria:** Pass security audit with no critical findings

**Key Risks:**

- JWT token management complexity
- Performance impact of auth middleware

-----

### Phase 4: Observability and Reliability (Months 8-9)

**Objective:** Production-grade monitoring and operational tooling

**Deliverables:**

- Prometheus metrics instrumentation
- Grafana dashboards for key metrics
- OpenTelemetry tracing integration
- Alerting rules and runbooks
- Load testing and capacity planning
- Disaster recovery procedures documented and tested

**Success Criteria:** Successfully handle 10K concurrent conversations in load test

**Key Risks:**

- Metric cardinality explosion
- Tracing overhead at scale

-----

### Phase 5: Batch Processing and Analytics (Months 10-11)

**Objective:** Enable downstream processing of conversation data

**Deliverables:**

- Durable consumer patterns for batch jobs
- Token usage billing processor
- Conversation analytics pipeline
- RAG indexing integration (vector embeddings)
- Admin dashboard for usage visibility

**Success Criteria:** Billing accuracy within 0.1% of actual LLM costs

**Key Risks:**

- Consumer position management complexity
- Analytics query performance

-----

### Phase 6: Polish and Launch (Month 12)

**Objective:** Production launch and handoff

**Deliverables:**

- Performance optimization based on production data
- Documentation: API reference, architecture guide, runbooks
- On-call rotation and escalation procedures
- Feature flag infrastructure for gradual rollout
- Production launch with staged rollout

**Success Criteria:** Zero critical incidents in first 30 days post-launch

**Key Risks:**

- Unknown unknowns from real user traffic
- Team burnout before launch

-----

## 12. Team Structure

### 12.1 Core Team Composition

|Role                     |Count|Responsibilities                                        |
|-------------------------|-----|--------------------------------------------------------|
|Tech Lead / Architect    |1    |Architecture decisions, code review, technical direction|
|Backend Engineer (Go)    |2    |API development, NATS integration, LLM orchestration    |
|Frontend Engineer (React)|2    |Client application, IndexedDB, SSE handling             |
|Platform/DevOps Engineer |1    |Northflank, Vercel, CI/CD, monitoring infrastructure    |
|QA Engineer              |1    |Test strategy, automation, load testing                 |
|Product Manager          |1    |Requirements, prioritization, stakeholder management    |

**Total:** 8 FTEs

### 12.2 Recommended Hires by Phase

|Phase  |New Hires                     |Rationale                             |
|-------|------------------------------|--------------------------------------|
|Phase 1|Full team onboarded           |Foundation work is parallelizable     |
|Phase 3|Security consultant (contract)|Penetration testing, compliance review|
|Phase 5|Data Engineer (optional)      |Analytics pipeline if scope expands   |

### 12.3 Team Rituals

- **Daily standup:** 15 min async (Slack) or sync
- **Sprint planning:** Bi-weekly, 2-hour sessions
- **Architecture review:** Weekly, 1-hour deep dives
- **Demo day:** End of each sprint, stakeholder presentation
- **Retrospective:** Bi-weekly, continuous improvement focus

### 12.4 External Dependencies

- **LLM Provider (Anthropic/OpenAI):** API access, rate limit increases, support escalation
- **Northflank:** API hosting, auto-scaling, zero-downtime deploys
- **Vultr:** VPS uptime SLA, snapshot storage, network reliability
- **Vercel:** Edge function limits for SSE proxy if needed, support tier

-----

## 13. Risk Assessment

|Risk                          |Likelihood|Impact  |Mitigation                                                                          |
|------------------------------|----------|--------|------------------------------------------------------------------------------------|
|NATS JetStream data loss      |Low       |Critical|Automated backups every 6 hours; weekly Vultr snapshots; test restore monthly       |
|SSE connection limits at scale|Medium    |High    |Implement connection pooling; prepare WebSocket fallback                            |
|LLM provider rate limits      |High      |Medium  |Multi-provider support; proactive limit increase requests; implement queue overflow |
|IndexedDB storage limits      |Medium    |Medium  |Implement LRU eviction; monitor quota usage; warn users at 80%                      |
|Vultr VPS failure             |Low       |High    |Documented recovery procedure; < 1 hour RTO from snapshot; consider standby instance|
|Team attrition                |Medium    |High    |Documentation-first culture; cross-training; no single points of failure            |
|Scope creep                   |High      |Medium  |Strict PRD adherence; change control process; PM empowered to say no                |
|LLM API changes               |Medium    |Medium  |Abstract LLM interactions; version-pin APIs; monitor deprecation notices            |

### 13.1 Contingency Plans

**NATS Unavailable (> 5 min):**

1. Immediate: Alert on-call, SSH into Vultr VPS, check systemd status
1. 15 min: If unrecoverable, provision new VPS from latest snapshot
1. 30 min: Restore from snapshot, update Northflank env vars with new IP
1. Recovery: Post-incident review, consider standby instance for future

**Vultr VPS Hardware Failure:**

1. Immediate: Vultr auto-migrates to healthy hardware (usually < 15 min)
1. If prolonged: Provision new instance, restore from snapshot
1. DNS TTL is 5 min, so traffic shifts within 10 min of IP update

**LLM Provider Outage:**

1. Immediate: Failover to secondary provider (if configured)
1. Client: Show degraded state message, allow read-only access
1. Recovery: Queue failed requests for retry

-----

## 14. Success Metrics

### 14.1 Technical KPIs

|Metric                          |Target              |Measurement Method      |
|--------------------------------|--------------------|------------------------|
|Token streaming latency (P95)   |< 100ms             |API metrics (Prometheus)|
|Message persistence success rate|99.99%              |JetStream ack rate      |
|Conversation replay time (P95)  |< 500ms             |API metrics             |
|System uptime                   |99.9%               |Northflank monitoring   |
|Client offline capability       |100% read, 95% write|QA validation           |
|API error rate                  |< 0.1%              |API metrics             |
|Mean time to recovery (MTTR)    |< 30 min            |Incident tracking       |

### 14.2 Business KPIs

|Metric                                  |Target      |Measurement Method    |
|----------------------------------------|------------|----------------------|
|User-perceived latency satisfaction     |> 90%       |User surveys, NPS     |
|Message delivery reliability            |> 99.9%     |Audit logs            |
|Infrastructure cost per 1K conversations|< $X (TBD)  |Cloud billing analysis|
|Time to onboard new tenant              |< 1 hour    |Operational logs      |
|Incident response time (P1)             |< 15 minutes|PagerDuty metrics     |

### 14.3 Launch Criteria

The system is ready for production launch when:

1. ✅ All Phase 1-4 deliverables complete and validated
1. ✅ Load test demonstrates 10K concurrent conversations
1. ✅ Security audit passed with no critical/high findings
1. ✅ Disaster recovery tested and documented
1. ✅ On-call rotation staffed and trained
1. ✅ Product and engineering sign-off obtained
1. ✅ Runbooks complete for top 10 failure scenarios
1. ✅ Monitoring dashboards reviewed and approved

-----

## Appendix A: Technology Decisions

### A.1 Why NATS JetStream over Alternatives

|Alternative             |Reason for Rejection                                                                |
|------------------------|------------------------------------------------------------------------------------|
|Kafka                   |Operational complexity disproportionate to scale; JVM overhead; ZooKeeper dependency|
|Redis Streams           |Persistence less mature; clustering more complex; memory-bound                      |
|Postgres + LISTEN/NOTIFY|Not designed for high-throughput streaming; polling overhead                        |
|AWS Kinesis             |Vendor lock-in; cost at scale; cold start latency; shard management                 |
|Custom WebSocket + DB   |Significant development effort; reinventing replay semantics                        |

**Why NATS JetStream wins:**

- Single binary, minimal operational overhead
- Native persistence with replay semantics
- Subject-based filtering without consumer groups
- Excellent Go client library
- Reasonable defaults with production-ready config
- Active development and community

### A.2 Why Vultr VPS over Container Platforms for NATS

|Concern            |Container (Northflank/K8s)                       |Dedicated VPS                      |
|-------------------|-------------------------------------------------|-----------------------------------|
|Persistent storage |PVC complexity, potential data loss on reschedule|Native disk, survives reboots      |
|Performance        |Variable, noisy neighbors                        |Consistent, dedicated resources    |
|Networking         |Service mesh overhead                            |Direct TCP, lower latency          |
|Recovery           |Pod restart may relocate, new PVC mount          |Same disk, same IP, faster recovery|
|Minimal viable cost|~$40-60/mo                                       |**$5/mo**                          |
|Production cost    |~$80-120/mo for equivalent                       |$48/mo                             |
|Operations         |Managed, but stateful workloads are complex      |SSH access, simpler mental model   |

**The insight:** Stateless scales horizontally on platforms like Northflank. Stateful (databases, message brokers) wants a stable home. NATS is efficient enough to run on a $5/mo instance for personal use, then scale to $48/mo when you have customers — same architecture, same code, just resize the VM. When you need HA, add two more instances and configure NATS clustering.

### A.3 Why SSE over WebSocket

- **Unidirectional data flow** matches our use case (server → client tokens)
- **Simpler implementation** and debugging (standard HTTP)
- **Native browser reconnection** support
- **Works through proxies** and load balancers without special configuration
- **@microsoft/fetch-event-source** provides POST support and robust reconnection
- WebSocket can be added later if bidirectional streaming needed

### A.4 Why Dexie.js over Raw IndexedDB

- **Promise-based API** vs callback hell
- **TypeScript support** with type-safe queries
- **Built-in hooks** for reactive updates (useLiveQuery)
- **Bulk operations** optimized
- **Versioning and migrations** handled cleanly
- **Mature library** with active maintenance (9+ years)

### A.5 Why @microsoft/fetch-event-source

- **POST request support** (native EventSource is GET-only)
- **Automatic reconnection** with backoff
- **Header customization** for auth tokens
- **Error handling** with retry logic
- **TypeScript support**
- **Battle-tested** at Microsoft scale

-----

## Appendix B: Go Project Structure

```
/
├── cmd/
│   └── api/
│       └── main.go              # Entry point
├── internal/
│   ├── config/
│   │   └── config.go            # Environment configuration
│   ├── handler/
│   │   ├── conversations.go     # Conversation endpoints
│   │   ├── messages.go          # Message endpoints
│   │   └── stream.go            # SSE streaming
│   ├── middleware/
│   │   ├── auth.go              # JWT authentication
│   │   ├── cors.go              # CORS handling
│   │   ├── logging.go           # Request logging
│   │   ├── ratelimit.go         # Rate limiting
│   │   └── security.go          # Security headers
│   ├── nats/
│   │   ├── client.go            # NATS connection management
│   │   ├── stream.go            # JetStream stream operations
│   │   └── consumer.go          # Consumer management
│   ├── llm/
│   │   ├── client.go            # LLM client interface
│   │   ├── anthropic.go         # Anthropic implementation
│   │   └── openai.go            # OpenAI implementation
│   ├── model/
│   │   ├── conversation.go      # Conversation types
│   │   ├── message.go           # Message types
│   │   └── event.go             # Event types
│   └── service/
│       ├── conversation.go      # Business logic
│       └── message.go           # Message handling
├── pkg/
│   ├── logger/
│   │   └── logger.go            # Structured logging
│   ├── metrics/
│   │   └── metrics.go           # Prometheus metrics
│   └── tracing/
│       └── tracing.go           # OpenTelemetry setup
├── scripts/
│   ├── migrate.go               # Stream setup scripts
│   └── seed.go                  # Test data seeding
├── Dockerfile
├── docker-compose.yml           # Local development
├── go.mod
├── go.sum
└── README.md
```

-----

## Appendix C: Client Project Structure

```
/
├── src/
│   ├── api/
│   │   ├── client.ts            # API client setup
│   │   ├── conversations.ts     # Conversation API calls
│   │   └── messages.ts          # Message API calls
│   ├── components/
│   │   ├── Chat/
│   │   │   ├── ChatContainer.tsx
│   │   │   ├── MessageList.tsx
│   │   │   ├── MessageInput.tsx
│   │   │   └── StreamingMessage.tsx
│   │   ├── Sidebar/
│   │   │   ├── ConversationList.tsx
│   │   │   └── ConversationItem.tsx
│   │   └── common/
│   │       ├── Button.tsx
│   │       ├── Input.tsx
│   │       └── Loading.tsx
│   ├── db/
│   │   ├── index.ts             # Dexie database setup
│   │   ├── schema.ts            # TypeScript interfaces
│   │   └── sync.ts              # Sync utilities
│   ├── hooks/
│   │   ├── useConversation.ts   # Conversation data hook
│   │   ├── useSendMessage.ts    # Message mutation hook
│   │   └── useStream.ts         # SSE connection hook
│   ├── lib/
│   │   ├── auth.ts              # Authentication utilities
│   │   └── stream.ts            # SSE client class
│   ├── pages/
│   │   ├── index.tsx            # Home/conversation list
│   │   └── chat/[id].tsx        # Chat view
│   ├── store/
│   │   └── useAppStore.ts       # Zustand store
│   ├── styles/
│   │   └── globals.css          # Global styles
│   ├── App.tsx
│   └── main.tsx
├── public/
├── index.html
├── package.json
├── tsconfig.json
├── vite.config.ts
└── vercel.json
```

-----

## Document History

|Version|Date        |Author     |Changes        |
|-------|------------|-----------|---------------|
|1.0    |January 2026|Engineering|Initial release|

-----

*End of Document*



# formerly AI Capitalizer Chrome Extension

A simple Chrome Extension that automatically capitalizes instances of 'ai' on web pages.

## Features

- Automatically capitalizes 'ai' to 'AI' on any webpage
- Works with dynamically loaded content
- Case-insensitive matching
- No configuration required

## Installation

1. Download or clone this repository
2. Open Chrome and navigate to `chrome://extensions/`
3. Enable "Developer mode" in the top right corner
4. Click "Load unpacked" and select the directory containing the extension files
5. The extension will be installed and active immediately

## Usage

Once installed, the extension will automatically capitalize instances of 'ai' on any webpage you visit. No additional configuration is needed.

## Files

- `manifest.json`: Extension configuration
- `content.js`: Main script that handles the text replacement
- `README.md`: This file

## License

MIT License 
