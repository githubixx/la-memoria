(function () {
  "use strict";

  var initializedForms = new WeakSet();

  function setupMenu(menu) {
    var toggle = menu.querySelector(".menu-toggle");
    var list = menu.querySelector(".menu-list");
    if (!toggle || !list) return;

    function setOpen(open) {
      list.classList.toggle("open", open);
      toggle.setAttribute("aria-expanded", String(open));
    }

    toggle.addEventListener("click", function () {
      setOpen(toggle.getAttribute("aria-expanded") !== "true");
    });
    menu.addEventListener("keydown", function (event) {
      if (event.key === "Escape") {
        setOpen(false);
        toggle.focus();
      }
    });
    menu.addEventListener("click", function (event) {
      if (event.target.closest("a, button[type='submit']")) {
        setOpen(false);
        toggle.focus();
      }
    });
    document.addEventListener("click", function (event) {
      if (!menu.contains(event.target)) setOpen(false);
    });
    menu.addEventListener("focusout", function (event) {
      if (!menu.contains(event.relatedTarget)) setOpen(false);
    });
    setOpen(false);
    menu.classList.add("menu-ready");
  }

  function setupTags(form) {
    var input = form.querySelector("[data-tag-input]");
    var addButton = form.querySelector("[data-tag-add]");
    var list = form.querySelector("[data-tag-list]");
    if (!input || !addButton || !list) return;

    function addTag(value) {
      var trimmed = value.trim();
      if (!trimmed) return;
      var item = document.createElement("li");
      var hidden = document.createElement("input");
      hidden.type = "hidden";
      hidden.name = "tags";
      hidden.value = trimmed;
      var label = document.createElement("span");
      label.textContent = trimmed;
      var remove = document.createElement("button");
      remove.type = "button";
      remove.textContent = "Remove";
      remove.setAttribute("aria-label", "Remove tag " + trimmed);
      remove.addEventListener("click", function () {
        item.remove();
      });
      item.appendChild(label);
      item.appendChild(hidden);
      item.appendChild(remove);
      list.appendChild(item);
      input.value = "";
      input.focus();
    }

    addButton.addEventListener("click", function () {
      addTag(input.value);
    });
    input.addEventListener("keydown", function (event) {
      if (event.key === "Enter") {
        event.preventDefault();
        addTag(input.value);
      }
    });
  }

  function setupCaptureTrigger(form) {
    var urlField = form.querySelector("[data-capture-trigger]");
    var status = form.querySelector("#capture-status");
    if (!urlField || !status || typeof window.htmx === "undefined") return;

    var lastValue = urlField.value;
    urlField.addEventListener("change", function () {
      if (urlField.value === lastValue || !urlField.value) return;
      lastValue = urlField.value;
      status.innerHTML = "";
      window.htmx.ajax("POST", "/captures", {
        target: "#capture-status",
        swap: "outerHTML",
        values: { url: urlField.value, bookmark_id: form.querySelector("[name=bookmark_id]")?.value || "", csrf_token: form.querySelector("[name=csrf_token]").value },
      });
    });
  }

  function setupForms() {
    document.querySelectorAll("form").forEach(function (form) {
      if (initializedForms.has(form)) return;
      setupTags(form);
      setupCaptureTrigger(form);
      initializedForms.add(form);
    });
  }

  document.querySelectorAll(".site-menu").forEach(setupMenu);
  setupForms();
  document.body.addEventListener("htmx:afterSwap", setupForms);
})();
