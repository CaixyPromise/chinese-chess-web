export function safeReturnPath(value: string | null | undefined): string {
  if (
    !value ||
    !value.startsWith("/") ||
    value.startsWith("//") ||
    /[\\\u0000-\u001f\u007f]/.test(value)
  )
    return "/account";
  try {
    const base = "https://local.invalid",
      url = new URL(value, base);
    return url.origin === base && !url.pathname.includes("%") ? url.pathname + url.search : "/account";
  } catch {
    return "/account";
  }
}
// Consume before the app mounts; email links use fragments, never query parameters.
export function consumeFragmentToken(): string | null {
  if (!location.hash) return null;
  const token = new URLSearchParams(location.hash.slice(1)).get("token");
  history.replaceState(history.state, "", location.pathname + location.search);
  return token;
}
