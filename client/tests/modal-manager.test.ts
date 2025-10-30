import { ModalManager } from "../dom/modal-manager";
import { createLogger } from "../utils/logger";

describe("ModalManager", () => {
  let consoleLogSpy: jest.SpyInstance;
  let consoleWarnSpy: jest.SpyInstance;

  beforeEach(() => {
    consoleLogSpy = jest.spyOn(console, "log").mockImplementation(() => {});
    consoleWarnSpy = jest.spyOn(console, "warn").mockImplementation(() => {});
    document.body.innerHTML = "";
  });

  afterEach(() => {
    consoleLogSpy.mockRestore();
    consoleWarnSpy.mockRestore();
    jest.useRealTimers();
    document.body.innerHTML = "";
  });

  it("opens a modal and focuses the first input when no element is active", () => {
    jest.useFakeTimers();

    const modal = document.createElement("div");
    modal.id = "test-modal";
    modal.setAttribute("hidden", "");
    modal.style.display = "none";
    modal.setAttribute("aria-hidden", "true");

    const input = document.createElement("input");
    input.type = "text";
    modal.appendChild(input);

    document.body.appendChild(modal);

    const manager = new ModalManager(
      createLogger({ scope: "ModalManagerTest", level: "silent" })
    );
    const openedListener = jest.fn();
    modal.addEventListener("lvt:modal-opened", openedListener);

    manager.open("test-modal");

    expect(modal.hasAttribute("hidden")).toBe(false);
    expect(modal.style.display).toBe("flex");
    expect(modal.getAttribute("aria-hidden")).toBe("false");
    expect(openedListener).toHaveBeenCalled();

    jest.runAllTimers();

    expect(document.activeElement).toBe(input);
  });

  it("respects the current focus when a visible element inside the modal is already active", () => {
    jest.useFakeTimers();

    const modal = document.createElement("div");
    modal.id = "test-modal-visible";
    modal.setAttribute("hidden", "");
    modal.style.display = "none";
    modal.setAttribute("aria-hidden", "true");

    const firstInput = document.createElement("input");
    firstInput.type = "text";
    const secondInput = document.createElement("input");
    secondInput.type = "text";

    // Emulate layout visibility checks performed in ModalManager.
    Object.defineProperty(secondInput, "offsetParent", {
      get: () => modal,
    });
    secondInput.getClientRects = () => [{ width: 10, height: 10 }] as any;

    modal.appendChild(firstInput);
    modal.appendChild(secondInput);
    document.body.appendChild(modal);

    const manager = new ModalManager(
      createLogger({ scope: "ModalManagerTest", level: "silent" })
    );
    manager.open("test-modal-visible");

    secondInput.focus();

    jest.runAllTimers();

    expect(document.activeElement).toBe(secondInput);
  });

  it("closes a modal and emits the closed event", () => {
    const modal = document.createElement("div");
    modal.id = "test-modal-close";
    document.body.appendChild(modal);

    const manager = new ModalManager(
      createLogger({ scope: "ModalManagerTest", level: "silent" })
    );
    const closedListener = jest.fn();
    modal.addEventListener("lvt:modal-closed", closedListener);

    manager.close("test-modal-close");

    expect(modal.hasAttribute("hidden")).toBe(true);
    expect(modal.style.display).toBe("none");
    expect(modal.getAttribute("aria-hidden")).toBe("true");
    expect(closedListener).toHaveBeenCalled();
  });
});
