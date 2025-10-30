/**
 * Enables and disables all form controls inside the LiveTemplate wrapper.
 */
export class FormDisabler {
  disable(wrapper: Element | null): void {
    if (!wrapper) return;

    const forms = wrapper.querySelectorAll("form");
    forms.forEach((form) => {
      const inputs = form.querySelectorAll("input, textarea, select, button");
      inputs.forEach((input) => {
        (input as HTMLInputElement).disabled = true;
      });
    });
  }

  enable(wrapper: Element | null): void {
    if (!wrapper) return;

    const forms = wrapper.querySelectorAll("form");
    forms.forEach((form) => {
      const inputs = form.querySelectorAll("input, textarea, select, button");
      inputs.forEach((input) => {
        (input as HTMLInputElement).disabled = false;
      });
    });
  }
}
