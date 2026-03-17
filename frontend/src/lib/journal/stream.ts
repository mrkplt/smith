import { appState } from '$lib/stores';
import { apiBaseUrl } from '$lib/api';

/** Appends a journal entry to the shared store if it advances the sequence. */
export function appendJournalEntry(entry: any) {
    if (!entry || typeof entry !== 'object') return;
    appState.update(state => {
        const seq = Number(entry.sequence || entry.Sequence || 0);
        if (seq <= state.journalLastSeq) return state;

        const journalEntries = [...state.journalEntries, entry].slice(-500);
        let latencySamplesMs = state.latencySamplesMs;
        const timestamp = String(entry.timestamp || entry.Timestamp || '').trim();
        if (timestamp !== '') {
            const parsedMs = Date.parse(timestamp);
            if (Number.isFinite(parsedMs)) {
                const latencyMs = Math.floor(Date.now() - parsedMs);
                if (latencyMs >= 0 && latencyMs < 600000) {
                    latencySamplesMs = [...state.latencySamplesMs, latencyMs].slice(-500);
                }
            }
        }

        return { ...state, journalEntries, journalLastSeq: seq, latencySamplesMs };
    });
}

/** Renders journal entries into the terminal-like plain text view. */
export function renderJournalText(entries: any[]) {
    if (entries.length === 0) {
        return '[journal] waiting for entries...\n';
    }

    const lines = entries.map((entry: any) => {
        const ts = String(entry.timestamp || entry.Timestamp || '');
        const level = String(entry.level || entry.Level || 'info').toLowerCase();
        const phase = String(entry.phase || entry.Phase || '-');
        const actor = String(entry.actor_id || entry.ActorID || '-');
        const msg = String(entry.message || entry.Message || '');
        return `[${ts}] [${level}] [${phase}] [${actor}] ${msg}`;
    });

    return lines.join('\n') + '\n';
}

/** Opens the loop journal event stream and wires entry/error callbacks. */
export function openJournalStream(
    loopID: string,
    onEntry: (entry: any) => void,
    onError: () => void,
    sinceSeq = 0
) {
    const safeSinceSeq = Number.isFinite(sinceSeq) ? Math.max(0, Math.floor(sinceSeq)) : 0;
    const url = `${apiBaseUrl}/v1/loops/${encodeURIComponent(loopID)}/journal/stream?since_seq=${safeSinceSeq}`;
    const source = new EventSource(url);

    const handleMessage = (raw: string) => {
        try {
            const payload = JSON.parse(raw);
            const entry = payload && typeof payload === 'object' && payload.entry ? payload.entry : payload;
            onEntry(entry);
        } catch (err) {
            console.error('Failed to parse journal entry', err);
        }
    };

    source.addEventListener('entry', event => {
        handleMessage((event as MessageEvent<string>).data);
    });

    source.onmessage = event => {
        handleMessage(event.data);
    };

    source.onerror = () => {
        source.close();
        onError();
    };

    return source;
}
