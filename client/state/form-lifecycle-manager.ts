import type { ResponseMetadata } from "../types";
import { ModalManager } from "../dom/modal-manager";

/**
 * Tracks form submission lifecycle for LiveTemplate actions.
 */
export class FormLifecycleManager {
  private activeForm: HTMLFormElement | null = null;
  private activeButton: HTMLButtonElement | null = null;
  private originalButtonText: string | null = null;

  constructor(private readonly modalManager: ModalManager) {}

  setActiveSubmission(
    form: HTMLFormElement | null,
    button: HTMLButtonElement | null,
    originalButtonText: string | null
  ): void {
    this.activeForm = form;
    this.activeButton = button;
    this.originalButtonText = originalButtonText;
  }

  handleResponse(meta: ResponseMetadata): void {
    if (this.activeForm) {
      this.activeForm.dispatchEvent(
        new CustomEvent("lvt:done", { detail: meta })
      );
    }

    if (meta.success) {
      this.handleSuccess(meta);
    } else {
      this.handleError(meta);
    }

    this.restoreFormState();
  }

  reset(): void {
    this.restoreFormState();
  }

  private handleSuccess(meta: ResponseMetadata): void {
    if (!this.activeForm) {
      return;
    }

    this.activeForm.dispatchEvent(
      new CustomEvent("lvt:success", { detail: meta })
    );

    const modalParent = this.activeForm.closest('[role="dialog"]');
    if (modalParent && modalParent.id) {
      this.modalManager.close(modalParent.id);
    }

    if (!this.activeForm.hasAttribute("lvt-preserve")) {
      this.activeForm.reset();
    }
  }

  private handleError(meta: ResponseMetadata): void {
    if (!this.activeForm) {
      return;
    }

    this.activeForm.dispatchEvent(
      new CustomEvent("lvt:error", { detail: meta })
    );
  }

  private restoreFormState(): void {
    if (this.activeButton && this.originalButtonText !== null) {
      this.activeButton.disabled = false;
      this.activeButton.textContent = this.originalButtonText;
    }

    this.activeForm = null;
    this.activeButton = null;
    this.originalButtonText = null;
  }
}
