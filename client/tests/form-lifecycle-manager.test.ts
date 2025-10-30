import { FormLifecycleManager } from "../state/form-lifecycle-manager";
import { ModalManager } from "../dom/modal-manager";
import { createLogger } from "../utils/logger";
import type { ResponseMetadata } from "../types";

describe("FormLifecycleManager", () => {
  let modalCloseSpy: jest.SpyInstance;
  let modalManager: ModalManager;

  beforeEach(() => {
    modalManager = new ModalManager(
      createLogger({ scope: "ModalManagerTest", level: "silent" })
    );
    modalCloseSpy = jest
      .spyOn(modalManager, "close")
      .mockImplementation(() => {});
    document.body.innerHTML = "";
  });

  afterEach(() => {
    modalCloseSpy.mockRestore();
    document.body.innerHTML = "";
  });

  const createForm = () => {
    const modal = document.createElement("div");
    modal.id = "modal-1";
    modal.setAttribute("role", "dialog");

    const form = document.createElement("form");
    modal.appendChild(form);

    const button = document.createElement("button");
    button.textContent = "Submit";
    form.appendChild(button);

    document.body.appendChild(modal);

    return { form, button, modal };
  };

  it("dispatches success events, resets the form, and closes the modal on success", () => {
    const { form, button, modal } = createForm();
    const manager = new FormLifecycleManager(modalManager);

    const doneListener = jest.fn();
    const successListener = jest.fn();
    form.addEventListener("lvt:done", doneListener);
    form.addEventListener("lvt:success", successListener);

    manager.setActiveSubmission(form, button, "Submit");

    const metadata: ResponseMetadata = { success: true, errors: {} };
    manager.handleResponse(metadata);

    expect(doneListener).toHaveBeenCalledWith(expect.any(CustomEvent));
    expect(successListener).toHaveBeenCalledWith(expect.any(CustomEvent));
    expect(modalCloseSpy).toHaveBeenCalledWith("modal-1");
    expect(form.elements.length).toBeGreaterThan(0);
    expect(button.disabled).toBe(false);
    expect(button.textContent).toBe("Submit");
  });

  it("respects lvt-preserve and keeps the fields intact", () => {
    const { form, button } = createForm();
    form.setAttribute("lvt-preserve", "");

    const input = document.createElement("input");
    input.value = "Keep me";
    form.appendChild(input);

    const manager = new FormLifecycleManager(modalManager);
    manager.setActiveSubmission(form, button, "Submit");

    const metadata: ResponseMetadata = { success: true, errors: {} };
    manager.handleResponse(metadata);

    expect(input.value).toBe("Keep me");
  });

  it("dispatches error events and keeps the form when the response fails", () => {
    const { form, button } = createForm();
    const manager = new FormLifecycleManager(modalManager);

    const doneListener = jest.fn();
    const errorListener = jest.fn();
    form.addEventListener("lvt:done", doneListener);
    form.addEventListener("lvt:error", errorListener);

    manager.setActiveSubmission(form, button, "Submit");

    const metadata: ResponseMetadata = {
      success: false,
      errors: { title: "Required" },
    };
    manager.handleResponse(metadata);

    expect(doneListener).toHaveBeenCalledWith(expect.any(CustomEvent));
    expect(errorListener).toHaveBeenCalledWith(expect.any(CustomEvent));
    expect(modalCloseSpy).not.toHaveBeenCalled();
    expect(button.disabled).toBe(false);
    expect(button.textContent).toBe("Submit");
  });

  it("reset simply clears active submission state", () => {
    const { form, button } = createForm();
    const manager = new FormLifecycleManager(modalManager);

    manager.setActiveSubmission(form, button, "Submit");
    manager.reset();

    const metadata: ResponseMetadata = { success: true, errors: {} };
    manager.handleResponse(metadata);

    expect(modalCloseSpy).not.toHaveBeenCalled();
  });
});
