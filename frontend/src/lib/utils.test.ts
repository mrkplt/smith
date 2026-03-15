import { describe, expect, it } from 'vitest';

import { escapeHtml, normalizeBranchName, slugifySegment } from '$lib/utils';

describe('utils', () => {
  it('escapes HTML-sensitive characters', () => {
    expect(escapeHtml(`<script>alert("x")</script>`)).toBe(
      '&lt;script&gt;alert(&quot;x&quot;)&lt;/script&gt;'
    );
  });

  it('slugifies mixed input into lowercase segments', () => {
    expect(slugifySegment('Feature Branch 123!')).toBe('feature-branch-123');
  });

  it('normalizes branch names without stripping valid separators', () => {
    expect(normalizeBranchName('feature/my branch@2026')).toBe('feature/my-branch-2026');
  });
});
