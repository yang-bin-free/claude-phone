(() => {
  const state = { ws: null, sessionId: "", retry: 0 };
  const messages = document.querySelector("#messages");
  const connection = document.querySelector("#connection-state");
  const token = new URLSearchParams(location.hash.slice(1)).get("token") || "";
  window.claudePhone = { adminToken: token, state };

  function append(role, text) {
    messages.querySelector(".empty")?.remove();
    const node = document.createElement("div");
    node.className = `message ${role}`;
    node.textContent = text;
    messages.append(node);
    messages.scrollTop = messages.scrollHeight;
    return node;
  }

  function send(value) {
    if (state.ws?.readyState === WebSocket.OPEN) state.ws.send(JSON.stringify(value));
  }

  function connect() {
    const scheme = location.protocol === "https:" ? "wss" : "ws";
    const ws = new WebSocket(`${scheme}://${location.host}/ws`);
    state.ws = ws;
    ws.onopen = () => {
      state.retry = 0;
      connection.textContent = "已连接";
      document.querySelector("#status-dot").classList.add("online");
      send({ type: "auth", deviceToken: `mac-${token || "local"}`, deviceName: "Mac" });
    };
    ws.onclose = () => {
      connection.textContent = "重新连接中";
      document.querySelector("#status-dot").classList.remove("online");
      const delay = Math.min(1000 * (2 ** state.retry++), 15000);
      setTimeout(connect, delay);
    };
    ws.onmessage = event => {
      const msg = JSON.parse(event.data);
      if (msg.type === "session_created") state.sessionId = msg.sessionId;
      if (msg.type === "token") append("assistant", msg.content);
      if (msg.type === "error") append("error", `${msg.code}: ${msg.message}`);
    };
  }

  document.querySelector("#new-session").addEventListener("click", () => send({ type: "control", action: "create_session", name: "Mac 会话" }));
  document.querySelector("#composer").addEventListener("submit", event => {
    event.preventDefault();
    const input = document.querySelector("#prompt");
    const content = input.value.trim();
    if (!content) return;
    append("user", content);
    send({ type: "text", content });
    input.value = "";
  });
  document.querySelector("#prompt").addEventListener("keydown", event => {
    if (event.metaKey && event.key === "Enter") document.querySelector("#composer").requestSubmit();
  });
  connect();
})();
