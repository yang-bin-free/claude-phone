(function installProviderWorkspace(root) {
  const storageKey = "codeafar.providerWorkspace.v1";

  function clean(value) {
    return {
      activeProvider: typeof value?.activeProvider === "string" ? value.activeProvider : "",
      lastSessions: Object.fromEntries(Object.entries(value?.lastSessions || {}).filter(
        ([provider, session]) => typeof provider === "string" && typeof session === "string"
      )),
    };
  }

  function load(storage) {
    try { return clean(JSON.parse(storage?.getItem(storageKey) || "{}")); }
    catch { return clean({}); }
  }

  function save(storage, value) {
    try { storage?.setItem(storageKey, JSON.stringify(clean(value))); }
    catch {}
  }

  function availableProvider(providers, preferred) {
    return providers.find(item => item.id === preferred && item.available)?.id
      || providers.find(item => item.available)?.id || "";
  }

  function sessionsForProvider(sessions, providerID) {
    return (sessions || []).filter(item => item.provider === providerID);
  }

  function rememberedSession(sessions, providerID, lastSessions) {
    return sessionsForProvider(sessions, providerID)
      .find(item => item.sessionId === lastSessions?.[providerID]) || null;
  }

  const api = { load, save, availableProvider, sessionsForProvider, rememberedSession };
  if (typeof module !== "undefined" && module.exports) module.exports = api;
  else root.CodeAfarProviderWorkspace = api;
})(typeof globalThis === "undefined" ? this : globalThis);
