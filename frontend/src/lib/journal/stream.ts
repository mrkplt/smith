import { appState } from '$lib/stores';

/** Appends a journal entry to the shared store if it advances the sequence. */
export function appendJournalEntry(entry: any) {
    if (!entry || typeof entry !== 'object') return;
    appState.update(state => {
        const seq = Number(entry.sequence || entry.Sequence || 0);
        if (seq <= state.journalLastSeq) return state;

        const journalEntries = [...state.journalEntries, entry].slice(-500);
        return { ...state, journalEntries, journalLastSeq: seq };
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
export function openJournalStream(loopID: string, onEntry: (entry: any) => void, onError: () => void) {
    const url = `/api/v1/loops/${encodeURIComponent(loopID)}/journal/stream`;
    const source = new EventSource(url);

    source.onmessage = event => {
        try {
            onEntry(JSON.parse(event.data));
        } catch (err) {
            console.error('Failed to parse journal entry', err);
        }
    };

    source.onerror = () => {
        source.close();
        onError();
    };

    return source;
}
