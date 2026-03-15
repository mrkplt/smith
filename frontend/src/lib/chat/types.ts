export type MessageRole = 'user' | 'assistant' | 'system';

export interface Message {
    id: string;
    role: MessageRole;
    content: string;
    timestamp: Date;
}

export type ChatEventType =
    | 'message.delta'
    | 'message.completed'
    | 'tool.started'
    | 'tool.completed'
    | 'structured.result'
    | 'error';

export interface ChatEvent {
    event: ChatEventType;
    data: any;
}
