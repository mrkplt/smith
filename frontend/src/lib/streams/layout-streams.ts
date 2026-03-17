import { apiBaseUrl, fetchJSON } from '$lib/api';
import { appState, pushToast } from '$lib/stores';

/** Normalizes a loop stream payload into the UI loop shape. */
export function normalizeLoop(item: any) {
  const record = item.record || item.Record || item.state || item.State || {};
  const loopID = record.loop_id || record.LoopID || item.loop_id || item.LoopID || 'unknown-loop';
  const status = (record.state || record.State || 'unknown').toLowerCase();
  const attempt = Number(record.attempt || record.Attempt || 0);
  const reason = record.reason || record.Reason || '';
  const revision = Number(item.revision || item.Revision || record.observed_revision || 0);
  return {
    loopID,
    project: record.project_id || record.project || record.project_name || 'default',
    status,
    attempt,
    reason,
    revision
  };
}

/** Loads initial layout data required before opening realtime streams. */
export async function initLayoutState() {
  try {
    const [projects, onboarding] = await Promise.all([
      fetchJSON('/v1/projects'),
      fetchJSON('/v1/onboarding/readiness').catch(() => null)
    ]);
    appState.update(state => ({
      ...state,
      projects: Array.isArray(projects) ? projects : [],
      onboardingReady: !!onboarding?.ready,
      onboardingChecked: onboarding !== null,
      onboardingState: onboarding
    }));
  } catch (err) {
    console.error('Failed to load projects', err);
  }
}

/** Connects the layout-level SSE streams and returns a disposer. */
export function connectLayoutStreams() {
  const loopsSource = new EventSource(`${apiBaseUrl}/v1/loops/stream`);
  loopsSource.addEventListener('update', event => {
    try {
      const normalized = normalizeLoop(JSON.parse((event as MessageEvent).data));
      appState.update(state => {
        const loops = [...state.loops];
        const idx = loops.findIndex((loop: any) => loop.loopID === normalized.loopID);
        if (idx >= 0) {
          loops[idx] = normalized;
        } else {
          loops.push(normalized as never);
        }
        return { ...state, loops };
      });
    } catch {}
  });

  const docsSource = new EventSource(`${apiBaseUrl}/v1/documents/stream`);
  docsSource.addEventListener('update', event => {
    try {
      const doc = JSON.parse((event as MessageEvent).data);
      appState.update(state => {
        const docs = [...state.documents];
        const idx = docs.findIndex((item: any) => item.id === doc.id);
        if (idx >= 0) {
          docs[idx] = doc as never;
        } else {
          docs.push(doc as never);
        }
        return { ...state, documents: docs };
      });
    } catch {}
  });

  const auditSource = new EventSource(`${apiBaseUrl}/v1/audit/stream`);
  auditSource.addEventListener('update', event => {
    try {
      const rec = JSON.parse((event as MessageEvent).data);
      pushToast(`[${rec.action}] ${rec.target_loop_id}`, 'muted');
    } catch {}
  });

  return () => {
    loopsSource.close();
    docsSource.close();
    auditSource.close();
  };
}
