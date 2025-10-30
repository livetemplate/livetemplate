import { debounce, throttle } from "../utils/rate-limit";
import type { Logger } from "../utils/logger";

export interface EventDelegationContext {
  getWrapperElement(): Element | null;
  getRateLimitedHandlers(): WeakMap<Element, Map<string, Function>>;
  parseValue(value: string): any;
  send(message: any): void;
  setActiveSubmission(
    form: HTMLFormElement | null,
    button: HTMLButtonElement | null,
    originalButtonText: string | null
  ): void;
  openModal(modalId: string): void;
  closeModal(modalId: string): void;
  getWebSocketReadyState(): number | undefined;
}

/**
 * Handles all DOM event delegation concerns for LiveTemplateClient.
 */
export class EventDelegator {
  constructor(
    private readonly context: EventDelegationContext,
    private readonly logger: Logger
  ) {}

  setupEventDelegation(): void {
    const wrapperElement = this.context.getWrapperElement();
    if (!wrapperElement) return;

    const eventTypes = [
      "click",
      "submit",
      "change",
      "input",
      "keydown",
      "keyup",
      "focus",
      "blur",
      "mouseenter",
      "mouseleave",
    ];
    const wrapperId = wrapperElement.getAttribute("data-lvt-id");
    const rateLimitedHandlers = this.context.getRateLimitedHandlers();

    eventTypes.forEach((eventType) => {
      const listenerKey = `__lvt_delegated_${eventType}_${wrapperId}`;
      const existingListener = (document as any)[listenerKey];
      if (existingListener) {
        document.removeEventListener(eventType, existingListener, false);
      }

      const listener = (e: Event) => {
        const currentWrapper = this.context.getWrapperElement();
        if (!currentWrapper) return;

        if (eventType === "submit") {
          (window as any).__lvtSubmitListenerTriggered = true;
          (window as any).__lvtSubmitEventTarget = (
            e.target as Element
          )?.tagName;
        }

        this.logger.debug("Event listener triggered:", eventType, e.target);

        const target = e.target as Element;
        if (!target) return;

        let element: Element | null = target;
        let inWrapper = false;

        while (element) {
          if (element === currentWrapper) {
            inWrapper = true;
            break;
          }
          element = element.parentElement;
        }

        if (eventType === "submit") {
          (window as any).__lvtInWrapper = inWrapper;
          (window as any).__lvtWrapperElement =
            currentWrapper.getAttribute("data-lvt-id");
        }

        if (!inWrapper) return;

        const attrName = `lvt-${eventType}`;
        element = target;

        while (element && element !== currentWrapper.parentElement) {
          let action = element.getAttribute(attrName);
          let actionElement = element;

          if (!action && (eventType === "change" || eventType === "input")) {
            const formElement: HTMLFormElement | null = element.closest("form");
            if (formElement && formElement.hasAttribute("lvt-change")) {
              action = formElement.getAttribute("lvt-change");
              actionElement = formElement;
            }
          }

          if (action && actionElement) {
            if (eventType === "submit") {
              (window as any).__lvtActionFound = action;
              (window as any).__lvtActionElement = actionElement.tagName;
            }

            if (eventType === "submit") {
              e.preventDefault();
            }

            if (
              (eventType === "keydown" || eventType === "keyup") &&
              actionElement.hasAttribute("lvt-key")
            ) {
              const keyFilter = actionElement.getAttribute("lvt-key");
              const keyboardEvent = e as KeyboardEvent;
              if (keyFilter && keyboardEvent.key !== keyFilter) {
                element = element.parentElement;
                continue;
              }
            }

            const targetElement = actionElement;

            const handleAction = () => {
              this.logger.debug("handleAction called", {
                action,
                eventType,
                targetElement,
              });

              if (
                action === "delete" &&
                targetElement.hasAttribute("lvt-confirm")
              ) {
                const confirmMessage =
                  targetElement.getAttribute("lvt-confirm") ||
                  "Are you sure you want to delete this item?";
                if (!confirm(confirmMessage)) {
                  this.logger.debug("Delete action cancelled by user");
                  return;
                }
              }

              const message: any = { action, data: {} };

              if (targetElement instanceof HTMLFormElement) {
                this.logger.debug("Processing form element");
                const formData = new FormData(targetElement);

                const checkboxes = Array.from(
                  targetElement.querySelectorAll('input[type="checkbox"][name]')
                ) as HTMLInputElement[];
                const checkboxNames = new Set(checkboxes.map((cb) => cb.name));

                checkboxNames.forEach((name) => {
                  message.data[name] = false;
                });

                formData.forEach((value, key) => {
                  if (checkboxNames.has(key)) {
                    message.data[key] = true;
                    this.logger.debug("Converted checkbox", key, "to true");
                  } else {
                    message.data[key] = this.context.parseValue(
                      value as string
                    );
                  }
                });
                this.logger.debug("Form data collected:", message.data);
              } else if (eventType === "change" || eventType === "input") {
                if (targetElement instanceof HTMLInputElement) {
                  const key = targetElement.name || "value";
                  message.data[key] = this.context.parseValue(
                    targetElement.value
                  );
                } else if (targetElement instanceof HTMLSelectElement) {
                  const key = targetElement.name || "value";
                  message.data[key] = this.context.parseValue(
                    targetElement.value
                  );
                } else if (targetElement instanceof HTMLTextAreaElement) {
                  const key = targetElement.name || "value";
                  message.data[key] = this.context.parseValue(
                    targetElement.value
                  );
                }
              }

              Array.from(targetElement.attributes).forEach((attr) => {
                if (attr.name.startsWith("lvt-data-")) {
                  const key = attr.name.replace("lvt-data-", "");
                  message.data[key] = this.context.parseValue(attr.value);
                }
              });

              Array.from(targetElement.attributes).forEach((attr) => {
                if (attr.name.startsWith("lvt-value-")) {
                  const key = attr.name.replace("lvt-value-", "");
                  message.data[key] = this.context.parseValue(attr.value);
                }
              });

              if (
                eventType === "submit" &&
                targetElement instanceof HTMLFormElement
              ) {
                const submitEvent = e as SubmitEvent;
                const submitButton =
                  submitEvent.submitter as HTMLButtonElement | null;
                let originalButtonText: string | null = null;

                if (
                  submitButton &&
                  submitButton.hasAttribute("lvt-disable-with")
                ) {
                  originalButtonText = submitButton.textContent;
                  submitButton.disabled = true;
                  submitButton.textContent =
                    submitButton.getAttribute("lvt-disable-with");
                  this.logger.debug("Disabled submit button");
                }

                this.context.setActiveSubmission(
                  targetElement,
                  submitButton || null,
                  originalButtonText
                );

                targetElement.dispatchEvent(
                  new CustomEvent("lvt:pending", { detail: message })
                );
                this.logger.debug("Emitted lvt:pending event");
              }

              this.logger.debug("About to send message:", message);
              this.logger.debug(
                "WebSocket state:",
                this.context.getWebSocketReadyState()
              );

              this.context.send(message);
              this.logger.debug("send() called");
            };

            const throttleValue = actionElement.getAttribute("lvt-throttle");
            const debounceValue = actionElement.getAttribute("lvt-debounce");

            if (throttleValue || debounceValue) {
              if (!rateLimitedHandlers.has(actionElement)) {
                rateLimitedHandlers.set(actionElement, new Map());
              }
              const handlerCache = rateLimitedHandlers.get(actionElement)!;
              const cacheKey = `${eventType}:${action}`;

              let rateLimitedHandler = handlerCache.get(cacheKey);
              if (!rateLimitedHandler) {
                if (throttleValue) {
                  const limit = parseInt(throttleValue, 10);
                  rateLimitedHandler = throttle(handleAction, limit);
                } else if (debounceValue) {
                  const wait = parseInt(debounceValue, 10);
                  rateLimitedHandler = debounce(handleAction, wait);
                }
                if (rateLimitedHandler) {
                  handlerCache.set(cacheKey, rateLimitedHandler);
                }
              }

              if (rateLimitedHandler) {
                rateLimitedHandler();
              }
            } else {
              if (eventType === "submit") {
                (window as any).__lvtBeforeHandleAction = true;
              }
              handleAction();
              if (eventType === "submit") {
                (window as any).__lvtAfterHandleAction = true;
              }
            }

            return;
          }
          element = element.parentElement;
        }
      };

      (document as any)[listenerKey] = listener;
      document.addEventListener(eventType, listener, false);
      this.logger.debug(
        "Registered event listener:",
        eventType,
        "with key:",
        listenerKey
      );
    });
  }

