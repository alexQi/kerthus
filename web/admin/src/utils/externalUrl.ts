/** Browser destinations only. Never resolve relative URLs against the SaaS origin. */
export function externalUrl(value: unknown): string {
  if (typeof value !== 'string') return '';
  const input = value.trim();
  if (
    !/^https?:\/\/[^/]/i.test(input) ||
    /^https?:\/\/[^/?#]*@/i.test(input) ||
    /[\s\\\u0000-\u001f\u007f]/.test(input)
  )
    return '';
  try {
    const url = new URL(input);
    if (
      !['http:', 'https:'].includes(url.protocol) ||
      !url.hostname ||
      url.username ||
      url.password
    )
      return '';
    return input;
  } catch {
    return '';
  }
}

export function isExternalDestination(value: string): boolean {
  return /^(?:[a-z][a-z\d+.-]*:|\/\/|\\)/i.test(value.trim());
}

export function normalizeAppType(value: unknown): string {
  return value === 'thrid' ? 'third' : typeof value === 'string' && value ? value : 'self';
}

export function isThirdPartyApp(app: { type?: unknown }): boolean {
  return normalizeAppType(app.type) === 'third';
}

/** A synchronous user click opens only the configured URL, without platform state or credentials. */
export function openExternalUrl(value: unknown): boolean {
  const url = externalUrl(value);
  if (!url) return false;
  const link = document.createElement('a');
  link.href = url;
  link.target = '_blank';
  link.rel = 'noopener noreferrer';
  link.referrerPolicy = 'no-referrer';
  link.click();
  return true;
}
