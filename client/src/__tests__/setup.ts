/**
 * Test environment setup for StateTemplate client
 */

// Test configuration
process.env.NODE_ENV = 'test';

// Ensure this file is treated as a test file with at least one test
describe('Test Environment Setup', () => {
  it('should have NODE_ENV set to test', () => {
    expect(process.env.NODE_ENV).toBe('test');
  });

  it('should have jest available', () => {
    expect(jest).toBeDefined();
    expect(expect).toBeDefined();
  });

  it('should support async/await in tests', async () => {
    const promise = Promise.resolve('test');
    const result = await promise;
    expect(result).toBe('test');
  });
});