(function installToolFormatter(root) {
  const presentations = Object.assign(Object.create(null), {
    Read: ["读取文件", "file_path"],
    Write: ["写入文件", "file_path"],
    Edit: ["编辑文件", "file_path"],
    MultiEdit: ["编辑文件", "file_path"],
    NotebookEdit: ["编辑笔记", "notebook_path"],
    Bash: ["执行命令", "command"],
    Glob: ["查找文件", "pattern"],
    Grep: ["搜索内容", "pattern"],
    WebFetch: ["读取网页", "url"],
    WebSearch: ["搜索网页", "query"]
  });

  function formatToolUse(tool, input) {
    const presentation = presentations[tool];
    const [label, preferredKey] = presentation || [tool || "工具", ""];
    let value = input;
    if (typeof input === "string") {
      const trimmed = input.trim();
      if (!trimmed) return `🔧 ${label}`;
      try { value = JSON.parse(trimmed); }
      catch { return `🔧 ${label}\n${trimmed}`; }
    }
    if (!value || typeof value !== "object") {
      return value === undefined || value === null || value === "" ? `🔧 ${label}` : `🔧 ${label}\n${value}`;
    }
    const detail = preferredKey && value[preferredKey] !== undefined ? value[preferredKey] : "";
    if (detail !== "") return `🔧 ${label}\n${Array.isArray(detail) ? detail.join(", ") : detail}`;
    if (Object.keys(value).length === 0) return `🔧 ${label}`;
    return `🔧 ${label}\n${JSON.stringify(value, null, 2)}`;
  }

  const api = { formatToolUse };
  if (typeof module !== "undefined" && module.exports) module.exports = api;
  else root.CodeAfarToolFormat = api;
})(typeof globalThis === "undefined" ? this : globalThis);
