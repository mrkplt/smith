import { commitChatAction, createChatSession, openChatStream, postChatMessage } from '$lib/chat/transport';
import type { Message } from '$lib/chat/types';

/** Manages frontend chat session state and streaming events. */
export class ChatSession {
    sessionId = $state<string | null>(null);
    messages = $state<Message[]>([]);
    structuredResults = $state<any[]>([]);
    isStreaming = $state(false);
    activeTool = $state<string | null>(null);
    error = $state<string | null>(null);

    async createSession(type: string, context: Record<string, string>) {
        try {
            const response = await createChatSession({ type, context });
            const data = await response.json();
            this.sessionId = data.sessionId;
            this.messages = [];
            this.structuredResults = [];
            this.error = null;
        } catch (err: any) {
            this.error = 'Failed to create session: ' + err.message;
        }
    }

    async sendMessage(content: string) {
        if (!this.sessionId) return;

        this.structuredResults = [];
        this.messages.push({
            id: 'msg_' + Date.now(),
            role: 'user',
            content,
            timestamp: new Date()
        });
        this.error = null;

        try {
            await postChatMessage(this.sessionId, content);
            this.startStreaming();
        } catch (err: any) {
            this.error = 'Failed to send message: ' + err.message;
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

    private startStreaming() {
        if (!this.sessionId) return;

        this.isStreaming = true;
        const eventSource = openChatStream(this.sessionId);
        const assistantMessage = this.ensureAssistantMessage();

        eventSource.addEventListener('message.delta', (event: MessageEvent) => {
            const data = JSON.parse(event.data);
            assistantMessage.content += data.delta;
        });

        eventSource.addEventListener('tool.started', (event: MessageEvent) => {
            const data = JSON.parse(event.data);
            this.activeTool = data.tool;
        });

        eventSource.addEventListener('tool.completed', () => {
            this.activeTool = null;
        });

        eventSource.addEventListener('structured.result', (event: MessageEvent) => {
            this.structuredResults.push(JSON.parse(event.data));
        });

        eventSource.addEventListener('message.completed', () => {
            this.isStreaming = false;
            eventSource.close();
        });

        eventSource.addEventListener('error', () => {
            this.error = 'Streaming error';
            this.isStreaming = false;
            eventSource.close();
        });
    }

    private ensureAssistantMessage(): Message {
        const assistantMessage: Message = {
            id: 'msg_ast_' + Date.now(),
            role: 'assistant',
            content: '',
            timestamp: new Date()
        };
        this.messages.push(assistantMessage);
        return assistantMessage;
    }
}

export const chatSession = new ChatSession();
