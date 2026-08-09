(function () {
  const params = new URLSearchParams(window.location.search);
  const token = params.get("token") || "";
  const state = {
    rules: [],
    selected: -1
  };

  const elements = {
    list: document.getElementById("rule-list"),
    form: document.getElementById("rule-form"),
    keyword: document.getElementById("keyword"),
    matchMode: document.getElementById("match-mode"),
    caseSensitive: document.getElementById("case-sensitive"),
    replyType: document.getElementById("reply-type"),
    contents: document.getElementById("contents"),
    status: document.getElementById("status"),
    newRule: document.getElementById("new-rule"),
    saveRule: document.getElementById("save-rule"),
    deleteRule: document.getElementById("delete-rule"),
    moveUp: document.getElementById("move-up"),
    moveDown: document.getElementById("move-down")
  };

  function apiURL(path) {
    const url = new URL(path, window.location.origin);
    url.searchParams.set("token", token);
    return url.toString();
  }

  function setStatus(message, isError) {
    elements.status.textContent = message || "";
    elements.status.classList.toggle("error", Boolean(isError));
  }

  function emptyRule() {
    return {
      keyword: "",
      match_mode: "exact",
      case_sensitive: true,
      areas: ["group"],
      reply_type: "text",
      contents: [""]
    };
  }

  function renderList() {
    elements.list.textContent = "";
    state.rules.forEach((rule, index) => {
      const item = document.createElement("button");
      item.type = "button";
      item.className = "rule-item" + (index === state.selected ? " active" : "");
      item.setAttribute("role", "option");
      item.setAttribute("aria-selected", String(index === state.selected));

      const title = document.createElement("span");
      title.className = "rule-title";
      title.textContent = rule.keyword || "未命名规则";
      const meta = document.createElement("span");
      meta.className = "rule-meta";
      meta.textContent = `${rule.match_mode || "exact"} · ${rule.reply_type || "text"} · ${(rule.areas || []).length} 个区域`;

      item.append(title, meta);
      item.addEventListener("click", () => switchRule(index));
      elements.list.appendChild(item);
    });
  }

  function selectRule(index) {
    state.selected = index;
    const rule = state.rules[index] || emptyRule();
    elements.keyword.value = rule.keyword || "";
    elements.matchMode.value = rule.match_mode || "exact";
    elements.caseSensitive.checked = Boolean(rule.case_sensitive);
    elements.replyType.value = rule.reply_type || "text";
    elements.contents.value = (rule.contents || [""]).join("\n");
    const selectedAreas = new Set(rule.areas || []);
    elements.form.querySelectorAll("input[name='areas']").forEach((input) => {
      input.checked = selectedAreas.has(input.value);
    });
    renderList();
    updateAreaAvailability();
  }

  function switchRule(index) {
    if (state.selected >= 0) {
      updateCurrentRuleFromForm();
    }
    selectRule(index);
  }

  function readFormRule() {
    const areas = Array.from(elements.form.querySelectorAll("input[name='areas']:checked")).map((input) => input.value);
    return {
      keyword: elements.keyword.value.trim(),
      match_mode: elements.matchMode.value,
      case_sensitive: elements.caseSensitive.checked,
      areas,
      reply_type: elements.replyType.value,
      contents: [elements.contents.value]
    };
  }

  function updateCurrentRuleFromForm() {
    if (state.selected < 0) {
      state.rules.push(emptyRule());
      state.selected = state.rules.length - 1;
    }
    state.rules[state.selected] = readFormRule();
  }

  function updateAreaAvailability() {
    const media = ["audio", "video", "file"].includes(elements.replyType.value);
    elements.form.querySelectorAll("input[name='areas']").forEach((input) => {
      if (input.value === "channel" || input.value === "channel_private") {
        input.disabled = media;
        if (media) {
          input.checked = false;
        }
      }
    });
  }

  async function loadRules() {
    const response = await fetch(apiURL("/api/rules"));
    if (!response.ok) {
      throw new Error((await response.json()).error || "读取规则失败");
    }
    state.rules = await response.json();
    state.selected = state.rules.length > 0 ? 0 : -1;
    renderList();
    if (state.selected >= 0) {
      selectRule(state.selected);
    } else {
      selectRule(-1);
    }
    setStatus(`已加载 ${state.rules.length} 条规则`, false);
  }

  async function saveRules() {
    updateCurrentRuleFromForm();
    const response = await fetch(apiURL("/api/rules"), {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(state.rules)
    });
    const payload = await response.json();
    if (!response.ok) {
      throw new Error(payload.error || "保存规则失败");
    }
    state.rules = payload;
    if (state.selected >= state.rules.length) {
      state.selected = state.rules.length - 1;
    }
    renderList();
    setStatus("规则已保存", false);
  }

  function moveSelected(delta) {
    const from = state.selected;
    const to = from + delta;
    if (from < 0 || to < 0 || to >= state.rules.length) {
      return;
    }
    updateCurrentRuleFromForm();
    const current = state.rules[from];
    state.rules[from] = state.rules[to];
    state.rules[to] = current;
    selectRule(to);
  }

  elements.newRule.addEventListener("click", () => {
    if (state.selected >= 0) {
      updateCurrentRuleFromForm();
    }
    state.rules.push(emptyRule());
    selectRule(state.rules.length - 1);
    setStatus("已新增，保存后生效", false);
  });

  elements.saveRule.addEventListener("click", () => {
    saveRules().catch((error) => setStatus(error.message, true));
  });

  elements.deleteRule.addEventListener("click", () => {
    if (state.selected < 0) {
      return;
    }
    state.rules.splice(state.selected, 1);
    const next = Math.min(state.selected, state.rules.length - 1);
    state.selected = next;
    if (next >= 0) {
      selectRule(next);
    } else {
      selectRule(-1);
    }
    setStatus("已删除规则，保存后生效", false);
  });

  elements.moveUp.addEventListener("click", () => moveSelected(-1));
  elements.moveDown.addEventListener("click", () => moveSelected(1));
  elements.replyType.addEventListener("change", updateAreaAvailability);
  elements.form.addEventListener("submit", (event) => {
    event.preventDefault();
    saveRules().catch((error) => setStatus(error.message, true));
  });

  loadRules().catch((error) => setStatus(error.message, true));
})();
