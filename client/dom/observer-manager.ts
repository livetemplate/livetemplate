import type { Logger } from "../utils/logger";

export interface ObserverContext {
  getWrapperElement(): Element | null;
  send(message: any): void;
}

/**
 * Manages LiveTemplate observers such as infinite scroll and DOM mutations.
 */
export class ObserverManager {
  private infiniteScrollObserver: IntersectionObserver | null = null;
  private mutationObserver: MutationObserver | null = null;

  constructor(
    private readonly context: ObserverContext,
    private readonly logger: Logger
  ) {}

  setupInfiniteScrollObserver(): void {
    const wrapperElement = this.context.getWrapperElement();
    if (!wrapperElement) return;

    const sentinel = document.getElementById("scroll-sentinel");
    if (!sentinel) {
      return;
    }

    if (this.infiniteScrollObserver) {
      this.infiniteScrollObserver.disconnect();
    }

    this.infiniteScrollObserver = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting) {
          this.logger.debug("Sentinel visible, sending load_more action");
          this.context.send({ action: "load_more" });
        }
      },
      {
        rootMargin: "200px",
      }
    );

    this.infiniteScrollObserver.observe(sentinel);
    this.logger.debug("Observer set up successfully");
  }

  setupInfiniteScrollMutationObserver(): void {
    const wrapperElement = this.context.getWrapperElement();
    if (!wrapperElement) return;

    if (this.mutationObserver) {
      this.mutationObserver.disconnect();
    }

    this.mutationObserver = new MutationObserver(() => {
      this.setupInfiniteScrollObserver();
    });

    this.mutationObserver.observe(wrapperElement, {
      childList: true,
      subtree: true,
    });

    this.logger.debug("MutationObserver set up successfully");
  }

  teardown(): void {
    if (this.infiniteScrollObserver) {
      this.infiniteScrollObserver.disconnect();
      this.infiniteScrollObserver = null;
    }
    if (this.mutationObserver) {
      this.mutationObserver.disconnect();
      this.mutationObserver = null;
    }
  }
}
