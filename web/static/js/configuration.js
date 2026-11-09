// Configuration validation errors are intentional form responses. Limit the
// override to this form and target so unrelated htmx errors retain defaults.
document.addEventListener("htmx:beforeSwap", (event) => {
  const detail = event.detail;
  const source = detail?.requestConfig?.elt;
  if (
    detail?.xhr?.status === 422 &&
    detail.target?.id === "content" &&
    source instanceof Element &&
    source.matches('form[action="/configuration"]')
  ) {
    detail.shouldSwap = true;
    detail.isError = false;
  }
});
