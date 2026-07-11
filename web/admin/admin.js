(() => {
  const chat = document.querySelector("#chat-view");
  const admin = document.querySelector("#admin-view");
  const title = document.querySelector("#view-title");
  async function refresh() {
    const token = window.claudePhone.adminToken;
    const response = await fetch("/admin/status", { headers: { Authorization: `Bearer ${token}` } });
    if (!response.ok) throw new Error(`admin status ${response.status}`);
    const { agent } = await response.json();
    document.querySelector("#metrics").innerHTML = [
      ["在线设备", agent.connectedDevices?.length || 0], ["活跃会话", agent.sessions?.length || 0],
      ["Agent", agent.agentVersion], ["Claude", agent.claudeVersion]
    ].map(([label, value]) => `<article><span>${label}</span><strong>${value}</strong></article>`).join("");
    document.querySelector("#admin-sessions").textContent = agent.sessions?.length ? JSON.stringify(agent.sessions, null, 2) : "暂无活跃会话";
  }
  document.querySelector("#show-admin").addEventListener("click", async () => {
    chat.hidden = true; admin.hidden = false; title.textContent = "管理与诊断";
    try { await refresh(); } catch (error) { document.querySelector("#admin-sessions").textContent = error.message; }
  });
})();
