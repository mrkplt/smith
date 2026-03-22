import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { get } from 'svelte/store';
import { pushToast, toastMessages } from './stores';

describe('stores: pushToast', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    // Reset the store before each test
    toastMessages.set([]);
  });

  afterEach(() => {
    vi.clearAllTimers();
    vi.useRealTimers();
  });

  it('adds a toast message to the store initially with show=true', () => {
    pushToast('Test message', 'ok');

    const messages = get(toastMessages);
    expect(messages.length).toBe(1);
    expect(messages[0].message).toBe('Test message');
    expect(messages[0].level).toBe('ok');
    expect(messages[0].show).toBe(true);
    expect(typeof messages[0].id).toBe('string');
  });

  it('adds a toast message with default level="muted"', () => {
    pushToast('Default level message');

    const messages = get(toastMessages);
    expect(messages.length).toBe(1);
    expect(messages[0].level).toBe('muted');
  });

  it('hides the toast after 2800ms', () => {
    pushToast('Hide me', 'err');

    // Initially showing
    expect(get(toastMessages)[0].show).toBe(true);

    // Advance time by 2800ms
    vi.advanceTimersByTime(2800);

    // Toast should still exist but show=false
    const messages = get(toastMessages);
    expect(messages.length).toBe(1);
    expect(messages[0].show).toBe(false);
  });

  it('removes the toast completely after 2960ms (2800ms + 160ms)', () => {
    pushToast('Remove me', 'ok');

    // Advance time to exactly when it gets hidden
    vi.advanceTimersByTime(2800);
    expect(get(toastMessages).length).toBe(1);

    // Advance time by another 160ms to trigger the second timeout
    vi.advanceTimersByTime(160);

    // Toast should be removed
    expect(get(toastMessages).length).toBe(0);
  });
});
