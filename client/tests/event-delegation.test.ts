import {
  EventDelegator,
  EventDelegationContext,
} from "../dom/event-delegation";
import { createLogger } from "../utils/logger";

describe("EventDelegator", () => {
  let consoleLogSpy: jest.SpyInstance;

  beforeEach(() => {
    consoleLogSpy = jest.spyOn(console, "log").mockImplementation(() => {});
    document.body.innerHTML = "";
  });

  afterEach(() => {
    consoleLogSpy.mockRestore();
    jest.useRealTimers();
    document.body.innerHTML = "";
  });

  const createContext = (
    wrapper: Element,
    overrides: Partial<EventDelegationContext> = {}
  ) => {
    const rateLimitedHandlers = new WeakMap<Element, Map<string, Function>>();

    const baseContext: EventDelegationContext = {
      getWrapperElement: () => wrapper,
      getRateLimitedHandlers: () => rateLimitedHandlers,
      parseValue: (value: string) => value,
      send: jest.fn(),
      setActiveSubmission: jest.fn(),
      openModal: jest.fn(),
      closeModal: jest.fn(),
      getWebSocketReadyState: () => 1,
    };

    return { ...baseContext, ...overrides } as EventDelegationContext & {
      send: jest.Mock;
      setActiveSubmission: jest.Mock;
    };
  };

  it("sends action payloads for delegated click events", () => {
    const wrapper = document.createElement("div");
    wrapper.setAttribute("data-lvt-id", "wrapper-1");
    wrapper.innerHTML = `
      <button id="save" lvt-click="save" lvt-data-id="42"></button>
    `;
    document.body.appendChild(wrapper);

    const context = createContext(wrapper);
    const delegator = new EventDelegator(
      context,
      createLogger({ scope: "EventDelegatorTest", level: "silent" })
    );
    delegator.setupEventDelegation();

    const button = document.getElementById("save")!;
    button.dispatchEvent(new MouseEvent("click", { bubbles: true }));

    expect(context.send).toHaveBeenCalledTimes(1);
    expect(context.send).toHaveBeenCalledWith({
      action: "save",
      data: { id: "42" },
    });
  });

  it("applies throttle semantics for rate limited handlers", () => {
    jest.useFakeTimers();

    const wrapper = document.createElement("div");
    wrapper.setAttribute("data-lvt-id", "wrapper-2");
    wrapper.innerHTML = `
      <button id="ping" lvt-click="ping" lvt-throttle="200"></button>
    `;
    document.body.appendChild(wrapper);

    const context = createContext(wrapper);
    const delegator = new EventDelegator(
      context,
      createLogger({ scope: "EventDelegatorTest", level: "silent" })
    );
    delegator.setupEventDelegation();

    const button = document.getElementById("ping")!;
    button.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    button.dispatchEvent(new MouseEvent("click", { bubbles: true }));

    expect(context.send).toHaveBeenCalledTimes(1);

    jest.advanceTimersByTime(200);
    button.dispatchEvent(new MouseEvent("click", { bubbles: true }));

    expect(context.send).toHaveBeenCalledTimes(2);
  });

  it("handles form submissions and records active submission state", () => {
    const wrapper = document.createElement("div");
    wrapper.setAttribute("data-lvt-id", "wrapper-3");
    wrapper.innerHTML = `
      <form id="add-form" lvt-submit="add">
        <input type="text" name="title" value="Hello" />
        <input type="checkbox" name="published" checked />
        <button type="submit" id="submit" lvt-disable-with="Saving...">Save</button>
      </form>
    `;
    document.body.appendChild(wrapper);

    const form = document.getElementById("add-form") as HTMLFormElement;
    const submitButton = document.getElementById("submit") as HTMLButtonElement;

    const context = createContext(wrapper);
    const delegator = new EventDelegator(
      context,
      createLogger({ scope: "EventDelegatorTest", level: "silent" })
    );
    delegator.setupEventDelegation();

    const submitEvent = new Event("submit", {
      bubbles: true,
      cancelable: true,
    }) as SubmitEvent & { submitter?: HTMLButtonElement };
    submitEvent.submitter = submitButton;

    form.dispatchEvent(submitEvent);

    expect(context.setActiveSubmission).toHaveBeenCalledWith(
      form,
      submitButton,
      "Save"
    );

    expect(submitButton.disabled).toBe(true);
    expect(submitButton.textContent).toBe("Saving...");

    expect(context.send).toHaveBeenCalledTimes(1);
    expect(context.send).toHaveBeenCalledWith(
      expect.objectContaining({
        action: "add",
        data: expect.objectContaining({
          title: "Hello",
          published: true,
        }),
      })
    );
  });
});
