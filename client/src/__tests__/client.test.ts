import { StateTemplateClient } from '../client.js';
import { UpdateError } from '../types.js';
import type { RealtimeUpdate } from '../types.js';
import morphdom from 'morphdom';

// Mock morphdom
jest.mock('morphdom', () => jest.fn());

const mockMorphdom = morphdom as jest.MockedFunction<typeof morphdom>;

describe('StateTemplateClient', () => {
  let client: StateTemplateClient;

  beforeEach(() => {
    client = new StateTemplateClient();
    document.body.innerHTML = '';
    mockMorphdom.mockClear();
    mockMorphdom.mockImplementation((fromNode: Element, toNode: Element) => {
      fromNode.innerHTML = toNode.innerHTML;
      return fromNode;
    });
  });

  describe('constructor', () => {
    it('should create client with default config', () => {
      const newClient = new StateTemplateClient();
      expect(newClient).toBeInstanceOf(StateTemplateClient);
    });

    it('should create client with custom config', () => {
      const config = { debug: true };
      const newClient = new StateTemplateClient(config);
      expect(newClient).toBeInstanceOf(StateTemplateClient);
    });
  });

  describe('applyUpdate', () => {
    beforeEach(() => {
      document.body.innerHTML = '<div id="test-fragment">Original Content</div>';
    });

    it('should successfully replace element content', async () => {
      const update: RealtimeUpdate = {
        fragment_id: 'test-fragment',
        html: '<div id="test-fragment">New Content</div>',
        action: 'replace'
      };

      const result = await client.applyUpdate(update);

      expect(result.success).toBe(true);
      expect(result.fragmentId).toBe('test-fragment');
      expect(result.action).toBe('replace');
      expect(mockMorphdom).toHaveBeenCalled();
    });

    it('should successfully append content to element', async () => {
      const update: RealtimeUpdate = {
        fragment_id: 'test-fragment',
        html: '<p>Appended content</p>',
        action: 'append'
      };

      const result = await client.applyUpdate(update);

      expect(result.success).toBe(true);
      expect(result.fragmentId).toBe('test-fragment');
      expect(result.action).toBe('append');
      
      const element = document.getElementById('test-fragment');
      expect(element?.innerHTML).toContain('Appended content');
    });

    it('should successfully prepend content to element', async () => {
      const update: RealtimeUpdate = {
        fragment_id: 'test-fragment',
        html: '<p>Prepended content</p>',
        action: 'prepend'
      };

      const result = await client.applyUpdate(update);

      expect(result.success).toBe(true);
      expect(result.fragmentId).toBe('test-fragment');
      expect(result.action).toBe('prepend');
      
      const element = document.getElementById('test-fragment');
      expect(element?.innerHTML).toContain('Prepended content');
    });

    it('should successfully remove element', async () => {
      const update: RealtimeUpdate = {
        fragment_id: 'test-fragment',
        html: '',
        action: 'remove'
      };

      const result = await client.applyUpdate(update);

      expect(result.success).toBe(true);
      expect(result.fragmentId).toBe('test-fragment');
      expect(result.action).toBe('remove');
      
      const element = document.getElementById('test-fragment');
      expect(element).toBeNull();
    });

    it('should fail when element is not found', async () => {
      const update: RealtimeUpdate = {
        fragment_id: 'non-existent',
        html: '<div>New Content</div>',
        action: 'replace'
      };

      const result = await client.applyUpdate(update);

      expect(result.success).toBe(false);
      expect(result.error).toBeInstanceOf(UpdateError);
      expect(result.error?.message).toContain('not found');
    });

    it('should fail with invalid action', async () => {
      const update: RealtimeUpdate = {
        fragment_id: 'test-fragment',
        html: '<div>New Content</div>',
        action: 'invalid-action'
      };

      const result = await client.applyUpdate(update);

      expect(result.success).toBe(false);
      expect(result.error).toBeInstanceOf(UpdateError);
      expect(result.error?.message).toContain('Unsupported action');
    });

    it('should fail with missing fragment_id', async () => {
      const update: RealtimeUpdate = {
        fragment_id: '',
        html: '<div>New Content</div>',
        action: 'replace'
      };

      const result = await client.applyUpdate(update);

      expect(result.success).toBe(false);
      expect(result.error).toBeInstanceOf(UpdateError);
      expect(result.error?.message).toContain('fragment_id is required');
    });

    it('should fail with missing html for non-remove actions', async () => {
      const update: RealtimeUpdate = {
        fragment_id: 'test-fragment',
        html: '',
        action: 'replace'
      };

      const result = await client.applyUpdate(update);

      expect(result.success).toBe(false);
      expect(result.error).toBeInstanceOf(UpdateError);
      expect(result.error?.message).toContain('html is required');
    });

    it('should find element by data-fragment-id attribute', async () => {
      document.body.innerHTML = '<div data-fragment-id="test-attr">Original Content</div>';
      
      const update: RealtimeUpdate = {
        fragment_id: 'test-attr',
        html: '<div data-fragment-id="test-attr">New Content</div>',
        action: 'replace'
      };

      const result = await client.applyUpdate(update);

      expect(result.success).toBe(true);
      expect(mockMorphdom).toHaveBeenCalled();
    });
  });

  describe('applyUpdates', () => {
    beforeEach(() => {
      document.body.innerHTML = `
        <div id="fragment-1">Content 1</div>
        <div id="fragment-2">Content 2</div>
      `;
    });

    it('should apply multiple updates successfully', async () => {
      const updates: RealtimeUpdate[] = [
        {
          fragment_id: 'fragment-1',
          html: '<div id="fragment-1">Updated 1</div>',
          action: 'replace'
        },
        {
          fragment_id: 'fragment-2',
          html: '<div id="fragment-2">Updated 2</div>',
          action: 'replace'
        }
      ];

      const results = await client.applyUpdates(updates);

      expect(results).toHaveLength(2);
      expect(results[0].success).toBe(true);
      expect(results[1].success).toBe(true);
    });

    it('should stop on first failure when debug is false', async () => {
      const updates: RealtimeUpdate[] = [
        {
          fragment_id: 'non-existent',
          html: '<div>New Content</div>',
          action: 'replace'
        },
        {
          fragment_id: 'fragment-2',
          html: '<div id="fragment-2">Updated 2</div>',
          action: 'replace'
        }
      ];

      const results = await client.applyUpdates(updates);

      expect(results).toHaveLength(1);
      expect(results[0].success).toBe(false);
    });

    it('should continue on failure when debug is true', async () => {
      const debugClient = new StateTemplateClient({ debug: true });
      const updates: RealtimeUpdate[] = [
        {
          fragment_id: 'non-existent',
          html: '<div>New Content</div>',
          action: 'replace'
        },
        {
          fragment_id: 'fragment-2',
          html: '<div id="fragment-2">Updated 2</div>',
          action: 'replace'
        }
      ];

      const results = await debugClient.applyUpdates(updates);

      expect(results).toHaveLength(2);
      expect(results[0].success).toBe(false);
      expect(results[1].success).toBe(true);
    });
  });

  describe('setInitialContent', () => {
    it('should set initial content in default container', () => {
      document.body.innerHTML = '<div id="app"></div>';
      const html = '<h1>Initial Content</h1>';

      client.setInitialContent(html);

      const container = document.getElementById('app');
      expect(container?.innerHTML).toBe(html);
    });

    it('should set initial content in custom container', () => {
      document.body.innerHTML = '<div id="custom-container"></div>';
      const html = '<h1>Initial Content</h1>';

      client.setInitialContent(html, 'custom-container');

      const container = document.getElementById('custom-container');
      expect(container?.innerHTML).toBe(html);
    });

    it('should throw error if container not found', () => {
      expect(() => {
        client.setInitialContent('<h1>Content</h1>', 'non-existent');
      }).toThrow('Container element with ID \'non-existent\' not found');
    });
  });

  describe('hasElement', () => {
    beforeEach(() => {
      document.body.innerHTML = `
        <div id="existing-element">Content</div>
        <div data-fragment-id="attr-element">Content</div>
      `;
    });

    it('should return true for existing element by ID', () => {
      expect(client.hasElement('existing-element')).toBe(true);
    });

    it('should return true for existing element by data attribute', () => {
      expect(client.hasElement('attr-element')).toBe(true);
    });

    it('should return false for non-existing element', () => {
      expect(client.hasElement('non-existent')).toBe(false);
    });
  });
});
