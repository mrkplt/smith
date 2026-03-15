import { chatBaseUrl } from '$lib/api';

export interface SessionRequest {
    type: string;
    context: Record<string, string>;
}

export interface CommitActionRequest {
    action: string;
    payload: any;
}

/** Creates a new backend chat session for the requested workflow type. */
export function createChatSession(request: SessionRequest) {
    return fetch(`${chatBaseUrl}/v1/chat/sessions`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(request)
    });
}

/** Posts a user message into an existing chat session. */
export function postChatMessage(sessionId: string, message: string) {
    return fetch(`${chatBaseUrl}/v1/chat/sessions/${sessionId}/messages`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message })
    });
}

/** Opens the server-sent event stream for a chat session. */
export function openChatStream(sessionId: string) {
    return new EventSource(`${chatBaseUrl}/v1/chat/sessions/${sessionId}/stream`);
}

/** Commits a structured chat action back to the backend workflow. */
export function commitChatAction(request: CommitActionRequest) {
    return fetch(`${chatBaseUrl}/v1/chat/actions/commit`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(request)
    });
}
