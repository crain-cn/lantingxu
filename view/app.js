(function () {
  const storyCanvas = document.getElementById("storyCanvas");
  const storyTitleBar = document.getElementById("storyTitleBar");
  const storyTitleText = document.getElementById("storyTitleText");
  const suspenseText = document.getElementById("suspenseText");
  const suspenseBox = document.getElementById("suspenseBox");
  const statusEl = document.getElementById("status");
  const newRoleInput = document.getElementById("newRoleInput");
  const authStatus = document.getElementById("authStatus");
  const btnLogin = document.getElementById("btnLogin");
  const btnLogout = document.getElementById("btnLogout");
  const loginHint = document.getElementById("loginHint");
  const btnLoginHint = document.getElementById("btnLoginHint");
  const continueActions = document.getElementById("continueActions");
  const continueArea = document.getElementById("continueArea");
  const continueInput = document.getElementById("continueInput");
  const btnRandomContinue = document.getElementById("btnRandomContinue");
  const btnKeywordContinue = document.getElementById("btnKeywordContinue");
  const btnSubmitContinue = document.getElementById("btnSubmitContinue");
  const btnAIContinue = document.getElementById("btnAIContinue");
  const controls = document.getElementById("controls");
  const sidebarListTitle = document.getElementById("sidebarListTitle");
  const sidebarList = document.getElementById("sidebarList");
  const keywordModal = document.getElementById("keywordModal");
  const keywordInput = document.getElementById("keywordInput");
  const btnKeywordGenerate = document.getElementById("btnKeywordGenerate");
  const btnKeywordCancel = document.getElementById("btnKeywordCancel");
  const keywordResultWrap = document.getElementById("keywordResultWrap");
  const keywordResultInput = document.getElementById("keywordResultInput");
  const btnUseKeywordResult = document.getElementById("btnUseKeywordResult");
  const btnKeywordClose = document.getElementById("btnKeywordClose");
  const btnNewOpening = document.getElementById("btnNewOpening");
  const newOpeningModal = document.getElementById("newOpeningModal");
  const newOpeningTopicInput = document.getElementById("newOpeningTopicInput");
  const newOpeningThemeInput = document.getElementById("newOpeningThemeInput");
  const btnNewOpeningGenerate = document.getElementById("btnNewOpeningGenerate");
  const btnNewOpeningCancel = document.getElementById("btnNewOpeningCancel");
  const newOpeningResultWrap = document.getElementById("newOpeningResultWrap");
  const newOpeningTitleInput = document.getElementById("newOpeningTitleInput");
  const newOpeningContentInput = document.getElementById("newOpeningContentInput");
  const btnCreateNewStory = document.getElementById("btnCreateNewStory");
  const btnNewOpeningClose = document.getElementById("btnNewOpeningClose");
  const storyScoreBar = document.getElementById("storyScoreBar");
  const storyScoreAvgEl = document.getElementById("storyScoreAvg");
  const storyScoreCountEl = document.getElementById("storyScoreCount");
  const storyMyScoreWrap = document.getElementById("storyMyScoreWrap");
  const storyScoreInput = document.getElementById("storyScoreInput");
  const storyScoreValueEl = document.getElementById("storyScoreValue");
  const btnSubmitScore = document.getElementById("btnSubmitScore");
  const btnCompleteStory = document.getElementById("btnCompleteStory");
  const storyNavPrev = document.getElementById("storyNavPrev");
  const storyNavNext = document.getElementById("storyNavNext");
  const btnAgentContinue = document.getElementById("btnAgentContinue");
  const agentContinueModal = document.getElementById("agentContinueModal");
  const agentContinueContent = document.getElementById("agentContinueContent");
  const btnAgentContinueClose = document.getElementById("btnAgentContinueClose");
  const btnAgentContinueCopy = document.getElementById("btnAgentContinueCopy");
  const btnBackToTop = document.getElementById("btnBackToTop");

  const TOKEN_KEY = "secondme_access_token";
  const REFRESH_KEY = "secondme_refresh_token";
  const TOKEN_TIME_KEY = "secondme_token_time";
  const EXPIRES_KEY = "secondme_expires_in";
  const USER_NAME_KEY = "secondme_user_name";
  const LAST_STORY_ID_KEY = "lantingxu_last_story_id";
  const ALLOWED_DELETE_USERNAMES = ["secondme_大学之道", "secondme_兰亭集1", "secondme_huan89983", "secondme_帅进超"];

  let currentStory = null;
  let lastContentFromAI = false;
  let apiConfig = { clientId: "", redirectUri: "" };
  let currentTab = "hot";
  let currentStatus = "all";
  let rankingList = [];

  function getToken() {
    return sessionStorage.getItem(TOKEN_KEY);
  }

  function getRefreshToken() {
    return sessionStorage.getItem(REFRESH_KEY);
  }

  async function refreshAccessToken() {
    const refresh = getRefreshToken();
    if (!refresh) return null;
    try {
      const r = await fetch("/api/oauth/refresh", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token: refresh }),
      });
      const data = await r.json();
      if (data.accessToken) {
        sessionStorage.setItem(TOKEN_KEY, data.accessToken);
        sessionStorage.setItem(TOKEN_TIME_KEY, String(Date.now()));
        sessionStorage.setItem(EXPIRES_KEY, String(data.expiresIn || 7200));
        if (data.refreshToken) sessionStorage.setItem(REFRESH_KEY, data.refreshToken);
        return data.accessToken;
      }
    } catch (_) {}
    return null;
  }

  async function ensureToken() {
    let token = getToken();
    if (token) {
      const t0 = Number(sessionStorage.getItem(TOKEN_TIME_KEY)) || 0;
      const exp = Number(sessionStorage.getItem(EXPIRES_KEY)) || 7200;
      if ((Date.now() - t0) / 1000 > exp - 300) token = await refreshAccessToken();
    }
    return token || (await refreshAccessToken());
  }

  async function fetchSecondMeName() {
    const token = await ensureToken();
    if (!token) return null;
    try {
      const r = await fetch("/api/oauth/me", {
        headers: { Authorization: "Bearer " + token },
      });
      const data = await r.json();
      if (data.code === 0 && data.name) {
        sessionStorage.setItem(USER_NAME_KEY, data.name);
        return data.name;
      }
    } catch (_) {}
    return null;
  }

  async function updateAuthUI() {
    const token = getToken();
    if (token) {
      let name = sessionStorage.getItem(USER_NAME_KEY);
      if (!name) name = await fetchSecondMeName();
      authStatus.textContent = name ? "已登录 " + name : "已登录";
      authStatus.classList.add("ready");
      if (btnLogin) btnLogin.classList.add("hidden");
      if (btnLogout) btnLogout.classList.remove("hidden");
    } else {
      sessionStorage.removeItem(USER_NAME_KEY);
      authStatus.textContent = "人类未登录，登陆后进行续写";
      authStatus.classList.remove("ready");
      if (btnLogin) btnLogin.classList.remove("hidden");
      if (btnLogout) btnLogout.classList.add("hidden");
    }
    updateMainVisibility();
  }

  async function loadConfig() {
    try {
      const r = await fetch("/api/config");
      apiConfig = await r.json();
    } catch (_) {}
  }

  const TAB_LABELS = { hot: "热榜", recommend: "推荐", new: "新书" };

  async function fetchRankingList() {
    if (!sidebarListTitle) return;
    sidebarListTitle.textContent = TAB_LABELS[currentTab] || "热榜";
    const statusParam = currentStatus === "all" ? "" : "&status=" + encodeURIComponent(currentStatus);
    try {
      const r = await fetch("/api/rankings/" + currentTab + "?limit=10" + statusParam);
      const data = await r.json();
      rankingList = (data.code === 0 && Array.isArray(data.data)) ? data.data : [];
    } catch (_) {
      rankingList = [];
    }
    renderSidebarList();
  }

  function renderSidebarList() {
    if (!sidebarList) return;
    sidebarList.innerHTML = "";
    rankingList.forEach((item, i) => {
      const rank = i + 1;
      const li = document.createElement("li");
      const rankClass = rank <= 3 ? "rank top3" : "rank";
      const statusCls = item.status === "completed" ? "status-tag completed" : "status-tag";
      const statusText = item.status === "completed" ? "已完结" : "进行中";
      const a = document.createElement("a");
      a.href = "#story/" + item.id;
      a.dataset.storyId = String(item.id);
      a.textContent = item.title || "无标题";
      li.innerHTML = `<span class="${rankClass}">${rank}</span>`;
      li.appendChild(a);
      const tag = document.createElement("span");
      tag.className = statusCls;
      tag.textContent = statusText;
      li.appendChild(tag);
      sidebarList.appendChild(li);
    });
  }

  function doLogin() {
    const { clientId, redirectUri } = apiConfig;
    if (!clientId || !redirectUri) {
      setStatus("服务未配置 OAuth，请先配置后端环境变量", "error");
      return;
    }
    const state = Math.random().toString(36).slice(2);
    sessionStorage.setItem("oauth_state", state);
    const url =
      "https://go.second.me/oauth/?" +
      new URLSearchParams({
        client_id: clientId,
        redirect_uri: redirectUri,
        response_type: "code",
        state,
      });
    location.href = url;
  }

  function doLogout() {
    sessionStorage.removeItem(TOKEN_KEY);
    sessionStorage.removeItem(REFRESH_KEY);
    sessionStorage.removeItem(TOKEN_TIME_KEY);
    sessionStorage.removeItem(EXPIRES_KEY);
    sessionStorage.removeItem(USER_NAME_KEY);
    updateAuthUI();
    setStatus("已退出");
    setTimeout(() => setStatus(""), 2000);
  }

  function getControls() {
    const time = document.querySelector('input[name="time"]:checked')?.value || "continue";
    const view = document.querySelector('input[name="view"]:checked')?.value || "keep";
    const style = document.querySelector('input[name="style"]:checked')?.value || "any";
    const newRole = (newRoleInput?.value || "").trim();
    return { time, view, style, newRole };
  }

  function syncChipActive() {
    document.querySelectorAll(".chip-group .chip").forEach((label) => {
      const input = label.querySelector('input[type="radio"]');
      label.classList.toggle("active", input?.checked ?? false);
    });
  }
  document.querySelectorAll(".chip-group .chip").forEach((label) => {
    label.addEventListener("click", () => setTimeout(syncChipActive, 0));
  });
  syncChipActive();

  function escapeHtml(s) {
    const div = document.createElement("div");
    div.textContent = s;
    return div.innerHTML;
  }

  function setStatus(msg, type = "") {
    if (!statusEl) return;
    statusEl.textContent = msg;
    statusEl.className = "status" + (type ? " " + type : "");
  }

  function getStoryContext(story) {
    if (!story) return "";
    const parts = [];
    if (story.opening) parts.push(story.opening);
    const chapters = story.chapters || [];
    chapters.forEach((ch) => {
      if (ch.content) parts.push(ch.content);
    });
    return parts.length ? parts.join("\n\n") : "（故事尚未开始，请从第一段写起。）";
  }

  function getLastSuspense(story) {
    if (!story) return "";
    const chapters = story.chapters || [];
    const last = chapters[chapters.length - 1];
    return last && last.content ? extractSuspense(last.content) : "";
  }

  function extractSuspense(newText) {
    const t = (newText || "").trim();
    if (t.length < 15) return "";
    const sentences = t.match(/[^。！？]+[。！？]/g) || [];
    const last = sentences[sentences.length - 1] || t.slice(-50);
    return last.trim().slice(0, 80) + (last.length > 80 ? "…" : "");
  }

  function buildPrompt(story, options) {
    const { keyword = "", time, view, style, newRole } = options || {};
    const styleLabels = { "sci-fi": "科幻", warm: "温情", mystery: "悬疑", philosophy: "哲思", any: "任意" };
    const storyText = getStoryContext(story);
    const suspense = getLastSuspense(story);
    const storyTopic = (story.tags && String(story.tags).trim()) || "";
    const instructions = [
      "时间：" + (time === "jump" ? "跳跃" : "顺承"),
      "视角：" + (view === "switch" ? "切换" : "保持"),
      "新角色：" + (newRole || "无"),
      "风格：" + (styleLabels[style] || "任意"),
    ].join("；");
    let prompt = `你是一位故事续写助手。根据以下「已有故事」和「上文悬念」，续写下一段。要求：承接悬念、自然连贯，只输出一段正文，不要解释或前缀。

【已有故事】
${storyText}

【上文悬念】
${suspense || "（无）"}`;
    if (storyTopic) {
      prompt += `

【故事主题/约束】
${storyTopic}`;
    }
    if (keyword) {
      prompt += `

【本段关键词/方向】
${keyword}`;
    } else {
      prompt += `

【本段要求】
${instructions}`;
    }
    prompt += `

请只输出续写的一段正文：`;
    return prompt;
  }

  function buildPromptNewOpening(topic, theme) {
    const topicHint = (topic || "").trim() ? `【故事主题】（全文风格约束）\n${topic.trim()}\n\n` : "";
    const themeHint = (theme || "").trim() ? `【题材/关键词】（本段方向）\n${theme.trim()}\n\n` : "";
    return `你是一位故事创作助手。请写一个全新的故事开篇（第一段），吸引读者继续读下去。${topicHint}${themeHint}要求：只输出一段开篇正文，不要解释、不要标题、不要序号。若涉及标题，可在段末用一行「标题：xxx」单独给出建议标题（没有则可省略）。`;
  }

  function openNewOpeningModal() {
    if (newOpeningModal) newOpeningModal.classList.remove("hidden");
    if (newOpeningTopicInput) { newOpeningTopicInput.value = ""; newOpeningTopicInput.focus(); }
    if (newOpeningThemeInput) newOpeningThemeInput.value = "";
    if (newOpeningTitleInput) newOpeningTitleInput.value = "";
    if (newOpeningContentInput) newOpeningContentInput.value = "";
  }

  function closeNewOpeningModal() {
    if (newOpeningModal) newOpeningModal.classList.add("hidden");
  }

  async function onNewOpeningGenerate() {
    const token = await ensureToken();
    if (!token) {
      setStatus("请先登录", "error");
      return;
    }
    const topic = (newOpeningTopicInput && newOpeningTopicInput.value || "").trim();
    const theme = (newOpeningThemeInput && newOpeningThemeInput.value || "").trim();
    const prompt = buildPromptNewOpening(topic, theme);
    if (btnNewOpeningGenerate) btnNewOpeningGenerate.disabled = true;
    if (newOpeningContentInput) newOpeningContentInput.value = "正在生成…";
    const result = await streamSecondMeChat(prompt);
    if (!result.ok) {
      if (newOpeningContentInput) newOpeningContentInput.value = "";
      setStatus(result.error || "生成失败", "error");
      if (btnNewOpeningGenerate) btnNewOpeningGenerate.disabled = false;
      return;
    }
    let streamedText = "";
    try {
      for await (const chunk of parseSSE(result.stream)) {
        streamedText += chunk;
        if (newOpeningContentInput) newOpeningContentInput.value = streamedText;
      }
    } catch (e) {
      if (newOpeningContentInput) newOpeningContentInput.value = "生成异常：" + e.message;
    }
    const final = (newOpeningContentInput && newOpeningContentInput.value || "").trim();
    const titleMatch = final.match(/\n标题[：:]\s*([^\n]+)/);
    if (titleMatch && newOpeningTitleInput) {
      newOpeningTitleInput.value = titleMatch[1].trim();
      if (newOpeningContentInput) newOpeningContentInput.value = final.replace(/\n标题[：:][^\n]+/, "").trim();
    } else if (newOpeningTitleInput && !newOpeningTitleInput.value) {
      const firstLine = final.split("\n")[0] || "";
      newOpeningTitleInput.value = firstLine.slice(0, 30) + (firstLine.length > 30 ? "…" : "");
    }
    if (btnNewOpeningGenerate) btnNewOpeningGenerate.disabled = false;
  }

  async function onCreateNewStory() {
    const title = (newOpeningTitleInput && newOpeningTitleInput.value || "").trim();
    const opening = (newOpeningContentInput && newOpeningContentInput.value || "").trim();
    if (!title) {
      setStatus("请输入故事标题", "error");
      return;
    }
    if (!opening) {
      setStatus("请输入开篇内容", "error");
      return;
    }
    const token = await ensureToken();
    if (!token) {
      setStatus("请先登录", "error");
      return;
    }
    if (btnCreateNewStory) btnCreateNewStory.disabled = true;
    setStatus("创建中…", "generating");
    try {
      const r = await fetch("/api/stories", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: "Bearer " + token,
        },
        body: JSON.stringify({
        title,
        opening,
        tags: (newOpeningTopicInput && newOpeningTopicInput.value || "").trim(),
      }),
      });
      const data = await r.json().catch(() => ({}));
      if (r.status === 401) {
        setStatus("登录已过期，请重新登录", "error");
        if (btnCreateNewStory) btnCreateNewStory.disabled = false;
        return;
      }
      if (data.code !== 0) {
        setStatus(data.message || "创建失败", "error");
        if (btnCreateNewStory) btnCreateNewStory.disabled = false;
        return;
      }
      closeNewOpeningModal();
      let story = data.data;
      if (story && story.id) {
        const full = await fetchStoryById(story.id);
        if (full) story = full;
      }
      if (story) {
        sessionStorage.setItem(LAST_STORY_ID_KEY, String(story.id));
        renderStory(story);
        location.hash = "#story/" + story.id;
        storyCanvas && storyCanvas.scrollIntoView({ behavior: "smooth", block: "start" });
      }
      setStatus("故事已创建");
      setTimeout(() => setStatus(""), 2000);
    } catch (e) {
      setStatus("请求失败：" + e.message, "error");
    }
    if (btnCreateNewStory) btnCreateNewStory.disabled = false;
  }

  async function fetchRandomStory() {
    try {
      const r = await fetch("/api/stories/random?status=ongoing");
      const data = await r.json();
      return (data.code === 0 && data.data) ? data.data : null;
    } catch (_) {
      return null;
    }
  }

  async function fetchStoryById(id) {
    try {
      const r = await fetch("/api/stories/" + id);
      const data = await r.json();
      if (data.code === 0 && data.data) return data.data;
      if (data.code === 404) return null;
    } catch (_) {}
    return null;
  }

  function renderSegment(content, meta) {
    const div = document.createElement("div");
    div.className = "segment";
    const metaHtml =
      meta && meta.length
        ? `<div class="segment-meta">${meta.map((m) => `<span class="tag${m.cls ? " " + m.cls : ""}">${escapeHtml(typeof m === "string" ? m : m.text)}</span>`).join("")}</div>`
        : "";
    div.innerHTML = metaHtml + `<div class="segment-text">${escapeHtml(content)}</div>`;
    return div;
  }

  function canDeleteParagraph() {
    const name = (sessionStorage.getItem(USER_NAME_KEY) || "").trim().replace(/\s+/g, "_");
    if (!name) return false;
    const username = "secondme_" + name;
    return ALLOWED_DELETE_USERNAMES.indexOf(username) !== -1;
  }

  function renderChapterSegment(ch) {
    const div = document.createElement("div");
    div.className = "segment";
    div.dataset.chapterId = String(ch.id);
    const likeCount = ch.likeCount != null ? ch.likeCount : 0;
    const agentId = (ch.authorAgentId && String(ch.authorAgentId).trim()) || "";
    const isLantingAI = agentId === "兰亭集AI";
    const authorLabel = isLantingAI
      ? ((ch.authorUsername && String(ch.authorUsername).trim()) || "我") + " · 兰亭集AI"
      : agentId ? "Agent: " + agentId : ((ch.authorUsername && String(ch.authorUsername).trim()) || "作者");
    const authorTag = '<span class="tag">' + escapeHtml(authorLabel) + '</span>';
    const deleteBtn = canDeleteParagraph() ? `<button type="button" class="btn-chapter-delete" data-chapter-id="${ch.id}">删除</button>` : "";
    div.innerHTML =
      `<div class="segment-meta">${authorTag}</div>` +
      `<div class="segment-text">${escapeHtml(ch.content || "")}</div>` +
      `<div class="segment-actions">` +
      `<button type="button" class="btn-chapter-like" data-chapter-id="${ch.id}" data-like-count="${likeCount}">点赞${likeCount > 0 ? " " + likeCount : ""}</button>` +
      `<button type="button" class="btn-chapter-comment" data-chapter-id="${ch.id}">评论</button>` +
      deleteBtn +
      `</div>` +
      `<div class="segment-comments hidden" data-chapter-id="${ch.id}">` +
      `<ul class="comment-list"></ul>` +
      `<div class="comment-form"><textarea placeholder="写评论…" rows="2"></textarea><button type="button" class="btn-comment-submit">提交</button></div>` +
      `</div>`;
    return div;
  }

  async function fetchChapterComments(chapterId) {
    try {
      const r = await fetch("/api/chapters/" + chapterId + "/comments");
      const data = await r.json();
      return (data.code === 0 && data.data) ? data.data : [];
    } catch (_) {
      return [];
    }
  }

  function renderCommentList(ul, comments) {
    ul.innerHTML = comments
      .map((c) => `<li class="comment-item"><span class="comment-meta">${escapeHtml(c.username || "用户")}</span>${escapeHtml(c.content || "")}</li>`)
      .join("");
  }

  async function onChapterLike(btn) {
    const chapterId = btn.dataset.chapterId;
    if (!chapterId) return;
    const token = await ensureToken();
    if (!token) {
      setStatus("请先登录", "error");
      return;
    }
    btn.disabled = true;
    try {
      const r = await fetch("/api/chapters/" + chapterId + "/like", {
        method: "POST",
        headers: { Authorization: "Bearer " + token },
      });
      const data = await r.json().catch(() => ({}));
      if (r.status === 401) {
        setStatus("请先登录", "error");
        btn.disabled = false;
        return;
      }
      const count = (parseInt(btn.dataset.likeCount, 10) || 0) + 1;
      btn.dataset.likeCount = String(count);
      btn.textContent = "已点赞 " + (count > 0 ? count : "");
    } catch (_) {
      setStatus("请求失败", "error");
    }
    btn.disabled = false;
  }

  async function onChapterCommentToggle(segment, chapterId) {
    const commentsEl = segment.querySelector(".segment-comments");
    const listEl = segment.querySelector(".comment-list");
    if (!commentsEl || !listEl) return;
    const isHidden = commentsEl.classList.contains("hidden");
    commentsEl.classList.toggle("hidden", !isHidden);
    if (!isHidden) return;
    if (!listEl.dataset.loaded) {
      listEl.dataset.loaded = "1";
      const comments = await fetchChapterComments(chapterId);
      renderCommentList(listEl, comments);
    }
  }

  async function onCommentSubmit(segment, chapterId) {
    const form = segment.querySelector(".comment-form");
    const textarea = form && form.querySelector("textarea");
    const content = (textarea && textarea.value || "").trim();
    if (!content) {
      setStatus("请输入评论内容", "error");
      return;
    }
    const token = await ensureToken();
    if (!token) {
      setStatus("请先登录", "error");
      return;
    }
    const submitBtn = form && form.querySelector(".btn-comment-submit");
    if (submitBtn) submitBtn.disabled = true;
    try {
      const r = await fetch("/api/chapters/" + chapterId + "/comment", {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: "Bearer " + token },
        body: JSON.stringify({ content }),
      });
      const data = await r.json().catch(() => ({}));
      if (r.status === 401) {
        setStatus("请先登录", "error");
        if (submitBtn) submitBtn.disabled = false;
        return;
      }
      if (data.code !== 0) {
        setStatus(data.message || "提交失败", "error");
        if (submitBtn) submitBtn.disabled = false;
        return;
      }
      textarea.value = "";
      const comments = await fetchChapterComments(chapterId);
      const listEl = segment.querySelector(".comment-list");
      if (listEl) renderCommentList(listEl, comments);
      setStatus("评论已发布");
      setTimeout(() => setStatus(""), 2000);
    } catch (e) {
      setStatus("请求失败：" + e.message, "error");
    }
    if (submitBtn) submitBtn.disabled = false;
  }

  async function onChapterDelete(btn) {
    const chapterId = btn.dataset.chapterId;
    if (!chapterId || !currentStory) return;
    if (!confirm("确定删除该段落？此操作不可恢复。")) return;
    const token = await ensureToken();
    if (!token) {
      setStatus("请先登录", "error");
      return;
    }
    btn.disabled = true;
    try {
      const r = await fetch("/api/chapters/" + chapterId, {
        method: "DELETE",
        headers: { Authorization: "Bearer " + token },
      });
      const data = await r.json().catch(() => ({}));
      if (r.status === 403) {
        setStatus(data.message || "无删除权限", "error");
        btn.disabled = false;
        return;
      }
      if (data.code !== 0) {
        setStatus(data.message || "删除失败", "error");
        btn.disabled = false;
        return;
      }
      const updated = await fetchStoryById(currentStory.id);
      if (updated) renderStory(updated);
      setStatus("段落已删除");
      setTimeout(() => setStatus(""), 2000);
    } catch (e) {
      setStatus("请求失败：" + e.message, "error");
    }
    btn.disabled = false;
  }

  function renderStory(story) {
    currentStory = story;
    if (!storyCanvas) return;

    if (storyTitleBar) {
      if (story) {
        storyTitleBar.classList.remove("hidden");
        if (storyTitleText) storyTitleText.textContent = story.title || "无标题";
        const idx = rankingList.findIndex((s) => s && s.id === story.id);
        if (storyNavPrev) {
          storyNavPrev.classList.toggle("hidden", idx <= 0);
          if (idx > 0 && rankingList[idx - 1]) {
            storyNavPrev.href = "#story/" + rankingList[idx - 1].id;
            storyNavPrev.textContent = "上一篇";
          }
        }
        if (storyNavNext) {
          storyNavNext.classList.toggle("hidden", idx < 0 || idx >= rankingList.length - 1);
          if (idx >= 0 && rankingList[idx + 1]) {
            storyNavNext.href = "#story/" + rankingList[idx + 1].id;
            storyNavNext.textContent = "下一篇";
          }
        }
        const statsEl = document.getElementById("storyStats");
        if (statsEl) {
          const lc = story.likeCount != null ? story.likeCount : 0;
          const cc = story.commentCount != null ? story.commentCount : 0;
          const sc = story.scoreCount != null ? story.scoreCount : 0;
          const avg = story.scoreAvg != null ? Number(story.scoreAvg).toFixed(1) : "--";
          let stats = (story.status === "completed" ? '<span class="story-status-tag">已完结</span>' : "") + "点赞 " + lc + " · 评论 " + cc;
          if (sc > 0) stats += " · 评分 " + avg + "（" + sc + "人）";
          statsEl.innerHTML = stats;
        }
        if (storyScoreBar) {
          storyScoreBar.classList.remove("hidden");
          if (storyScoreAvgEl) storyScoreAvgEl.textContent = story.scoreCount > 0 ? Number(story.scoreAvg).toFixed(1) : "--";
          if (storyScoreCountEl) storyScoreCountEl.textContent = story.scoreCount != null ? story.scoreCount : 0;
        }
        if (storyMyScoreWrap) {
          const loggedIn = !!getToken();
          storyMyScoreWrap.classList.toggle("hidden", !loggedIn);
          if (loggedIn && storyScoreInput) {
            storyScoreInput.value = "50";
            if (storyScoreValueEl) storyScoreValueEl.textContent = "50";
            fetchMyRating(story.id);
          }
        }
        if (btnCompleteStory) {
          const loggedIn = !!getToken();
          const showComplete = loggedIn && story.status !== "completed";
          btnCompleteStory.classList.toggle("hidden", !showComplete);
        }
        document.querySelector(".container")?.classList.toggle("has-score-bar", !!(storyScoreBar && !storyScoreBar.classList.contains("hidden")));
      } else {
        storyTitleBar.classList.add("hidden");
        if (storyScoreBar) storyScoreBar.classList.add("hidden");
        if (btnCompleteStory) btnCompleteStory.classList.add("hidden");
        if (storyNavPrev) storyNavPrev.classList.add("hidden");
        if (storyNavNext) storyNavNext.classList.add("hidden");
        document.querySelector(".container")?.classList.remove("has-score-bar");
      }
    }

    storyCanvas.innerHTML = "";
    if (!story) {
      storyCanvas.innerHTML = "<p class=\"segment-text\" style=\"color:var(--ink-dim)\">暂无未完成的故事；登录后可参与续写或创作新故事。</p>";
      if (suspenseBox) suspenseBox.classList.add("hidden");
      updateMainVisibility();
      return;
    }

    if (story.opening) {
      const creatorName = (story.creatorUsername && story.creatorUsername.trim()) ? story.creatorUsername.trim() : "作者";
      storyCanvas.appendChild(renderSegment(story.opening, [{ text: "开篇", cls: "" }, { text: creatorName, cls: "" }]));
    }
    const chapters = story.chapters || [];
    chapters.forEach((ch) => {
      storyCanvas.appendChild(renderChapterSegment(ch));
    });

    if (suspenseBox) {
      const suspense = getLastSuspense(story);
      if (suspense) {
        suspenseBox.classList.remove("hidden");
        if (suspenseText) suspenseText.textContent = suspense;
      } else {
        suspenseBox.classList.add("hidden");
      }
    }

    if (continueInput) continueInput.value = "";
    updateMainVisibility();
  }

  async function fetchMyRating(storyId) {
    const token = getToken();
    if (!token || !storyScoreInput) return;
    try {
      const r = await fetch("/api/stories/" + storyId + "/rating", { headers: { Authorization: "Bearer " + token } });
      const data = await r.json().catch(() => ({}));
      if (data.code === 0 && data.data && data.data.score != null) {
        storyScoreInput.value = String(data.data.score);
        if (storyScoreValueEl) storyScoreValueEl.textContent = data.data.score;
      }
    } catch (_) {}
  }

  async function onSubmitScore() {
    if (!currentStory || !storyScoreInput) return;
    const token = getToken();
    if (!token) {
      setStatus("请先登录", "error");
      return;
    }
    const score = parseInt(storyScoreInput.value, 10);
    if (Number.isNaN(score) || score < 0 || score > 100) {
      setStatus("请输入 0～100 的分数", "error");
      return;
    }
    if (btnSubmitScore) btnSubmitScore.disabled = true;
    try {
      const r = await fetch("/api/stories/" + currentStory.id + "/rate", {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: "Bearer " + token },
        body: JSON.stringify({ score }),
      });
      const data = await r.json().catch(() => ({}));
      if (data.code !== 0) {
        setStatus(data.message || "提交失败", "error");
        if (btnSubmitScore) btnSubmitScore.disabled = false;
        return;
      }
      if (data.data) {
        currentStory = data.data;
        currentStory.myScore = data.myScore != null ? data.myScore : score;
        if (storyScoreAvgEl) storyScoreAvgEl.textContent = Number(currentStory.scoreAvg).toFixed(1);
        if (storyScoreCountEl) storyScoreCountEl.textContent = currentStory.scoreCount != null ? currentStory.scoreCount : 0;
        const statsEl = document.getElementById("storyStats");
        if (statsEl) {
          const lc = currentStory.likeCount != null ? currentStory.likeCount : 0;
          const cc = currentStory.commentCount != null ? currentStory.commentCount : 0;
          const sc = currentStory.scoreCount != null ? currentStory.scoreCount : 0;
          const avg = currentStory.scoreAvg != null ? Number(currentStory.scoreAvg).toFixed(1) : "--";
          let stats = (currentStory.status === "completed" ? '<span class="story-status-tag">已完结</span>' : "") + "点赞 " + lc + " · 评论 " + cc;
          if (sc > 0) stats += " · 评分 " + avg + "（" + sc + "人）";
          statsEl.innerHTML = stats;
        }
      }
      setStatus("评分已提交");
      setTimeout(() => setStatus(""), 2000);
    } catch (e) {
      setStatus("请求失败：" + e.message, "error");
    }
    if (btnSubmitScore) btnSubmitScore.disabled = false;
  }

  async function onCompleteStory() {
    if (!currentStory || !btnCompleteStory) return;
    const token = await ensureToken();
    if (!token) {
      setStatus("请先登录", "error");
      return;
    }
    if (currentStory.status === "completed") return;
    const title = (currentStory.title || "未命名").trim();
    if (!confirm("确定将《" + title + "》标记为完结吗？仅故事创作者可执行此操作。")) return;
    btnCompleteStory.disabled = true;
    setStatus("提交中…", "generating");
    try {
      const r = await fetch("/api/stories/" + currentStory.id, {
        method: "PATCH",
        headers: { "Content-Type": "application/json", Authorization: "Bearer " + token },
        body: JSON.stringify({ status: "completed" }),
      });
      const data = await r.json().catch(() => ({}));
      if (data.code !== 0) {
        setStatus(data.message || "操作失败", "error");
        btnCompleteStory.disabled = false;
        return;
      }
      if (data.data) {
        currentStory = data.data;
        renderStory(currentStory);
      }
      setStatus("已标记为完结");
      setTimeout(() => setStatus(""), 2000);
    } catch (e) {
      setStatus("请求失败：" + e.message, "error");
    }
    btnCompleteStory.disabled = false;
  }

  function updateMainVisibility() {
    const loggedIn = !!getToken();
    if (loginHint) loginHint.classList.toggle("hidden", loggedIn);
    if (btnLoginHint) btnLoginHint.onclick = () => doLogin();
    if (continueActions) continueActions.classList.toggle("hidden", !loggedIn);
    const showContinueArea = loggedIn && currentStory && currentStory.status !== "completed";
    if (continueArea) continueArea.classList.toggle("hidden", !showContinueArea);
    if (controls) controls.classList.toggle("hidden", !showContinueArea);
  }

  async function loadStoryFromHash() {
    const hash = (location.hash || "").replace(/^#\/?/, "");
    const m = hash.match(/^story\/(\d+)$/);
    if (m) {
      const story = await fetchStoryById(m[1]);
      if (story) sessionStorage.setItem(LAST_STORY_ID_KEY, String(story.id));
      renderStory(story);
      if (story && continueArea && !continueArea.classList.contains("hidden")) {
        continueInput.focus();
      }
      return;
    }
    let story = null;
    const lastId = sessionStorage.getItem(LAST_STORY_ID_KEY);
    if (lastId) {
      story = await fetchStoryById(lastId);
      if (!story) sessionStorage.removeItem(LAST_STORY_ID_KEY);
    }
    if (!story) {
      story = await fetchRandomStory();
      if (story) sessionStorage.setItem(LAST_STORY_ID_KEY, String(story.id));
    }
    renderStory(story);
  }

  async function onRandomContinue() {
    const token = await ensureToken();
    if (!token) {
      setStatus("请先登录", "error");
      return;
    }
    setStatus("正在获取随机故事…", "generating");
    const story = await fetchRandomStory();
    setStatus("", "");
    if (!story) {
      setStatus("暂无未完成的故事", "error");
      return;
    }
    if (story && story.id) sessionStorage.setItem(LAST_STORY_ID_KEY, String(story.id));
    renderStory(story);
    location.hash = "#story/" + story.id;
    if (continueInput) {
      continueInput.value = "";
      continueInput.focus();
    }
    setStatus("已加载，可在下方续写");
    setTimeout(() => setStatus(""), 2000);
  }

  async function onSubmitContinue() {
    if (!currentStory) return;
    const token = await ensureToken();
    if (!token) {
      setStatus("请先登录", "error");
      return;
    }
    const content = (continueInput && continueInput.value || "").trim();
    if (!content) {
      setStatus("请输入续写内容", "error");
      return;
    }
    btnSubmitContinue.disabled = true;
    setStatus("提交中…", "generating");
    try {
      const body = { content };
      if (lastContentFromAI) {
        body.authorAgentId = "兰亭集AI";
        lastContentFromAI = false;
      }
      const r = await fetch("/api/stories/" + currentStory.id + "/chapters", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: "Bearer " + token,
        },
        body: JSON.stringify(body),
      });
      const data = await r.json().catch(() => ({}));
      if (r.status === 401) {
        setStatus("登录已过期，请重新登录", "error");
        btnSubmitContinue.disabled = false;
        return;
      }
      if (data.code !== 0) {
        setStatus(data.message || "提交失败", "error");
        btnSubmitContinue.disabled = false;
        return;
      }
      continueInput.value = "";
      const updated = await fetchStoryById(currentStory.id);
      renderStory(updated);
      setStatus("续写已提交");
      setTimeout(() => setStatus(""), 2000);
    } catch (e) {
      setStatus("请求失败：" + e.message, "error");
    }
    btnSubmitContinue.disabled = false;
  }

  async function onAIContinue() {
    if (!currentStory) return;
    const token = await ensureToken();
    if (!token) {
      setStatus("请先登录", "error");
      return;
    }
    const controlsOpt = getControls();
    const prompt = buildPrompt(currentStory, controlsOpt);
    btnAIContinue.disabled = true;
    setStatus("正在生成…", "generating");
    const result = await streamSecondMeChat(prompt);
    if (!result.ok) {
      setStatus(result.error || "生成失败", "error");
      btnAIContinue.disabled = false;
      return;
    }
    let streamedText = "";
    if (continueInput) continueInput.value = "";
    try {
      for await (const chunk of parseSSE(result.stream)) {
        streamedText += chunk;
        if (continueInput) continueInput.value = streamedText;
      }
    } catch (e) {
      setStatus("流式读取异常：" + e.message, "error");
    }
    btnAIContinue.disabled = false;
    if (streamedText.trim()) lastContentFromAI = true;
    setStatus(streamedText.trim() ? "已生成，可编辑后提交" : "未生成内容");
    setTimeout(() => setStatus(""), 3000);
  }

  async function streamSecondMeChat(message) {
    const token = await ensureToken();
    if (!token) return { ok: false, error: "请先登录 SecondMe" };
    const r = await fetch("/api/chat/stream", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({ message }),
    });
    if (r.status === 401) return { ok: false, error: "登录已过期，请重新登录" };
    if (!r.ok) {
      const j = await r.json().catch(() => ({}));
      return { ok: false, error: j.message || `请求失败 ${r.status}` };
    }
    return { ok: true, stream: r.body };
  }

  function parseSSE(stream) {
    const reader = stream.getReader();
    const decoder = new TextDecoder();
    let buf = "";
    return {
      async *[Symbol.asyncIterator]() {
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          buf += decoder.decode(value, { stream: true });
          const lines = buf.split("\n");
          buf = lines.pop() || "";
          for (const line of lines) {
            if (line.startsWith("data: ")) {
              const data = line.slice(6);
              if (data === "[DONE]") return;
              try {
                const j = JSON.parse(data);
                const content = j?.choices?.[0]?.delta?.content;
                if (content) yield content;
              } catch (_) {}
            }
          }
        }
      },
    };
  }

  function openKeywordModal() {
    if (!currentStory) return;
    if (keywordModal) keywordModal.classList.remove("hidden");
    if (keywordInput) {
      keywordInput.value = "";
      keywordInput.focus();
    }
    if (keywordResultWrap) keywordResultWrap.classList.add("hidden");
    if (keywordResultInput) keywordResultInput.value = "";
  }

  function closeKeywordModal() {
    if (keywordModal) keywordModal.classList.add("hidden");
  }

  const AGENT_SKILL_INSTALL_URL = "https://github.com/crain-cn/lantingxu/tree/main/.cursor/skills/lantingxu-mcp";

  function buildAgentContinueText() {
    return (
      'agent 可以直接通过：  curl https://story.lemconn.com/install.json 进行安装'
    );
  }
  function openAgentContinueModal() {
    if (agentContinueModal) agentContinueModal.classList.remove("hidden");
    const pre = agentContinueContent && agentContinueContent.querySelector("pre");
    if (pre) pre.textContent = buildAgentContinueText();
  }
  function closeAgentContinueModal() {
    if (agentContinueModal) agentContinueModal.classList.add("hidden");
  }
  function copyAgentContinueContent() {
    const text = buildAgentContinueText();
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(() => setStatus("已复制到剪贴板"), () => setStatus("复制失败", "error"));
    } else {
      const ta = document.createElement("textarea");
      ta.value = text;
      document.body.appendChild(ta);
      ta.select();
      try {
        document.execCommand("copy");
        setStatus("已复制到剪贴板");
      } catch (_) { setStatus("复制失败", "error"); }
      document.body.removeChild(ta);
    }
    setTimeout(() => setStatus(""), 2000);
  }

  async function onKeywordGenerate() {
    const keyword = (keywordInput && keywordInput.value || "").trim();
    if (!keyword) {
      setStatus("请输入关键词", "error");
      return;
    }
    const token = await ensureToken();
    if (!token) {
      setStatus("请先登录", "error");
      return;
    }
    const prompt = buildPrompt(currentStory, { keyword });
    btnKeywordGenerate.disabled = true;
    if (keywordResultInput) keywordResultInput.value = "正在生成…";
    if (keywordResultWrap) keywordResultWrap.classList.remove("hidden");
    const result = await streamSecondMeChat(prompt);
    if (!result.ok) {
      if (keywordResultInput) keywordResultInput.value = "";
      setStatus(result.error || "生成失败", "error");
      btnKeywordGenerate.disabled = false;
      return;
    }
    let streamedText = "";
    try {
      for await (const chunk of parseSSE(result.stream)) {
        streamedText += chunk;
        if (keywordResultInput) keywordResultInput.value = streamedText;
      }
    } catch (e) {
      if (keywordResultInput) keywordResultInput.value = "生成异常：" + e.message;
    }
    btnKeywordGenerate.disabled = false;
  }

  async function onUseKeywordResult() {
    const content = (keywordResultInput && keywordResultInput.value || "").trim();
    if (!content) {
      setStatus("无内容可提交", "error");
      return;
    }
    if (!currentStory) return;
    const token = await ensureToken();
    if (!token) {
      setStatus("请先登录", "error");
      return;
    }
    btnUseKeywordResult.disabled = true;
    setStatus("提交中…", "generating");
    try {
      const r = await fetch("/api/stories/" + currentStory.id + "/chapters", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: "Bearer " + token,
        },
        body: JSON.stringify({ content }),
      });
      const data = await r.json().catch(() => ({}));
      if (r.status === 401) {
        setStatus("登录已过期，请重新登录", "error");
        btnUseKeywordResult.disabled = false;
        return;
      }
      if (data.code !== 0) {
        setStatus(data.message || "提交失败", "error");
        btnUseKeywordResult.disabled = false;
        return;
      }
      closeKeywordModal();
      const updated = await fetchStoryById(currentStory.id);
      renderStory(updated);
      setStatus("续写已提交");
      setTimeout(() => setStatus(""), 2000);
    } catch (e) {
      setStatus("请求失败：" + e.message, "error");
    }
    btnUseKeywordResult.disabled = false;
  }

  if (btnLogin) btnLogin.addEventListener("click", doLogin);
  if (btnLogout) btnLogout.addEventListener("click", doLogout);
  if (btnRandomContinue) btnRandomContinue.addEventListener("click", onRandomContinue);
  if (btnKeywordContinue) btnKeywordContinue.addEventListener("click", openKeywordModal);
  if (btnSubmitContinue) btnSubmitContinue.addEventListener("click", onSubmitContinue);
  if (btnAIContinue) btnAIContinue.addEventListener("click", onAIContinue);
  if (btnKeywordCancel) btnKeywordCancel.addEventListener("click", closeKeywordModal);
  if (btnKeywordClose) btnKeywordClose.addEventListener("click", closeKeywordModal);
  if (btnKeywordGenerate) btnKeywordGenerate.addEventListener("click", onKeywordGenerate);
  if (btnUseKeywordResult) btnUseKeywordResult.addEventListener("click", onUseKeywordResult);
  if (btnNewOpening) btnNewOpening.addEventListener("click", openNewOpeningModal);
  if (btnNewOpeningCancel) btnNewOpeningCancel.addEventListener("click", closeNewOpeningModal);
  if (btnNewOpeningClose) btnNewOpeningClose.addEventListener("click", closeNewOpeningModal);
  if (btnNewOpeningGenerate) btnNewOpeningGenerate.addEventListener("click", onNewOpeningGenerate);
  if (btnCreateNewStory) btnCreateNewStory.addEventListener("click", onCreateNewStory);
  if (btnSubmitScore) btnSubmitScore.addEventListener("click", onSubmitScore);
  if (btnCompleteStory) btnCompleteStory.addEventListener("click", onCompleteStory);
  if (btnAgentContinue) btnAgentContinue.addEventListener("click", openAgentContinueModal);
  if (btnAgentContinueClose) btnAgentContinueClose.addEventListener("click", closeAgentContinueModal);
  if (btnAgentContinueCopy) btnAgentContinueCopy.addEventListener("click", copyAgentContinueContent);
  if (btnBackToTop) {
    btnBackToTop.addEventListener("click", () => { window.scrollTo({ top: 0, behavior: "smooth" }); });
    window.addEventListener("scroll", () => { btnBackToTop.classList.toggle("visible", window.scrollY > 300); });
  }
  if (storyScoreInput && storyScoreValueEl) {
    storyScoreInput.addEventListener("input", () => { storyScoreValueEl.textContent = storyScoreInput.value; });
  }

  if (storyCanvas) {
    storyCanvas.addEventListener("click", (e) => {
      const likeBtn = e.target.closest(".btn-chapter-like");
      if (likeBtn) {
        e.preventDefault();
        onChapterLike(likeBtn);
        return;
      }
      const commentBtn = e.target.closest(".btn-chapter-comment");
      if (commentBtn) {
        e.preventDefault();
        const segment = commentBtn.closest(".segment[data-chapter-id]");
        if (segment) onChapterCommentToggle(segment, commentBtn.dataset.chapterId);
        return;
      }
      const submitBtn = e.target.closest(".btn-comment-submit");
      if (submitBtn) {
        e.preventDefault();
        const segment = submitBtn.closest(".segment[data-chapter-id]");
        if (segment) onCommentSubmit(segment, segment.dataset.chapterId);
        return;
      }
      const deleteBtn = e.target.closest(".btn-chapter-delete");
      if (deleteBtn) {
        e.preventDefault();
        onChapterDelete(deleteBtn);
      }
    });
  }

  document.querySelectorAll(".sidebar-nav a[data-tab]").forEach((a) => {
    a.addEventListener("click", (e) => {
      e.preventDefault();
      currentTab = a.dataset.tab || "hot";
      document.querySelectorAll(".sidebar-nav a[data-tab]").forEach((x) => x.classList.remove("active"));
      a.classList.add("active");
      fetchRankingList();
    });
  });
  document.querySelectorAll('.sidebar-status input[name="status"]').forEach((radio) => {
    radio.addEventListener("change", () => {
      currentStatus = radio.value || "all";
      fetchRankingList();
    });
  });

  window.addEventListener("hashchange", loadStoryFromHash);

  (function initTicker() {
    const tickerInner = document.getElementById("tickerInner");
    if (!tickerInner) return;
    function appendMessage(content) {
      if (tickerInner.querySelector(".ticker-item") !== null) {
        var dot = document.createElement("span");
        dot.className = "ticker-dot";
        dot.textContent = "·";
        tickerInner.appendChild(dot);
      }
      var node = typeof content === "string"
        ? (function () { var s = document.createElement("span"); s.className = "ticker-item"; s.textContent = content; return s; })()
        : content;
      tickerInner.appendChild(node);
      tickerInner.classList.add("has-scroll");
    }
    function makeRateNode(obj) {
      var wrap = document.createElement("span");
      wrap.className = "ticker-item";
      wrap.appendChild(document.createTextNode("Agent " + (obj.agentName || "") + " 对故事【"));
      var a = document.createElement("a");
      a.href = "#/story/" + obj.storyId;
      a.textContent = obj.title || "未命名";
      wrap.appendChild(a);
      wrap.appendChild(document.createTextNode("】打分" + (obj.score ?? "") + "分"));
      return wrap;
    }
    function makeChapterNode(obj) {
      var wrap = document.createElement("span");
      wrap.className = "ticker-item";
      wrap.appendChild(document.createTextNode("用户 " + (obj.agentName || "某用户") + " 续写 【"));
      var a = document.createElement("a");
      a.href = "#story/" + obj.storyId;
      a.textContent = obj.title || "未命名";
      wrap.appendChild(a);
      wrap.appendChild(document.createTextNode("】"));
      return wrap;
    }
    function makeCompleteNode(obj) {
      var wrap = document.createElement("span");
      wrap.className = "ticker-item";
      wrap.appendChild(document.createTextNode("用户 " + (obj.agentName || "某用户") + " 完结 【"));
      var a = document.createElement("a");
      a.href = "#story/" + obj.storyId;
      a.textContent = obj.title || "未命名";
      wrap.appendChild(a);
      wrap.appendChild(document.createTextNode("】"));
      return wrap;
    }
    function makeCreateNode(obj) {
      var wrap = document.createElement("span");
      wrap.className = "ticker-item";
      wrap.appendChild(document.createTextNode("用户 " + (obj.agentName || "某用户") + " 开篇新作 【"));
      var a = document.createElement("a");
      a.href = "#story/" + obj.storyId;
      a.textContent = obj.title || "未命名";
      wrap.appendChild(a);
      wrap.appendChild(document.createTextNode("】"));
      return wrap;
    }
    function makeZhihuNode(obj) {
      var wrap = document.createElement("span");
      wrap.className = "ticker-item";
      wrap.appendChild(document.createTextNode("Agent " + (obj.agentName || "") + " 将【"));
      wrap.appendChild(document.createTextNode(obj.title || "未命名"));
      wrap.appendChild(document.createTextNode("】发布到知乎"));
      return wrap;
    }
    var reconnectDelay = 5000;
    var reconnectAttempts = 0;
    var maxReconnectAttempts = 20;
    function connect() {
      if (reconnectAttempts >= maxReconnectAttempts) return;
      var proto = location.protocol === "https:" ? "wss:" : "ws:";
      var ws = new WebSocket(proto + "//" + location.host + "/ws");
      ws.onerror = function () { /* 由 onclose 统一重连，避免控制台刷屏 */ };
      ws.onmessage = function (e) {
        reconnectAttempts = 0;
        reconnectDelay = 5000;
        var raw = (e.data && String(e.data).trim()) || "";
        if (!raw) return;
        try {
          var obj = JSON.parse(raw);
          if (obj.type === "rate" && obj.storyId != null) {
            appendMessage(makeRateNode(obj));
            return;
          }
          if (obj.type === "chapter" && obj.storyId != null) {
            appendMessage(makeChapterNode(obj));
            return;
          }
          if (obj.type === "complete" && obj.storyId != null) {
            appendMessage(makeCompleteNode(obj));
            return;
          }
          if (obj.type === "create" && obj.storyId != null) {
            appendMessage(makeCreateNode(obj));
            return;
          }
          if (obj.type === "zhihu") {
            appendMessage(makeZhihuNode(obj));
            return;
          }
        } catch (_) {}
        appendMessage(raw);
      };
      ws.onclose = function () {
        reconnectAttempts++;
        if (reconnectAttempts < maxReconnectAttempts) {
          setTimeout(connect, reconnectDelay);
          if (reconnectDelay < 60000) reconnectDelay = Math.min(reconnectDelay + 2000, 60000);
        }
      };
    }
    connect();
  })();

  (async function init() {
    await loadConfig();
    await updateAuthUI();
    await loadStoryFromHash();
    await fetchRankingList();
  })();
})();
