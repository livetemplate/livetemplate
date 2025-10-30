/**
 * Handles showing and hiding the global LiveTemplate loading indicator.
 */
export class LoadingIndicator {
  private bar: HTMLElement | null = null;

  show(): void {
    if (this.bar) return;

    const bar = document.createElement("div");
    bar.style.cssText = `
      position: fixed;
      top: 0;
      left: 0;
      right: 0;
      height: 3px;
      background: linear-gradient(90deg, #3b82f6 0%, #60a5fa 50%, #3b82f6 100%);
      background-size: 200% 100%;
      z-index: 9999;
      animation: lvt-loading-shimmer 1.5s ease-in-out infinite;
    `;

    if (!document.getElementById("lvt-loading-styles")) {
      const style = document.createElement("style");
      style.id = "lvt-loading-styles";
      style.textContent = `
        @keyframes lvt-loading-shimmer {
          0% { background-position: 200% 0; }
          100% { background-position: -200% 0; }
        }
      `;
      document.head.appendChild(style);
    }

    document.body.insertBefore(bar, document.body.firstChild);
    this.bar = bar;
  }

  hide(): void {
    if (!this.bar) return;

    if (this.bar.parentNode) {
      this.bar.parentNode.removeChild(this.bar);
    }
    this.bar = null;
  }
}
