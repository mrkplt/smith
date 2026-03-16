import type { AppState } from '$lib/stores';

type ContextState = Pick<AppState, 'activeProvider' | 'documents' | 'loops' | 'projects' | 'selectedLoop'>;

const CONTEXT_QUERY_PREFIX = 'ctx_';
const SENSITIVE_CONTEXT_KEYS = new Set(['providerApiKey', 'apiKey']);

/** Builds the allowlisted chat context for the current application surface. */
export function buildGlobalChatContext(
    pathname: string,
    sessionType: string,
    state: ContextState
): Record<string, string> {
    const next: Record<string, string> = {
        app: 'smith-console',
        route: pathname,
        surface: detectSurface(pathname),
        sessionType
    };

    if (state.projects.length > 0) {
        next.projectsCount = String(state.projects.length);
    }
    if (state.documents.length > 0) {
        next.documentsCount = String(state.documents.length);
    }
    if (state.loops.length > 0) {
        next.loopsCount = String(state.loops.length);
        next.activeLoopsCount = String(
            state.loops.filter((loop: any) => {
                const status = String(loop?.status || '').toLowerCase();
                return status === 'running' || status === 'unresolved';
            }).length
        );
    }
    if (state.activeProvider.trim() !== '') {
        next.activeProvider = state.activeProvider;
    }

    const routeLoopID = extractLoopIDFromPath(pathname);
    if (routeLoopID !== '') {
        next.loopId = routeLoopID;
    } else if (state.selectedLoop.trim() !== '') {
        next.loopId = state.selectedLoop;
    }

    if (next.loopId) {
        const selected = state.loops.find((loop: any) => String(loop?.loopID || '') === next.loopId);
        const projectID = String(selected?.project || '').trim();
        if (projectID !== '') {
            next.projectId = projectID;
        }
    }

    return next;
}

/** Encodes non-sensitive chat context into the full-screen assistant URL. */
export function buildChatPageURL(context: Record<string, string>, returnToPath: string): string {
    const params = new URLSearchParams();
    params.set('returnTo', sanitizeReturnToPath(returnToPath));

    for (const [key, value] of Object.entries(context)) {
        if (SENSITIVE_CONTEXT_KEYS.has(key)) {
            continue;
        }
        const trimmed = value.trim();
        if (trimmed === '') {
            continue;
        }
        params.set(`${CONTEXT_QUERY_PREFIX}${key}`, trimmed);
    }

    return `/assistant?${params.toString()}`;
}

/** Extracts chat context carried through assistant route query parameters. */
export function extractCarriedChatContext(searchParams: URLSearchParams): Record<string, string> {
    const next: Record<string, string> = {};
    for (const [key, value] of searchParams.entries()) {
        if (!key.startsWith(CONTEXT_QUERY_PREFIX)) {
            continue;
        }
        const contextKey = key.slice(CONTEXT_QUERY_PREFIX.length);
        const trimmed = value.trim();
        if (contextKey === '' || trimmed === '') {
            continue;
        }
        next[contextKey] = trimmed;
    }
    return next;
}

/** Produces a stable signature used to detect meaningful context changes. */
export function contextSignature(context: Record<string, string>): string {
    const entries = Object.entries(context)
        .map(([key, value]) => [key, value.trim()] as const)
        .filter(([, value]) => value !== '')
        .sort(([a], [b]) => a.localeCompare(b));
    return JSON.stringify(entries);
}

/** Normalizes return paths so assistant navigation cannot loop back into itself. */
export function sanitizeReturnToPath(pathname: string): string {
    const trimmed = pathname.trim();
    if (trimmed === '' || !trimmed.startsWith('/')) {
        return '/projects';
    }
    if (trimmed.startsWith('/assistant')) {
        return '/projects';
    }
    return trimmed;
}

function detectSurface(pathname: string): string {
    if (pathname.startsWith('/pod-view/')) {
        return 'pod-view';
    }
    if (pathname.startsWith('/pods')) {
        return 'pods';
    }
    if (pathname.startsWith('/documents')) {
        return 'documents';
    }
    if (pathname.startsWith('/projects')) {
        return 'projects';
    }
    if (pathname.startsWith('/providers')) {
        return 'providers';
    }
    if (pathname.startsWith('/settings')) {
        return 'settings';
    }
    if (pathname.startsWith('/assistant')) {
        return 'chat';
    }
    return 'app';
}

function extractLoopIDFromPath(pathname: string): string {
    if (!pathname.startsWith('/pod-view/')) {
        return '';
    }
    const segments = pathname.split('/').filter(Boolean);
    if (segments.length < 2) {
        return '';
    }
    return decodeURIComponent(segments[1] || '').trim();
}
