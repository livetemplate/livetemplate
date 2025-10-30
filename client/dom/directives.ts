/**
 * Apply scroll directives on elements with lvt-scroll attributes.
 */
export function handleScrollDirectives(rootElement: Element): void {
  const scrollElements = rootElement.querySelectorAll("[lvt-scroll]");

  scrollElements.forEach((element) => {
    const htmlElement = element as HTMLElement;
    const mode = htmlElement.getAttribute("lvt-scroll");
    const behavior =
      (htmlElement.getAttribute("lvt-scroll-behavior") as ScrollBehavior) ||
      "auto";
    const threshold = parseInt(
      htmlElement.getAttribute("lvt-scroll-threshold") || "100",
      10
    );

    if (!mode) return;

    switch (mode) {
      case "bottom":
        htmlElement.scrollTo({
          top: htmlElement.scrollHeight,
          behavior,
        });
        break;

      case "bottom-sticky": {
        const isNearBottom =
          htmlElement.scrollHeight -
            htmlElement.scrollTop -
            htmlElement.clientHeight <=
          threshold;
        if (isNearBottom) {
          htmlElement.scrollTo({
            top: htmlElement.scrollHeight,
            behavior,
          });
        }
        break;
      }

      case "top":
        htmlElement.scrollTo({
          top: 0,
          behavior,
        });
        break;

      case "preserve":
        break;

      default:
        console.warn(`Unknown lvt-scroll mode: ${mode}`);
    }
  });
}

/**
 * Apply highlight directives to elements with lvt-highlight attributes.
 */
export function handleHighlightDirectives(rootElement: Element): void {
  const highlightElements = rootElement.querySelectorAll("[lvt-highlight]");

  highlightElements.forEach((element) => {
    const mode = element.getAttribute("lvt-highlight");
    const duration = parseInt(
      element.getAttribute("lvt-highlight-duration") || "500",
      10
    );
    const color = element.getAttribute("lvt-highlight-color") || "#ffc107";

    if (!mode) return;

    const htmlElement = element as HTMLElement;
    const originalBackground = htmlElement.style.backgroundColor;
    const originalTransition = htmlElement.style.transition;

    htmlElement.style.transition = `background-color ${duration}ms ease-out`;
    htmlElement.style.backgroundColor = color;

    setTimeout(() => {
      htmlElement.style.backgroundColor = originalBackground;

      setTimeout(() => {
        htmlElement.style.transition = originalTransition;
      }, duration);
    }, 50);
  });
}

/**
 * Apply animation directives to elements with lvt-animate attributes.
 */
export function handleAnimateDirectives(rootElement: Element): void {
  const animateElements = rootElement.querySelectorAll("[lvt-animate]");

  animateElements.forEach((element) => {
    const animation = element.getAttribute("lvt-animate");
    const duration = parseInt(
      element.getAttribute("lvt-animate-duration") || "300",
      10
    );

    if (!animation) return;

    const htmlElement = element as HTMLElement;

    htmlElement.style.setProperty("--lvt-animate-duration", `${duration}ms`);

    switch (animation) {
      case "fade":
        htmlElement.style.animation = `lvt-fade-in var(--lvt-animate-duration) ease-out`;
        break;

      case "slide":
        htmlElement.style.animation = `lvt-slide-in var(--lvt-animate-duration) ease-out`;
        break;

      case "scale":
        htmlElement.style.animation = `lvt-scale-in var(--lvt-animate-duration) ease-out`;
        break;

      default:
        console.warn(`Unknown lvt-animate mode: ${animation}`);
    }

    htmlElement.addEventListener(
      "animationend",
      () => {
        htmlElement.style.animation = "";
      },
      { once: true }
    );
  });

  if (!document.getElementById("lvt-animate-styles")) {
    const style = document.createElement("style");
    style.id = "lvt-animate-styles";
    style.textContent = `
      @keyframes lvt-fade-in {
        from { opacity: 0; }
        to { opacity: 1; }
      }
      @keyframes lvt-slide-in {
        from {
          opacity: 0;
          transform: translateY(-10px);
        }
        to {
          opacity: 1;
          transform: translateY(0);
        }
      }
      @keyframes lvt-scale-in {
        from {
          opacity: 0;
          transform: scale(0.95);
        }
        to {
          opacity: 1;
          transform: scale(1);
        }
      }
    `;
    document.head.appendChild(style);
  }
}
