const test = require("node:test");
const assert = require("node:assert/strict");

const { formatToolUse } = require("./tool-format.js");

test("formats completed tool inputs for people", () => {
  assert.equal(formatToolUse("Read", '{"file_path":"/tmp/README.md","limit":1}'), "🔧 读取文件\n/tmp/README.md");
  assert.equal(formatToolUse("Bash", '{"command":"git status --short"}'), "🔧 执行命令\ngit status --short");
  assert.equal(formatToolUse("Read", "{}"), "🔧 读取文件");
});

test("unknown and prototype-named tools always have a safe fallback", () => {
  assert.equal(formatToolUse("constructor", '{"value":1}'), '🔧 constructor\n{\n  "value": 1\n}');
  assert.equal(formatToolUse("__proto__", "{}"), "🔧 __proto__");
  assert.equal(formatToolUse("CustomTool", "raw input"), "🔧 CustomTool\nraw input");
});
