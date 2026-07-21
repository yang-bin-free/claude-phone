const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

const modulePath = path.join(__dirname, "provider-workspace.js");

test("provider workspace module exists", () => {
  assert.equal(fs.existsSync(modulePath), true, "provider-workspace.js must exist");
});

test("filters and restores sessions independently per provider", () => {
  const workspace = require(modulePath);
  const sessions = [
    { sessionId: "c1", provider: "claude" },
    { sessionId: "x1", provider: "codex" },
  ];
  assert.deepEqual(workspace.sessionsForProvider(sessions, "codex"), [sessions[1]]);
  assert.equal(workspace.rememberedSession(sessions, "claude", { claude: "c1", codex: "x1" }), sessions[0]);
  assert.equal(workspace.rememberedSession(sessions, "codex", { codex: "missing" }), null);
});

test("persists only provider and session identifiers", () => {
  const workspace = require(modulePath);
  const values = new Map();
  const storage = { getItem: key => values.get(key) ?? null, setItem: (key, value) => values.set(key, value) };
  workspace.save(storage, { activeProvider: "codex", lastSessions: { claude: "c1", codex: "x1" } });
  assert.deepEqual(workspace.load(storage), { activeProvider: "codex", lastSessions: { claude: "c1", codex: "x1" } });
  assert.doesNotMatch(values.values().next().value, /token|content|message/i);
});

test("falls back to the first available provider", () => {
  const workspace = require(modulePath);
  const providers = [
    { id: "claude", available: false },
    { id: "codex", available: true },
  ];
  assert.equal(workspace.availableProvider(providers, "claude"), "codex");
  assert.equal(workspace.availableProvider([], "claude"), "");
});
