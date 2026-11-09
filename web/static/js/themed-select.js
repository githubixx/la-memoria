// Enhance only devices where a custom popup is preferable to the native picker.
(() => {
  const media = matchMedia("(hover: hover) and (pointer: fine) and (forced-colors: none)");
  const enhanced = new Map();
  const pendingSwaps = new Map();
  const supported = CSS.supports("color", "light-dark(black, white)") &&
    CSS.supports("color", "color-mix(in srgb, red, white)");

  // The select must stay in the form for validation, reset, and submission.
  // Do not hide it from assistive technology unless CSS actually removes it
  // from layout, painting, and pointer hit testing, and the replacement can
  // itself be seen.
  function enhancementIsSafe(select, button) {
    const nativeStyle = getComputedStyle(select);
    const buttonStyle = getComputedStyle(button);
    const bounds = button.getBoundingClientRect();
    return nativeStyle.position === "absolute" &&
      Number(nativeStyle.opacity) === 0 &&
      nativeStyle.pointerEvents === "none" &&
      nativeStyle.display !== "none" &&
      buttonStyle.display !== "none" &&
      buttonStyle.visibility === "visible" &&
      Number(buttonStyle.opacity) > 0 &&
      bounds.width > 0 && bounds.height > 0 &&
      (typeof button.checkVisibility !== "function" ||
        button.checkVisibility({ opacityProperty: true, visibilityProperty: true }));
  }

  const resizeObserver = typeof ResizeObserver === "function" ? new ResizeObserver(() => {
    for (const [select, widget] of enhanced) {
      if (select.isConnected && !enhancementIsSafe(select, widget.button)) {
        refresh();
        break;
      }
    }
  }) : null;

  function enhance(select) {
    const labels = Array.from(select.labels);
    if (!select.id || labels.length === 0 || select.closest(".themed-select")) return;

    const controller = new AbortController();
    const signal = controller.signal;
    const originalTabindex = select.getAttribute("tabindex");
    const originalHidden = select.getAttribute("aria-hidden");
    const originallyThemed = select.classList.contains("themed-select-native");
    const originallyClassed = select.hasAttribute("class");
    const hadFocus = document.activeElement === select;
    const restoreNativeClass = () => {
      if (!originallyThemed) {
        select.classList.remove("themed-select-native");
        if (!originallyClassed && select.getAttribute("class") === "") select.removeAttribute("class");
      }
    };
    const wrapper = document.createElement("div");
    const button = document.createElement("button");
    const list = document.createElement("div");
    const error = document.createElement("span");
    wrapper.className = "themed-select";
    button.className = "themed-select-trigger";
    if (document.documentElement.dataset.theme === "spaceship-2") {
      button.classList.add("custom-select-trigger");
    }
    button.type = "button";
    button.id = `${select.id}-trigger`;
    button.textContent = select.selectedOptions[0]?.textContent || "";
    button.setAttribute("role", "combobox");
    button.setAttribute("aria-haspopup", "listbox");
    button.setAttribute("aria-expanded", "false");
    list.className = "themed-select-list";
    list.id = `${select.id}-listbox`;
    list.setAttribute("role", "listbox");
    list.hidden = true;
    button.setAttribute("aria-controls", list.id);
    select.before(wrapper);
    error.id = `${select.id}-native-error`;
    error.className = "field-error";
    error.setAttribute("role", "alert");
    error.hidden = true;
    wrapper.append(select, button, list, error);
    select.classList.add("themed-select-native");

    // Applying the class and measuring are synchronous, so a failed attempt
    // is rolled back before either control can be painted as a duplicate.
    if (!enhancementIsSafe(select, button)) {
      restoreNativeClass();
      wrapper.replaceWith(select);
      if (hadFocus) select.focus({ preventScroll: true });
      return;
    }

    const labelStates = labels.map(label => ({
      id: label.getAttribute("id"), target: label.getAttribute("for"),
    }));
    labels.forEach((label, index) => {
      if (!label.id) label.id = `${select.id}-label-${index}`;
    });
    button.setAttribute("aria-labelledby", labels.map(label => label.id).join(" "));
    list.setAttribute("aria-labelledby", labels.map(label => label.id).join(" "));
    select.setAttribute("tabindex", "-1");
    select.setAttribute("aria-hidden", "true");
    labels.forEach(label => { label.htmlFor = button.id; });

    let active = -1;
    let search = "";
    let searchAt = 0;
    const options = () => Array.from(select.options);
    const enabled = option => option && !option.disabled && !option.parentElement?.disabled;
    const close = () => {
      list.hidden = true;
      button.setAttribute("aria-expanded", "false");
      button.removeAttribute("aria-activedescendant");
    };
    function render(scroll = false) {
      const entries = options();
      button.textContent = select.selectedOptions[0]?.textContent || "";
      button.disabled = select.matches(":disabled");
      for (const attr of ["aria-invalid", "aria-describedby", "aria-errormessage", "aria-required"]) {
        if (select.hasAttribute(attr)) button.setAttribute(attr, select.getAttribute(attr));
        else button.removeAttribute(attr);
      }
      if (select.required) button.setAttribute("aria-required", "true");
      if (!error.hidden) {
        if (select.validity.valid) {
          error.hidden = true;
          error.textContent = "";
        } else {
          button.setAttribute("aria-invalid", "true");
          button.setAttribute("aria-describedby", [select.getAttribute("aria-describedby"), error.id].filter(Boolean).join(" "));
        }
      }
      list.replaceChildren(...entries.map((option, index) => {
        const item = document.createElement("div");
        item.className = "themed-select-option";
        item.id = `${list.id}-option-${index}`;
        item.setAttribute("role", "option");
        item.setAttribute("aria-selected", String(option.selected));
        if (!enabled(option)) item.setAttribute("aria-disabled", "true");
        if (index === active) item.dataset.active = "true";
        item.textContent = option.textContent;
        return item;
      }));
      if (!list.hidden && active >= 0 && entries[active]) {
        const item = list.children[active];
        button.setAttribute("aria-activedescendant", item.id);
        if (scroll) {
          const itemTop = item.offsetTop;
          if (itemTop < list.scrollTop) list.scrollTop = itemTop;
          else if (itemTop + item.offsetHeight > list.scrollTop + list.clientHeight) {
            list.scrollTop = itemTop + item.offsetHeight - list.clientHeight;
          }
        }
      } else {
        button.removeAttribute("aria-activedescendant");
      }
    }
    function position() {
      const rect = button.getBoundingClientRect();
      const below = window.innerHeight - rect.bottom;
      const above = rect.top;
      const upward = below < 160 && above > below;
      list.style.top = upward ? "auto" : "100%";
      list.style.bottom = upward ? "100%" : "auto";
      list.style.maxHeight = `${Math.max(0, Math.min(320, window.innerHeight / 2, (upward ? above : below) - 8))}px`;
    }
    function open() {
      if (button.disabled) return;
      const entries = options();
      active = enabled(entries[select.selectedIndex]) ? select.selectedIndex : entries.findIndex(enabled);
      list.hidden = false;
      position();
      button.setAttribute("aria-expanded", "true");
      render(true);
    }
    function move(index) {
      active = index;
      render(true);
    }
    function commit(index) {
      const option = options()[index];
      if (!enabled(option)) return;
      if (select.selectedIndex !== index) {
        select.selectedIndex = index;
        select.dispatchEvent(new Event("input", { bubbles: true }));
        select.dispatchEvent(new Event("change", { bubbles: true }));
      }
      close();
      render();
    }
    button.addEventListener("click", () => { if (list.hidden) open(); else close(); }, { signal });
    button.addEventListener("keydown", event => {
      const entries = options();
      if (event.key === "Escape") { if (!list.hidden) { event.preventDefault(); close(); } return; }
      if (event.key === "Tab") { close(); return; }
      if (event.key === "Enter" || event.key === " ") {
        event.preventDefault();
        if (list.hidden) open(); else commit(active);
        return;
      }
      if (["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)) {
        event.preventDefault();
        if (list.hidden) {
          open();
          if (event.key === "ArrowDown" || event.key === "ArrowUp") return;
        }
        const direction = event.key === "ArrowUp" || event.key === "End" ? -1 : 1;
        let index = event.key === "Home" ? 0 : event.key === "End" ? entries.length - 1 : active + direction;
        while (index >= 0 && index < entries.length && !enabled(entries[index])) index += direction;
        if (index >= 0 && index < entries.length) move(index);
        return;
      }
      if (event.key.length === 1 && !event.ctrlKey && !event.metaKey && !event.altKey) {
        event.preventDefault();
        if (list.hidden) open();
        const now = Date.now();
        search = now - searchAt > 700 ? event.key : search + event.key;
        searchAt = now;
        const typed = search.toLocaleLowerCase();
        const query = Array.from(typed).every(char => char === typed[0]) ? typed[0] : typed;
        const start = query.length === 1 ? active + 1 : 0;
        for (let offset = 0; offset < entries.length; offset++) {
          const index = (start + offset) % entries.length;
          if (enabled(entries[index]) && entries[index].textContent.trim().toLocaleLowerCase().startsWith(query)) {
            move(index);
            break;
          }
        }
      }
    }, { signal });
    list.addEventListener("mousedown", event => event.preventDefault(), { signal });
    list.addEventListener("click", event => {
      const item = event.target.closest('[role="option"]');
      if (item && list.contains(item)) { commit(Array.from(list.children).indexOf(item)); button.focus(); }
    }, { signal });
    document.addEventListener("pointerdown", event => { if (!wrapper.contains(event.target)) close(); }, { signal });
    window.addEventListener("resize", () => { if (!list.hidden) position(); }, { signal });
    button.addEventListener("blur", () => close(), { signal });
    select.addEventListener("change", () => render(), { signal });
    select.addEventListener("invalid", event => {
      event.preventDefault();
      error.textContent = select.validationMessage;
      error.hidden = false;
      button.setAttribute("aria-invalid", "true");
      button.setAttribute("aria-describedby", [select.getAttribute("aria-describedby"), error.id].filter(Boolean).join(" "));
      button.focus();
    }, { signal });
    select.form?.addEventListener("reset", () => setTimeout(() => { close(); render(); }, 0), { signal });
    const observer = new MutationObserver(() => render());
    observer.observe(select, { subtree: true, childList: true, characterData: true, attributes: true });
    render();
    const destroy = () => {
      const restoreFocus = document.activeElement === button;
      resizeObserver?.unobserve(select);
      resizeObserver?.unobserve(wrapper);
      observer.disconnect();
      controller.abort();
      labels.forEach((label, index) => {
        const { id, target } = labelStates[index];
        if (id === null) label.removeAttribute("id"); else label.setAttribute("id", id);
        if (target === null) label.removeAttribute("for"); else label.setAttribute("for", target);
      });
      restoreNativeClass();
      if (originalTabindex === null) select.removeAttribute("tabindex"); else select.setAttribute("tabindex", originalTabindex);
      if (originalHidden === null) select.removeAttribute("aria-hidden"); else select.setAttribute("aria-hidden", originalHidden);
      wrapper.replaceWith(select);
      if (restoreFocus && select.isConnected) select.focus({ preventScroll: true });
    };
    enhanced.set(select, { button, destroy });
    resizeObserver?.observe(select);
    resizeObserver?.observe(wrapper);
    if (hadFocus) button.focus({ preventScroll: true });
  }
  function refresh() {
    const rejected = new Set();
    for (const [select, widget] of enhanced) {
      if (!select.isConnected || !media.matches || !enhancementIsSafe(select, widget.button)) {
        widget.destroy();
        enhanced.delete(select);
        rejected.add(select);
      }
    }
    if (!supported || !media.matches) return;
    document.querySelectorAll('form[action="/configuration"] select:not([multiple]):not([size])').forEach(select => {
      const settling = Array.from(pendingSwaps.keys()).some(target => target.contains(select));
      if (!settling && !enhanced.has(select) && !rejected.has(select)) enhance(select);
    });
  }
  function beforeSwap(event) {
    const target = event.detail?.target;
    if (!event.detail?.shouldSwap || !(target instanceof Element)) return;
    pendingSwaps.set(target, (pendingSwaps.get(target) || 0) + 1);
    // HTMX copies and then restores matching element attributes while
    // settling. Restore native attributes before it snapshots the old form.
    for (const [select, widget] of enhanced) {
      if (target.contains(select)) {
        widget.destroy();
        enhanced.delete(select);
      }
    }
  }
  function afterSettle(event) {
    const target = event.detail?.target;
    const remaining = pendingSwaps.get(target);
    if (remaining === 1) pendingSwaps.delete(target);
    else if (remaining > 1) pendingSwaps.set(target, remaining - 1);
    for (const root of pendingSwaps.keys()) if (!root.isConnected) pendingSwaps.delete(root);
    refresh();
  }
  // Stylesheets and the active theme can also change after initialization.
  // Recheck on DOM-based style changes, stylesheet completion, or layout
  // changes; a failed check restores the native control and its focus.
  const styleObserver = new MutationObserver(refresh);
  styleObserver.observe(document.head, {
    childList: true, subtree: true, characterData: true, attributes: true,
    attributeFilter: ["disabled", "href", "media", "rel"],
  });
  styleObserver.observe(document.documentElement, {
    attributes: true, attributeFilter: ["class", "data-theme", "style"],
  });
  const stylesheetFinished = event => {
    if (event.target instanceof HTMLLinkElement && event.target.relList.contains("stylesheet")) refresh();
  };
  document.addEventListener("load", stylesheetFinished, true);
  document.addEventListener("error", stylesheetFinished, true);
  window.addEventListener("resize", refresh);
  media.addEventListener("change", refresh);
  document.addEventListener("htmx:beforeSwap", beforeSwap);
  document.addEventListener("htmx:afterSettle", afterSettle);
  document.addEventListener("htmx:swapError", afterSettle);
  refresh();
})();