  setupWindowEventDelegation(): void {
    const wrapperElement = this.context.getWrapperElement();
    if (!wrapperElement) return;

    const windowEvents = [
      "keydown",
      "keyup",
      "scroll",
      "resize",
      "focus",
      "blur",
    ];
    const wrapperId = wrapperElement.getAttribute("data-lvt-id");
    const rateLimitedHandlers = this.context.getRateLimitedHandlers();

    windowEvents.forEach((eventType) => {
      const listenerKey = `__lvt_window_${eventType}_${wrapperId}`;
      const existingListener = (window as any)[listenerKey];
      if (existingListener) {
        window.removeEventListener(eventType, existingListener);
      }

      const listener = (e: Event) => {
        const currentWrapper = this.context.getWrapperElement();
        if (!currentWrapper) return;

        const attrName = `lvt-window-${eventType}`;
        const elements = currentWrapper.querySelectorAll(`[${attrName}]`);

        elements.forEach((element) => {
          const action = element.getAttribute(attrName);
          if (!action) return;

          if (
            (eventType === "keydown" || eventType === "keyup") &&
            element.hasAttribute("lvt-key")
          ) {
            const keyFilter = element.getAttribute("lvt-key");
            const keyboardEvent = e as KeyboardEvent;
            if (keyFilter && keyboardEvent.key !== keyFilter) {
              return;
            }
          }

          const message: any = { action, data: {} };

          Array.from(element.attributes).forEach((attr) => {
            if (attr.name.startsWith("lvt-data-")) {
              const key = attr.name.replace("lvt-data-", "");
              message.data[key] = this.context.parseValue(attr.value);
            }
          });

          Array.from(element.attributes).forEach((attr) => {
            if (attr.name.startsWith("lvt-value-")) {
              const key = attr.name.replace("lvt-value-", "");
              message.data[key] = this.context.parseValue(attr.value);
            }
          });

          const throttleValue = element.getAttribute("lvt-throttle");
          const debounceValue = element.getAttribute("lvt-debounce");

          const handleAction = () => this.context.send(message);

          if (throttleValue || debounceValue) {
            if (!rateLimitedHandlers.has(element)) {
              rateLimitedHandlers.set(element, new Map());
            }
            const handlerCache = rateLimitedHandlers.get(element)!;
            const cacheKey = `window-${eventType}:${action}`;

            let rateLimitedHandler = handlerCache.get(cacheKey);
            if (!rateLimitedHandler) {
              if (throttleValue) {
                const limit = parseInt(throttleValue, 10);
                rateLimitedHandler = throttle(handleAction, limit);
              } else if (debounceValue) {
                const wait = parseInt(debounceValue, 10);
                rateLimitedHandler = debounce(handleAction, wait);
              }
              if (rateLimitedHandler) {
                handlerCache.set(cacheKey, rateLimitedHandler);
              }
            }

            if (rateLimitedHandler) {
              rateLimitedHandler();
            }
          } else {
            handleAction();
          }
        });
      };

      (window as any)[listenerKey] = listener;
      window.addEventListener(eventType, listener);
    });
  }

