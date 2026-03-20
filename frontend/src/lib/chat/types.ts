export type MessageRole = 'user' | 'assistant' | 'system';

export interface Message {
    id: string;
    role: MessageRole;
    content: string;
    timestamp: Date;
}

export type ChatEventType =
    | 'session.started'
    | 'context.loaded'
    | 'stream.keepalive'
    | 'message.delta'
    | 'message.completed'
    | 'tool.started'
    | 'tool.completed'
    | 'document.patch.proposed'
    | 'readiness.updated'
    | 'structured.result'
    | 'error';

export interface ChatEvent {
    event: ChatEventType;
    data: any;
}
