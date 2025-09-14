// Jest setup file
import '@testing-library/jest-dom';

// Mock WebSocket globally
global.WebSocket = jest.fn();

// Mock console methods to reduce noise in tests
global.console = {
  ...console,
  log: jest.fn(),
  warn: jest.fn(),
  error: jest.fn(),
  debug: jest.fn(),
};