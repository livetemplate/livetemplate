import { createClient, initializeGlobalClient, applyUpdate, setInitialContent, getGlobalClient, resetGlobalClient } from '../utils';
import { StateTemplateClient } from '../client';
import { RealtimeUpdate } from '../types';

// Mock the client module
jest.mock('../client');

describe('Utils', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    document.body.innerHTML = '';
    resetGlobalClient();
  });

  describe('createClient', () => {
    it('should create a new StateTemplateClient instance', () => {
      const client = createClient();
      expect(StateTemplateClient).toHaveBeenCalled();
      expect(client).toBeInstanceOf(StateTemplateClient);
    });

    it('should create client with custom config', () => {
      const config = { debug: true };
      const client = createClient(config);
      expect(StateTemplateClient).toHaveBeenCalledWith(config);
      expect(client).toBeInstanceOf(StateTemplateClient);
    });
  });

  describe('Global client functions', () => {
    let mockClient: jest.Mocked<StateTemplateClient>;

    beforeEach(() => {
      mockClient = {
        applyUpdate: jest.fn(),
        setInitialContent: jest.fn(),
        applyUpdates: jest.fn(),
        hasElement: jest.fn()
      } as any;

      (StateTemplateClient as jest.MockedClass<typeof StateTemplateClient>).mockImplementation(() => mockClient);
    });

    describe('initializeGlobalClient', () => {
      it('should create and set global client', () => {
        const client = initializeGlobalClient();
        expect(client).toBe(mockClient);
        expect(getGlobalClient()).toBe(mockClient);
      });

      it('should create global client with config', () => {
        const config = { debug: true };
        const client = initializeGlobalClient(config);
        expect(StateTemplateClient).toHaveBeenCalledWith(config);
        expect(client).toBe(mockClient);
      });
    });

    describe('applyUpdate', () => {
      it('should apply update using global client', async () => {
        initializeGlobalClient();
        mockClient.applyUpdate.mockResolvedValue({ success: true, fragmentId: 'test', action: 'replace' });

        const update: RealtimeUpdate = {
          fragment_id: 'test',
          html: '<div>New Content</div>',
          action: 'replace'
        };

        await applyUpdate(update);

        expect(mockClient.applyUpdate).toHaveBeenCalledWith(update);
      });

      it('should throw error if global client not initialized', async () => {
        const update: RealtimeUpdate = {
          fragment_id: 'test',
          html: '<div>New Content</div>',
          action: 'replace'
        };

        await expect(applyUpdate(update)).rejects.toThrow('Global client not initialized');
      });

      it('should throw error if update fails', async () => {
        initializeGlobalClient();
        const error = new Error('Update failed');
        mockClient.applyUpdate.mockResolvedValue({ success: false, fragmentId: 'test', action: 'replace', error });

        const update: RealtimeUpdate = {
          fragment_id: 'test',
          html: '<div>New Content</div>',
          action: 'replace'
        };

        await expect(applyUpdate(update)).rejects.toThrow('Update failed');
      });
    });

    describe('setInitialContent', () => {
      it('should set initial content using global client', () => {
        initializeGlobalClient();
        const html = '<h1>Initial Content</h1>';

        setInitialContent(html);

        expect(mockClient.setInitialContent).toHaveBeenCalledWith(html, undefined);
      });

      it('should set initial content with custom container', () => {
        initializeGlobalClient();
        const html = '<h1>Initial Content</h1>';
        const containerId = 'custom-container';

        setInitialContent(html, containerId);

        expect(mockClient.setInitialContent).toHaveBeenCalledWith(html, containerId);
      });

      it('should throw error if global client not initialized', () => {
        expect(() => {
          setInitialContent('<h1>Content</h1>');
        }).toThrow('Global client not initialized');
      });
    });

    describe('getGlobalClient', () => {
      it('should return null when no global client set', () => {
        expect(getGlobalClient()).toBeNull();
      });

      it('should return global client when set', () => {
        const client = initializeGlobalClient();
        expect(getGlobalClient()).toBe(client);
      });
    });
  });
});
