export type LogLevel = "silent" | "error" | "warn" | "info" | "debug";

const levelPriority: Record<LogLevel, number> = {
  silent: 0,
  error: 1,
  warn: 2,
  info: 3,
  debug: 4,
};

interface LogState {
  level: LogLevel;
}

type ConsoleMethod = "error" | "warn" | "info" | "debug";

const DEFAULT_SCOPE = "LiveTemplate";

/**
 * Lightweight console logger with support for log levels and scoped prefixes.
 */
export class Logger {
  constructor(
    private readonly state: LogState,
    private readonly scope: string[] = [],
    private readonly sink: Console = console
  ) {}

  setLevel(level: LogLevel): void {
    this.state.level = level;
  }

  getLevel(): LogLevel {
    return this.state.level;
  }

  child(scope: string): Logger {
    return new Logger(this.state, [...this.scope, scope], this.sink);
  }

  isDebugEnabled(): boolean {
    return this.shouldLog("debug");
  }

  error(...args: unknown[]): void {
    this.log("error", "error", args);
  }

  warn(...args: unknown[]): void {
    this.log("warn", "warn", args);
  }

  info(...args: unknown[]): void {
    this.log("info", "info", args);
  }

  debug(...args: unknown[]): void {
    this.log("debug", "debug", args);
  }

  private log(level: LogLevel, method: ConsoleMethod, args: unknown[]): void {
    if (!this.shouldLog(level)) {
      return;
    }

    const target =
      (this.sink[method] as (...args: unknown[]) => void) ||
      (console[method] as (...args: unknown[]) => void) ||
      console.log;
    target.apply(this.sink, [this.formatPrefix(), ...args]);
  }

  private shouldLog(level: LogLevel): boolean {
    return levelPriority[level] <= levelPriority[this.state.level];
  }

  private formatPrefix(): string {
    if (this.scope.length === 0) {
      return `[${DEFAULT_SCOPE}]`;
    }

    return `[${DEFAULT_SCOPE}:${this.scope.join(":")}]`;
  }
}

export interface LoggerOptions {
  level?: LogLevel;
  scope?: string | string[];
  sink?: Console;
}

export function createLogger(options: LoggerOptions = {}): Logger {
  const state: LogState = {
    level: options.level ?? "info",
  };

  const scope = Array.isArray(options.scope)
    ? options.scope
    : options.scope
    ? [options.scope]
    : [];

  return new Logger(state, scope, options.sink ?? console);
}
