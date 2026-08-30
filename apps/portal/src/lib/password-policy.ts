const BLOCKED_PASSWORDS = new Set([
  "password", "passwordpassword", "123456789012", "qwertyuiopas",
  "ghanageo", "administrator", "letmeinletmein",
]);

export function passwordPolicyError(password: string): string | null {
  const length = Array.from(password).length;
  if (length < 12) return "Use at least 12 characters.";
  if (length > 1024) return "Use at most 1,024 characters.";
  const normalized = password.trim().toLocaleLowerCase();
  if (BLOCKED_PASSWORDS.has(normalized)) return "Choose a less common password.";
  if (new Set(Array.from(normalized)).size <= 2) return "Use more variety in your password.";
  return null;
}
