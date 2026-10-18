import "@testing-library/jest-dom/vitest";
import "@/app/workspace-i18n";
import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

// ProseMirror uses layout APIs that jsdom does not implement. Keep the editor
// behavior tests deterministic without changing the production editor path.
if (typeof document.elementFromPoint !== "function") {
  document.elementFromPoint = () => null;
}
if (typeof Range.prototype.getClientRects !== "function") {
  Range.prototype.getClientRects = () => [] as unknown as DOMRectList;
}
if (typeof Range.prototype.getBoundingClientRect !== "function") {
  Range.prototype.getBoundingClientRect = () => new DOMRect();
}
if (typeof HTMLElement.prototype.scrollIntoView !== "function") {
  HTMLElement.prototype.scrollIntoView = () => undefined;
}
if (!Object.getOwnPropertyDescriptor(HTMLDivElement.prototype, "value")) {
  Object.defineProperty(HTMLDivElement.prototype, "value", {
    configurable: true,
    get() {
      return this.getAttribute("value") || this.textContent || "";
    },
    set(value: string) {
      this.setAttribute("value", value);
    },
  });
}

afterEach(() => {
  cleanup();
});