  setupClickAwayDelegation(): void {
    const wrapperElement = this.context.getWrapperElement();
    if (!wrapperElement) return;

    const wrapperId = wrapperElement.getAttribute("data-lvt-id");
    const listenerKey = `__lvt_click_away_${wrapperId}`;
    const existingListener = (document as any)[listenerKey];
    if (existingListener) {
      document.removeEventListener("click", existingListener);
    }

    const listener = (e: Event) => {
      const currentWrapper = this.context.getWrapperElement();
      if (!currentWrapper) return;

      const target = e.target as Element;
      const elements = currentWrapper.querySelectorAll("[lvt-click-away]");

      elements.forEach((element) => {
        if (!element.contains(target)) {
          const action = element.getAttribute("lvt-click-away");
          if (!action) return;

          const message: any = { action, data: {} };

          Array.from(element.attributes).forEach((attr) => {
            if (attr.name.startsWith("lvt-data-")) {
              const key = attr.name.replace("lvt-data-", "");
              message.data[key] = this.context.parseValue(attr.value);
            }
          });

          Array.from(element.attributes).forEach((attr) => {
            if (attr.name.startsWith("lvt-value-")) {
              const key = attr.name.replace("lvt-value-", "");
              message.data[key] = this.context.parseValue(attr.value);
            }
          });

          this.context.send(message);
        }
      });
    };

    (document as any)[listenerKey] = listener;
    document.addEventListener("click", listener);
  }

