import { FOCUSABLE_INPUTS } from "../constants";

export class FocusManager {
  private wrapperElement: Element | null = null;
  private focusableElements: HTMLElement[] = [];
  private lastFocusedElement: HTMLElement | null = null;
  private lastFocusedSelectionStart: number | null = null;
  private lastFocusedSelectionEnd: number | null = null;

  attach(wrapper: Element | null): void {
    this.wrapperElement = wrapper;

    if (!wrapper) {
      return;
    }

    this.updateFocusableElements();
    this.setupFocusTracking();
  }

  reset(): void {
    this.wrapperElement = null;
    this.focusableElements = [];
    this.lastFocusedElement = null;
    this.lastFocusedSelectionStart = null;
    this.lastFocusedSelectionEnd = null;
  }

  updateFocusableElements(): void {
    if (!this.wrapperElement) return;

    const inputSelectors = FOCUSABLE_INPUTS.map((type) =>
      type === "textarea"
        ? "textarea:not([disabled])"
        : `input[type="${type}"]:not([disabled])`
    ).join(", ");

    const otherFocusable =
      'select:not([disabled]), button:not([disabled]), [contenteditable="true"], [tabindex]:not([tabindex="-1"])';
    const selector = `${inputSelectors}, ${otherFocusable}`;

    this.focusableElements = Array.from(
      this.wrapperElement.querySelectorAll(selector)
    );
  }

  setupFocusTracking(): void {
    if (!this.wrapperElement) return;

    const wrapperId = this.wrapperElement.getAttribute("data-lvt-id");
    const focusKey = `__lvt_focus_tracker_${wrapperId}`;
    const blurKey = `__lvt_blur_tracker_${wrapperId}`;

    const focusListener = (event: Event) => {
      const target = event.target as HTMLElement;
      if (!target || !this.wrapperElement?.contains(target)) return;

      if (this.isTextualInput(target) || target instanceof HTMLSelectElement) {
        this.lastFocusedElement = target;
        console.log(
          "[Focus Debug] Tracked focus on:",
          target.tagName,
          target.id || target.getAttribute("name")
        );

        if (this.isTextualInput(target)) {
          this.lastFocusedSelectionStart = target.selectionStart;
          this.lastFocusedSelectionEnd = target.selectionEnd;
        }
      }
    };

    const blurListener = (event: Event) => {
      const target = event.target as HTMLElement;
      if (!target || !this.wrapperElement?.contains(target)) return;

      if (this.isTextualInput(target) && target === this.lastFocusedElement) {
        this.lastFocusedSelectionStart = target.selectionStart;
        this.lastFocusedSelectionEnd = target.selectionEnd;
        console.log(
          "[Focus Debug] Saved cursor on blur:",
          this.lastFocusedSelectionStart,
          "-",
          this.lastFocusedSelectionEnd
        );
      }
    };

    if ((document as any)[focusKey]) {
      document.removeEventListener("focus", (document as any)[focusKey], true);
    }
    if ((document as any)[blurKey]) {
      document.removeEventListener("blur", (document as any)[blurKey], true);
    }

    (document as any)[focusKey] = focusListener;
    (document as any)[blurKey] = blurListener;

    document.addEventListener("focus", focusListener, true);
    document.addEventListener("blur", blurListener, true);

    console.log("[Focus Debug] Focus tracking set up");
  }

  restoreFocusedElement(): void {
    console.log(
      "[Focus Debug] restoreFocusedElement - lastFocusedElement:",
      this.lastFocusedElement?.tagName,
      this.lastFocusedElement?.id ||
        this.lastFocusedElement?.getAttribute("name")
    );

    if (!this.lastFocusedElement || !this.wrapperElement) {
      console.log("[Focus Debug] No element to restore");
      return;
    }

    const selector = this.getElementSelector(this.lastFocusedElement);
    console.log("[Focus Debug] Selector for last focused:", selector);

    if (!selector) {
      console.log("[Focus Debug] Could not generate selector");
      return;
    }

    let element: HTMLElement | null = null;

    if (selector.startsWith("data-focus-index-")) {
      this.updateFocusableElements();
      const index = parseInt(selector.replace("data-focus-index-", ""), 10);
      element = this.focusableElements[index] || null;
      console.log("[Focus Debug] Found by index:", index, element?.tagName);
    } else {
      element = this.wrapperElement.querySelector(selector);
      console.log(
        "[Focus Debug] Found by selector:",
        selector,
        element?.tagName
      );
    }

    if (!element) {
      console.log("[Focus Debug] Element not found in updated DOM");
      return;
    }

    const wasFocused = element.matches(":focus");
    console.log("[Focus Debug] Already focused:", wasFocused);

    if (!wasFocused) {
      element.focus();
      console.log("[Focus Debug] Restored focus");
    }

    if (
      this.isTextualInput(element) &&
      this.lastFocusedSelectionStart !== null &&
      this.lastFocusedSelectionEnd !== null
    ) {
      element.setSelectionRange(
        this.lastFocusedSelectionStart,
        this.lastFocusedSelectionEnd
      );
      console.log(
        "[Focus Debug] Restored cursor:",
        this.lastFocusedSelectionStart,
        "-",
        this.lastFocusedSelectionEnd
      );
    }
  }

  isTextualInput(el: Element): el is HTMLInputElement | HTMLTextAreaElement {
    if (el instanceof HTMLTextAreaElement) return true;
    if (el instanceof HTMLInputElement) {
      return FOCUSABLE_INPUTS.indexOf(el.type) >= 0;
    }
    return false;
  }

  getLastFocusedElement(): HTMLElement | null {
    return this.lastFocusedElement;
  }

  private getElementSelector(el: HTMLElement): string | null {
    if (el.id) return `#${el.id}`;
    if ((el as any).name) return `[name="${(el as any).name}"]`;
    if (el.getAttribute("data-key"))
      return `[data-key="${el.getAttribute("data-key")}"]`;

    const index = this.focusableElements.indexOf(el);
    return index >= 0 ? `data-focus-index-${index}` : null;
  }
}
