import { requestJSON } from '$lib/api';

export type PRDFormat = 'markdown' | 'json';

export interface PRDValidationDiagnostic {
  code: string;
  path: string;
  storyId?: string;
  message: string;
  suggestion?: string;
}

export interface PRDValidationReport {
  valid: boolean;
  errors: PRDValidationDiagnostic[];
  warnings: PRDValidationDiagnostic[];
  readiness: 'pass' | 'warn' | 'fail';
}

export interface PRDValidateResponse {
  format: PRDFormat;
  report: PRDValidationReport;
  canonical_markdown?: string;
}

/** Chooses JSON mode when content starts as an object payload, markdown otherwise. */
export function inferPRDFormat(content: string): PRDFormat {
  return content.trim().startsWith('{') ? 'json' : 'markdown';
}

/** Validates PRD content against canonical readiness criteria. */
export async function validatePRDContent(content: string, format: PRDFormat): Promise<PRDValidateResponse> {
  const payload = format === 'json'
    ? { format: 'json', json: content }
    : { format: 'markdown', markdown: content };
  return requestJSON('/v1/prd/validate', 'POST', payload);
}
