import { FocusManager } from "../dom/focus-manager";
import { createLogger } from "../utils/logger";

describe("FocusManager", () => {
  beforeEach(() => {
    document.body.innerHTML = "";
  });

  afterEach(() => {
    document.body.innerHTML = "";
  });

  it("restores focus to the last tracked element", () => {
    const wrapper = document.createElement("div");
    wrapper.setAttribute("data-lvt-id", "focus-wrapper");

    const input = document.createElement("input");
    input.type = "text";
    wrapper.appendChild(input);

    document.body.appendChild(wrapper);

    const manager = new FocusManager(
      createLogger({ scope: "FocusManagerTest", level: "silent" })
    );
    manager.attach(wrapper);
    manager.updateFocusableElements();

    (manager as any).lastFocusedElement = input;
    (manager as any).wrapperElement = wrapper;
    (manager as any).lastFocusedSelectionStart = 1;
    (manager as any).lastFocusedSelectionEnd = 3;

    manager.restoreFocusedElement();

    expect(document.activeElement).toBe(input);
  });

  it("fails gracefully when the tracked element no longer exists", () => {
    const wrapper = document.createElement("div");
    wrapper.setAttribute("data-lvt-id", "focus-wrapper-2");

    const input = document.createElement("input");
    wrapper.appendChild(input);
    document.body.appendChild(wrapper);

    const manager = new FocusManager(
      createLogger({ scope: "FocusManagerTest", level: "silent" })
    );
    manager.attach(wrapper);
    manager.updateFocusableElements();

    (manager as any).lastFocusedElement = input;
    (manager as any).wrapperElement = wrapper;

    input.remove();

    expect(() => manager.restoreFocusedElement()).not.toThrow();
    expect(document.activeElement).not.toBe(input);
  });
});
