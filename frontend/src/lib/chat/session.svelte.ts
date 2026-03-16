import { commitChatAction, createChatSession, openChatStream, postChatMessage } from '$lib/chat/transport';
import type { Message } from '$lib/chat/types';

interface DebugEvent {
    at: string;
    type: string;
    data: string;
}

/** Manages frontend chat session state and streaming events. */
export class ChatSession {
    sessionId = $state<string | null>(null);
    messages = $state<Message[]>([]);
    structuredResults = $state<any[]>([]);
    isCreating = $state(false);
    isStreaming = $state(false);
    activeTool = $state<string | null>(null);
    error = $state<string | null>(null);
    debugEvents = $state<DebugEvent[]>([]);

    async createSession(type: string, context: Record<string, string>): Promise<boolean> {
        this.isCreating = true;
        this.pushDebug('session.create.start', JSON.stringify({ type, context }));
        try {
            const response = await createChatSession({ type, context });
            this.pushDebug('session.create.response', `status=${response.status}`);
            if (!response.ok) {
                throw new Error(await readErrorResponse(response));
            }

            const data = await response.json();
            if (!data?.sessionId) {
                throw new Error('chat session creation returned no sessionId');
            }
            this.sessionId = data.sessionId;
            this.messages = [];
            this.structuredResults = [];
            this.activeTool = null;
            this.error = null;
            this.pushDebug('session.create.ok', `sessionId=${data.sessionId}`);
            return true;
        } catch (err: any) {
            this.error = 'Failed to create session: ' + err.message;
            this.sessionId = null;
            this.pushDebug('session.create.error', err?.message || 'unknown error');
            return false;
        } finally {
            this.isCreating = false;
        }
    }

    async sendMessage(content: string) {
        if (!this.sessionId || this.isCreating || this.isStreaming) return;

        const trimmed = content.trim();
        if (!trimmed) return;
        this.pushDebug('message.send.start', trimmed);

        this.structuredResults = [];
        this.messages.push({
            id: 'msg_' + Date.now(),
            role: 'user',
            content: trimmed,
            timestamp: new Date()
        });
        this.error = null;

        try {
            const response = await postChatMessage(this.sessionId, trimmed);
            this.pushDebug('message.send.response', `status=${response.status}`);
            if (!response.ok && response.status !== 409) {
                throw new Error(await readErrorResponse(response));
            }
            this.startStreaming(this.ensureAssistantMessage());
        } catch (err: any) {
            this.error = 'Failed to send message: ' + err.message;
            this.pushDebug('message.send.error', err?.message || 'unknown error');
        }
    }

    async commitAction(action: string, payload: any) {
        try {
            const response = await commitChatAction({ action, payload });
            if (response.ok) {
                this.structuredResults = this.structuredResults.filter(result => result.payload !== payload);
                return true;
            }
        } catch (err: any) {
            this.error = 'Failed to commit action: ' + err.message;
        }
        return false;
    }

    private startStreaming(assistantMessageID: string) {
        if (!this.sessionId) return;

        this.isStreaming = true;
        const eventSource = openChatStream(this.sessionId);
        this.pushDebug('stream.open', `sessionId=${this.sessionId}`);
        let completed = false;

        const cleanup = () => {
            this.isStreaming = false;
            this.activeTool = null;
            eventSource.close();
            if (this.getMessageContent(assistantMessageID).trim() === '') {
                this.messages = this.messages.filter((msg) => msg.id !== assistantMessageID);
            }
            this.pushDebug('stream.close', 'closed');
        };

        const complete = () => {
            if (completed) return;
            completed = true;
            cleanup();
        };

        eventSource.addEventListener('message.delta', (event: MessageEvent) => {
            const data = parseJSON<{ delta?: string }>(event.data);
            if (typeof data?.delta === 'string') {
                this.appendAssistantDelta(assistantMessageID, data.delta);
                this.pushDebug('message.delta', data.delta);
            }
        });

        eventSource.addEventListener('tool.started', (event: MessageEvent) => {
            const data = parseJSON<{ tool?: string }>(event.data);
            this.activeTool = data?.tool || null;
            this.pushDebug('tool.started', data?.tool || 'unknown');
        });

        eventSource.addEventListener('tool.completed', () => {
            this.activeTool = null;
            this.pushDebug('tool.completed', 'done');
        });

        eventSource.addEventListener('structured.result', (event: MessageEvent) => {
            const parsed = parseJSON<any>(event.data);
            if (parsed) {
                this.structuredResults.push(parsed);
                this.pushDebug('structured.result', JSON.stringify(parsed));
            }
        });

        eventSource.addEventListener('message.completed', () => {
            this.pushDebug('message.completed', 'done');
            complete();
        });

        eventSource.addEventListener('error', (event: Event) => {
            const maybeEvent = event as MessageEvent;
            const payload = parseJSON<{ message?: string; error?: string }>(maybeEvent?.data);
            this.error = payload?.message || payload?.error || 'Streaming error';
            this.pushDebug('stream.error', payload?.message || payload?.error || 'Streaming error');
            complete();
        });
    }

    private ensureAssistantMessage(): string {
        const assistantMessage: Message = {
            id: 'msg_ast_' + Date.now(),
            role: 'assistant',
            content: '',
            timestamp: new Date()
        };
        this.messages.push(assistantMessage);
        return assistantMessage.id;
    }

    private appendAssistantDelta(messageID: string, delta: string) {
        this.messages = this.messages.map((msg) => {
            if (msg.id !== messageID) {
                return msg;
            }
            return {
                ...msg,
                content: msg.content + delta
            };
        });
    }

    private getMessageContent(messageID: string): string {
        const target = this.messages.find((msg) => msg.id === messageID);
        return target?.content || '';
    }

    private pushDebug(type: string, data: string) {
        const next: DebugEvent = {
            at: new Date().toLocaleTimeString(),
            type,
            data: truncate(data, 220)
        };
        this.debugEvents = [...this.debugEvents.slice(-79), next];
    }
}

export const chatSession = new ChatSession();

async function readErrorResponse(response: Response): Promise<string> {
    try {
        const payload = await response.json();
        if (typeof payload?.error === 'string' && payload.error.trim() !== '') {
            return payload.error;
        }
        if (typeof payload?.message === 'string' && payload.message.trim() !== '') {
            return payload.message;
        }
    } catch {
        // Fall through to status text fallback.
    }
    return `HTTP ${response.status}`;
}

function parseJSON<T>(value: unknown): T | null {
    if (typeof value !== 'string' || value.trim() === '') {
        return null;
    }
    try {
        return JSON.parse(value) as T;
    } catch {
        return null;
    }
}

function truncate(value: string, max: number): string {
    if (value.length <= max) {
        return value;
    }
    return value.slice(0, max - 3) + '...';
}
