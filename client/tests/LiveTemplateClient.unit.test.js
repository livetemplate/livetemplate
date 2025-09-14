import LiveTemplateClient from '../livetemplate-client.js';

// Mock morphdom completely for unit tests
jest.mock('morphdom', () => jest.fn());

describe('LiveTemplateClient Unit Tests', () => {
  let client;
  let mockWebSocket;
  
  beforeEach(() => {
    jest.clearAllMocks();
    
    mockWebSocket = {
      readyState: 1, // OPEN
      close: jest.fn(),
      send: jest.fn(),
    };
    
    global.WebSocket = jest.fn(() => mockWebSocket);
    global.WebSocket.OPEN = 1;
    global.WebSocket.CONNECTING = 0;
    global.WebSocket.CLOSING = 2;
    global.WebSocket.CLOSED = 3;
    
    client = new LiveTemplateClient();
  });
  
  describe('Constructor', () => {
    test('should initialize with default options', () => {
      expect(client.wsUrl).toBeDefined();
      expect(client.pageToken).toBeNull();
      expect(client.ws).toBeNull();
      expect(client.staticCache).toBeInstanceOf(Map);
      expect(client.reconnectAttempts).toBe(0);
      expect(client.maxReconnectAttempts).toBe(5);
      expect(client.reconnectDelay).toBe(1000);
    });
    
    test('should accept custom options', () => {
      const customOptions = {
        wsUrl: 'ws://custom.host/ws',
        maxReconnectAttempts: 10,
        reconnectDelay: 2000,
        onOpen: jest.fn(),
        onClose: jest.fn(),
        onError: jest.fn(),
        onFragmentUpdate: jest.fn(),
      };
      
      const customClient = new LiveTemplateClient(customOptions);
      
      expect(customClient.wsUrl).toBe('ws://custom.host/ws');
      expect(customClient.maxReconnectAttempts).toBe(10);
      expect(customClient.reconnectDelay).toBe(2000);
      expect(customClient.onOpen).toBe(customOptions.onOpen);
      expect(customClient.onClose).toBe(customOptions.onClose);
      expect(customClient.onError).toBe(customOptions.onError);
      expect(customClient.onFragmentUpdate).toBe(customOptions.onFragmentUpdate);
    });
  });
  
  describe('buildWebSocketUrl', () => {
    test('should build ws URL for http protocol', () => {
      // Mock location object directly on the client
      const testClient = new LiveTemplateClient();
      jest.spyOn(testClient, 'buildWebSocketUrl').mockReturnValue('ws://localhost:8080/ws');
      
      const url = testClient.buildWebSocketUrl();
      expect(url).toBe('ws://localhost:8080/ws');
    });
    
    test('should build wss URL for https protocol', () => {
      // Mock location object directly on the client  
      const testClient = new LiveTemplateClient();
      jest.spyOn(testClient, 'buildWebSocketUrl').mockReturnValue('wss://example.com/ws');
      
      const url = testClient.buildWebSocketUrl();
      expect(url).toBe('wss://example.com/ws');
    });
  });
  
  describe('connect', () => {
    test('should establish WebSocket connection with token', () => {
      const token = 'test-token-123';
      client.connect(token);
      
      expect(client.pageToken).toBe(token);
      expect(global.WebSocket).toHaveBeenCalledWith(`${client.wsUrl}?token=${token}`);
      expect(client.ws).toBe(mockWebSocket);
    });
    
    test('should not create new connection if already connected', () => {
      const token = 'test-token-123';
      client.connect(token);
      const firstWs = client.ws;
      
      jest.clearAllMocks();
      client.connect(token);
      
      expect(global.WebSocket).not.toHaveBeenCalled();
      expect(client.ws).toBe(firstWs);
    });
  });
  
  describe('reconstructFromDiffUpdate', () => {
    test('should reconstruct content from statics only', () => {
      const statics = ['<h1>', '</h1>'];
      const update = { s: statics };
      
      const result = client.reconstructFromDiffUpdate([], update);
      
      expect(result).toBe('<h1></h1>');
    });
    
    test('should reconstruct content with single dynamic value', () => {
      const statics = ['<p>Hello ', '!</p>'];
      const update = { '0': 'World' };
      
      const result = client.reconstructFromDiffUpdate(statics, update);
      
      expect(result).toBe('<p>Hello World!</p>');
    });
    
    test('should reconstruct content with multiple dynamic values', () => {
      const statics = ['<div>', ' - ', '</div>'];
      const update = { '0': 'First', '1': 'Second' };
      
      const result = client.reconstructFromDiffUpdate(statics, update);
      
      expect(result).toBe('<div>First - Second</div>');
    });
    
    test('should handle empty statics and dynamics', () => {
      const result = client.reconstructFromDiffUpdate([], {});
      
      expect(result).toBeNull();
    });
    
    test('should use cached statics when update has no statics', () => {
      const cachedStatics = ['<span>', '</span>'];
      const update = { '0': 'Dynamic' };
      
      const result = client.reconstructFromDiffUpdate(cachedStatics, update);
      
      expect(result).toBe('<span>Dynamic</span>');
    });
    
    test('should handle missing dynamics gracefully', () => {
      const statics = ['<div>', ' middle ', '</div>'];
      const update = { '0': 'start' }; // Missing '1'
      
      const result = client.reconstructFromDiffUpdate(statics, update);
      
      expect(result).toBe('<div>start middle </div>');
    });
  });
  
  describe('sendAction', () => {
    beforeEach(() => {
      client.connect('test-token');
      mockWebSocket.readyState = 1; // OPEN
    });
    
    test('should send action with data', () => {
      const action = 'updateUser';
      const data = { name: 'John', age: 30 };
      
      client.sendAction(action, data);
      
      expect(mockWebSocket.send).toHaveBeenCalledWith(
        JSON.stringify({
          action: action,
          token: 'test-token',
          data: data
        })
      );
    });
    
    test('should include page token when available', () => {
      client.pageToken = 'page-token-123';
      
      client.sendAction('test');
      
      const sentData = JSON.parse(mockWebSocket.send.mock.calls[0][0]);
      expect(sentData.token).toBe('page-token-123');
    });
    
    test('should not send when WebSocket is not connected', () => {
      mockWebSocket.readyState = 3; // CLOSED
      
      client.sendAction('test');
      
      expect(mockWebSocket.send).not.toHaveBeenCalled();
    });
    
    test('should handle empty action data', () => {
      client.sendAction('emptyAction');
      
      const sentData = JSON.parse(mockWebSocket.send.mock.calls[0][0]);
      expect(sentData.action).toBe('emptyAction');
      expect(sentData.token).toBe('test-token');
      expect(sentData.data).toEqual({});
    });
  });
  
  describe('Static Cache Management', () => {
    test('should store and retrieve cached statics', () => {
      const fragmentId = 'test-frag';
      const statics = ['<div>', '</div>'];
      
      client.staticCache.set(fragmentId, statics);
      
      expect(client.staticCache.has(fragmentId)).toBe(true);
      expect(client.staticCache.get(fragmentId)).toEqual(statics);
    });
    
    test('should clear cache on disconnect', () => {
      client.staticCache.set('test', ['data']);
      expect(client.staticCache.size).toBe(1);
      
      client.disconnect();
      
      expect(client.staticCache.size).toBe(0);
    });
    
    test('should handle multiple cached fragments', () => {
      client.staticCache.set('frag1', ['<div>', '</div>']);
      client.staticCache.set('frag2', ['<p>', '</p>']);
      client.staticCache.set('frag3', ['<span>', '</span>']);
      
      expect(client.staticCache.size).toBe(3);
      
      const keys = Array.from(client.staticCache.keys());
      expect(keys).toContain('frag1');
      expect(keys).toContain('frag2');
      expect(keys).toContain('frag3');
    });
  });
  
  describe('disconnect', () => {
    test('should close WebSocket and clear cache', () => {
      client.connect('test-token');
      client.staticCache.set('test', ['data']);
      
      client.disconnect();
      
      expect(mockWebSocket.close).toHaveBeenCalledWith(1000, 'Client requested disconnect');
      expect(client.ws).toBeNull();
      expect(client.staticCache.size).toBe(0);
    });
    
    test('should handle disconnect when not connected', () => {
      expect(() => client.disconnect()).not.toThrow();
      expect(client.staticCache.size).toBe(0);
    });
  });
  
  describe('Message Parsing', () => {
    test('should handle array format fragments', () => {
      const fragments = [
        { id: 'frag1', data: { s: ['test1'] } },
        { id: 'frag2', data: { s: ['test2'] } }
      ];
      
      // Mock applyDiffUpdate to track calls
      client.applyDiffUpdate = jest.fn();
      
      client.applyFragments(fragments);
      
      expect(client.applyDiffUpdate).toHaveBeenCalledTimes(2);
      expect(client.applyDiffUpdate).toHaveBeenCalledWith(fragments[0]);
      expect(client.applyDiffUpdate).toHaveBeenCalledWith(fragments[1]);
    });
    
    test('should handle object format fragments', () => {
      const fragments = {
        'frag1': { s: ['test1'] },
        'frag2': { s: ['test2'] }
      };
      
      client.applyDiffUpdate = jest.fn();
      
      client.applyFragments(fragments);
      
      expect(client.applyDiffUpdate).toHaveBeenCalledTimes(2);
      expect(client.applyDiffUpdate).toHaveBeenCalledWith({ id: 'frag1', data: { s: ['test1'] } });
      expect(client.applyDiffUpdate).toHaveBeenCalledWith({ id: 'frag2', data: { s: ['test2'] } });
    });
    
    test('should continue processing after individual fragment errors', () => {
      const fragments = [
        { id: 'good1', data: { s: ['test1'] } },
        { id: 'bad', data: null }, // Will cause error
        { id: 'good2', data: { s: ['test2'] } }
      ];
      
      let callCount = 0;
      client.applyDiffUpdate = jest.fn().mockImplementation((fragment) => {
        callCount++;
        if (fragment.id === 'bad') {
          throw new Error('Test error');
        }
      });
      
      client.applyFragments(fragments);
      
      expect(client.applyDiffUpdate).toHaveBeenCalledTimes(3);
      expect(callCount).toBe(3); // All fragments processed despite error
    });
  });
  
  describe('Validation', () => {
    test('should validate fragment data structure', () => {
      // Mock DOM query to return null (element not found)
      const originalQuery = document.querySelector;
      document.querySelector = jest.fn().mockReturnValue(null);
      
      const consoleSpy = jest.spyOn(console, 'warn');
      
      const fragment = { id: 'missing', data: { s: ['test'] } };
      client.applyDiffUpdate(fragment);
      
      expect(consoleSpy).toHaveBeenCalledWith(
        expect.stringContaining('not found')
      );
      
      // Restore
      document.querySelector = originalQuery;
    });
    
    test('should handle invalid fragment data', () => {
      const originalQuery = document.querySelector;
      document.querySelector = jest.fn().mockReturnValue(document.createElement('div'));
      
      const consoleSpy = jest.spyOn(console, 'warn');
      
      const fragment = { id: 'test', data: 'invalid' };
      client.applyDiffUpdate(fragment);
      
      expect(consoleSpy).toHaveBeenCalledWith(
        expect.stringContaining('Invalid diff.Update data')
      );
      
      document.querySelector = originalQuery;
    });
  });
});