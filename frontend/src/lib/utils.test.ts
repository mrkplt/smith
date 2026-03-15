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

    // Test handling of multiple consecutive non-alphanumeric characters
    expect(slugifySegment('foo   bar')).toBe('foo-bar');
    expect(slugifySegment('foo!@#bar')).toBe('foo-bar');

    // Test stripping of leading and trailing non-alphanumeric characters
    expect(slugifySegment('  leading space')).toBe('leading-space');
    expect(slugifySegment('trailing space  ')).toBe('trailing-space');
    expect(slugifySegment('---dashes-around---')).toBe('dashes-around');

    // Test completely non-alphanumeric strings
    expect(slugifySegment('!@#$%^&*()')).toBe('');

    // Test empty string
    expect(slugifySegment('')).toBe('');

    // Test non-string input coercion (assuming it's allowed due to String(v) cast)
    expect(slugifySegment(123 as any)).toBe('123');
    expect(slugifySegment(null as any)).toBe('null');
  });

  it('normalizes branch names without stripping valid separators', () => {
    expect(normalizeBranchName('feature/my branch@2026')).toBe('feature/my-branch-2026');
  });
});