  setupModalDelegation(): void {
    const wrapperElement = this.context.getWrapperElement();
    if (!wrapperElement) return;

    const wrapperId = wrapperElement.getAttribute("data-lvt-id");

    const openListenerKey = `__lvt_modal_open_${wrapperId}`;
    const existingOpenListener = (document as any)[openListenerKey];
    if (existingOpenListener) {
      document.removeEventListener("click", existingOpenListener);
    }

    const openListener = (e: Event) => {
      const currentWrapper = this.context.getWrapperElement();
      if (!currentWrapper) return;

      const target = (e.target as Element)?.closest("[lvt-modal-open]");
      if (!target || !currentWrapper.contains(target)) return;

      const modalId = target.getAttribute("lvt-modal-open");
      if (!modalId) return;

      e.preventDefault();
      this.context.openModal(modalId);
    };

    (document as any)[openListenerKey] = openListener;
    document.addEventListener("click", openListener);

    const closeListenerKey = `__lvt_modal_close_${wrapperId}`;
    const existingCloseListener = (document as any)[closeListenerKey];
    if (existingCloseListener) {
      document.removeEventListener("click", existingCloseListener);
    }

    const closeListener = (e: Event) => {
      const currentWrapper = this.context.getWrapperElement();
      if (!currentWrapper) return;

      const target = (e.target as Element)?.closest("[lvt-modal-close]");
      if (!target || !currentWrapper.contains(target)) {
        return;
      }

      const modalId = target.getAttribute("lvt-modal-close");
      if (!modalId) return;

      e.preventDefault();
      this.context.closeModal(modalId);
    };

    (document as any)[closeListenerKey] = closeListener;
    document.addEventListener("click", closeListener);

    const backdropListenerKey = `__lvt_modal_backdrop_${wrapperId}`;
    const existingBackdropListener = (document as any)[backdropListenerKey];
    if (existingBackdropListener) {
      document.removeEventListener("click", existingBackdropListener);
    }

    const backdropListener = (e: Event) => {
      const target = e.target as Element;
      if (!target.hasAttribute("data-modal-backdrop")) return;

      const modalId = target.getAttribute("data-modal-id");
      if (modalId) {
        this.context.closeModal(modalId);
      }
    };

    (document as any)[backdropListenerKey] = backdropListener;
    document.addEventListener("click", backdropListener);

    const escapeListenerKey = `__lvt_modal_escape_${wrapperId}`;
    const existingEscapeListener = (document as any)[escapeListenerKey];
    if (existingEscapeListener) {
      document.removeEventListener("keydown", existingEscapeListener);
    }

    const escapeListener = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      const currentWrapper = this.context.getWrapperElement();
      if (!currentWrapper) return;

      const openModals = currentWrapper.querySelectorAll(
        '[role="dialog"]:not([hidden])'
      );
      if (openModals.length > 0) {
        const lastModal = openModals[openModals.length - 1];
        if (lastModal.id) {
          this.context.closeModal(lastModal.id);
        }
      }
    };

    (document as any)[escapeListenerKey] = escapeListener;
    document.addEventListener("keydown", escapeListener);
  }
}
