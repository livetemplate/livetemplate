import { FormDisabler } from "../dom/form-disabler";

describe("FormDisabler", () => {
  beforeEach(() => {
    document.body.innerHTML = "";
  });

  afterEach(() => {
    document.body.innerHTML = "";
  });

  it("disables and re-enables all controls within the wrapper", () => {
    const wrapper = document.createElement("div");

    const form = document.createElement("form");
    const input = document.createElement("input");
    const textarea = document.createElement("textarea");
    const select = document.createElement("select");
    const button = document.createElement("button");

    form.appendChild(input);
    form.appendChild(textarea);
    form.appendChild(select);
    form.appendChild(button);
    wrapper.appendChild(form);
    document.body.appendChild(wrapper);

    const disabler = new FormDisabler();
    disabler.disable(wrapper);

    [input, textarea, select, button].forEach((element) => {
      expect(element.disabled).toBe(true);
    });

    disabler.enable(wrapper);

    [input, textarea, select, button].forEach((element) => {
      expect(element.disabled).toBe(false);
    });
  });
});
