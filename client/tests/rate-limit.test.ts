import { debounce, throttle } from "../utils/rate-limit";

describe("rate limit utilities", () => {
  beforeEach(() => {
    jest.useFakeTimers();
  });

  afterEach(() => {
    jest.useRealTimers();
  });

  it("debounce delays invocation until the wait period has elapsed", () => {
    const spy = jest.fn();
    const debounced = debounce(spy, 300);

    debounced();
    debounced();
    debounced();

    expect(spy).not.toHaveBeenCalled();

    jest.advanceTimersByTime(299);
    expect(spy).not.toHaveBeenCalled();

    jest.advanceTimersByTime(1);
    expect(spy).toHaveBeenCalledTimes(1);
  });

  it("throttle executes immediately and then rate limits subsequent calls", () => {
    const spy = jest.fn();
    const throttled = throttle(spy, 200);

    throttled();
    throttled();
    throttled();

    expect(spy).toHaveBeenCalledTimes(1);

    jest.advanceTimersByTime(199);
    throttled();
    expect(spy).toHaveBeenCalledTimes(1);

    jest.advanceTimersByTime(1);
    throttled();
    expect(spy).toHaveBeenCalledTimes(2);
  });
});
