import type { Logger } from "../utils/logger";

/**
 * Manages client-side modal interactions for LiveTemplate.
 */
export class ModalManager {
  constructor(private readonly logger: Logger) {}

  open(modalId: string): void {
    const modal = document.getElementById(modalId);
    if (!modal) {
      this.logger.warn(`Modal with id="${modalId}" not found`);
      return;
    }

    modal.removeAttribute("hidden");
    modal.style.display = "flex";
    modal.setAttribute("aria-hidden", "false");
    modal.dispatchEvent(new CustomEvent("lvt:modal-opened", { bubbles: true }));

    this.logger.info(`Opened modal: ${modalId}`);

    const firstInput = modal.querySelector(
      "input, textarea, select"
    ) as HTMLElement | null;
    if (firstInput) {
      setTimeout(() => {
        const activeElement = document.activeElement as HTMLElement | null;
        const isVisible = (element: HTMLElement | null): boolean => {
          if (!element) {
            return false;
          }

          if (element === document.body) {
            return true;
          }

          // Use offsetParent and bounding rects to determine visibility without hitting layout too hard.
          if (element.offsetParent !== null) {
            return true;
          }

          return element.getClientRects().length > 0;
        };

        const shouldMoveFocus =
          !activeElement ||
          !modal.contains(activeElement) ||
          !isVisible(activeElement);

        if (shouldMoveFocus) {
          firstInput.focus();
        }
      }, 100);
    }
  }

  close(modalId: string): void {
    const modal = document.getElementById(modalId);
    if (!modal) {
      this.logger.warn(`Modal with id="${modalId}" not found`);
      return;
    }

    modal.setAttribute("hidden", "");
    modal.style.display = "none";
    modal.setAttribute("aria-hidden", "true");
    modal.dispatchEvent(new CustomEvent("lvt:modal-closed", { bubbles: true }));

    this.logger.info(`Closed modal: ${modalId}`);
  }
}
