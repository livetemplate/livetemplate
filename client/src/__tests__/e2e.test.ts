/**
 * End-to-end tests for StateTemplateClient
 * These tests use a real DOM environment and morphdom
 */

import { StateTemplateClient } from '../client';
import { RealtimeUpdate } from '../types';

describe('StateTemplateClient E2E', () => {
  let client: StateTemplateClient;

  beforeEach(() => {
    client = new StateTemplateClient({ debug: true });
    document.body.innerHTML = '';
  });

  describe('Real-world usage scenarios', () => {
    it('should handle a complete app initialization and updates', async () => {
      // Set up initial page structure
      document.body.innerHTML = '<div id="app"></div>';
      
      const initialHTML = `
        <div class="dashboard">
          <div id="counter">Count: 0</div>
          <div id="status">Status: Ready</div>
          <div id="user-list">
            <h3>Users</h3>
            <ul id="users"></ul>
          </div>
        </div>
      `;

      client.setInitialContent(initialHTML);

      // Verify initial content
      const counterElement = document.getElementById('counter');
      expect(counterElement?.textContent).toBe('Count: 0');

      // Apply counter update
      const counterUpdate: RealtimeUpdate = {
        fragment_id: 'counter',
        html: '<div id="counter">Count: 42</div>',
        action: 'replace'
      };

      const result1 = await client.applyUpdate(counterUpdate);
      expect(result1.success).toBe(true);
      expect(counterElement?.textContent).toBe('Count: 42');

      // Apply status update
      const statusUpdate: RealtimeUpdate = {
        fragment_id: 'status',
        html: '<div id="status">Status: Active</div>',
        action: 'replace'
      };

      const result2 = await client.applyUpdate(statusUpdate);
      expect(result2.success).toBe(true);
      
      const statusElement = document.getElementById('status');
      expect(statusElement?.textContent).toBe('Status: Active');

      // Add user to list - target the ul element with a specific ID
      const addUserUpdate: RealtimeUpdate = {
        fragment_id: 'users',
        html: '<li>John Doe</li>',
        action: 'append'
      };

      const result3 = await client.applyUpdate(addUserUpdate);
      expect(result3.success).toBe(true);
      
      // The li should be appended to the ul element with id="users"
      const usersElement = document.getElementById('users');
      expect(usersElement?.children.length).toBe(1);
      expect(usersElement?.children[0].textContent).toBe('John Doe');
    });

    it('should handle batch updates for live dashboard', async () => {
      // Set up dashboard
      document.body.innerHTML = `
        <div id="app">
          <div id="metrics">
            <div id="cpu">CPU: 0%</div>
            <div id="memory">Memory: 0%</div>
            <div id="disk">Disk: 0%</div>
          </div>
          <div id="alerts"></div>
        </div>
      `;

      // Simulate real-time metrics update
      const updates: RealtimeUpdate[] = [
        {
          fragment_id: 'cpu',
          html: '<div id="cpu">CPU: 45%</div>',
          action: 'replace'
        },
        {
          fragment_id: 'memory',
          html: '<div id="memory">Memory: 62%</div>',
          action: 'replace'
        },
        {
          fragment_id: 'disk',
          html: '<div id="disk">Disk: 78%</div>',
          action: 'replace'
        },
        {
          fragment_id: 'alerts',
          html: '<div class="alert warning">High disk usage detected</div>',
          action: 'append'
        }
      ];

      const results = await client.applyUpdates(updates);

      // Verify all updates succeeded
      expect(results).toHaveLength(4);
      results.forEach(result => {
        expect(result.success).toBe(true);
      });

      // Verify content updates
      expect(document.getElementById('cpu')?.textContent).toBe('CPU: 45%');
      expect(document.getElementById('memory')?.textContent).toBe('Memory: 62%');
      expect(document.getElementById('disk')?.textContent).toBe('Disk: 78%');
      
      const alertsElement = document.getElementById('alerts');
      expect(alertsElement?.children.length).toBe(1);
      expect(alertsElement?.children[0].textContent).toBe('High disk usage detected');
    });

    it('should handle chat application updates', async () => {
      // Set up chat interface
      document.body.innerHTML = `
        <div id="app">
          <div id="chat-messages"></div>
          <div id="user-status">0 users online</div>
        </div>
      `;

      // Add first message
      const message1Update: RealtimeUpdate = {
        fragment_id: 'chat-messages',
        html: '<div class="message">Alice: Hello everyone!</div>',
        action: 'append'
      };

      const result1 = await client.applyUpdate(message1Update);
      expect(result1.success).toBe(true);

      // Add second message
      const message2Update: RealtimeUpdate = {
        fragment_id: 'chat-messages',
        html: '<div class="message">Bob: Hi Alice!</div>',
        action: 'append'
      };

      const result2 = await client.applyUpdate(message2Update);
      expect(result2.success).toBe(true);

      // Update user count
      const userStatusUpdate: RealtimeUpdate = {
        fragment_id: 'user-status',
        html: '<div id="user-status">2 users online</div>',
        action: 'replace'
      };

      const result3 = await client.applyUpdate(userStatusUpdate);
      expect(result3.success).toBe(true);

      // Verify chat state
      const chatMessages = document.getElementById('chat-messages');
      expect(chatMessages?.children.length).toBe(2);
      expect(chatMessages?.children[0].textContent).toBe('Alice: Hello everyone!');
      expect(chatMessages?.children[1].textContent).toBe('Bob: Hi Alice!');
      
      const userStatus = document.getElementById('user-status');
      expect(userStatus?.textContent).toBe('2 users online');
    });

    it('should handle element removal and recreation', async () => {
      // Set up initial content
      document.body.innerHTML = `
        <div id="app">
          <div id="notification">Important message</div>
          <div id="content">Main content</div>
        </div>
      `;

      // Remove notification
      const removeUpdate: RealtimeUpdate = {
        fragment_id: 'notification',
        html: '',
        action: 'remove'
      };

      const result1 = await client.applyUpdate(removeUpdate);
      expect(result1.success).toBe(true);
      expect(document.getElementById('notification')).toBeNull();

      // Add notification back (should fail since element doesn't exist)
      const addBackUpdate: RealtimeUpdate = {
        fragment_id: 'notification',
        html: '<div id="notification">New notification</div>',
        action: 'replace'
      };

      const result2 = await client.applyUpdate(addBackUpdate);
      expect(result2.success).toBe(false);
      expect(result2.error?.message).toContain('not found');
    });

    it('should preserve element attributes and data during updates', async () => {
      // Set up element with attributes
      document.body.innerHTML = `
        <div id="app">
          <div id="counter" class="metric" data-value="0" data-max="100">Count: 0</div>
        </div>
      `;

      const counterElement = document.getElementById('counter');
      expect(counterElement?.getAttribute('class')).toBe('metric');
      expect(counterElement?.getAttribute('data-value')).toBe('0');
      expect(counterElement?.getAttribute('data-max')).toBe('100');

      // Update content while preserving structure
      const update: RealtimeUpdate = {
        fragment_id: 'counter',
        html: '<div id="counter" class="metric active" data-value="50" data-max="100">Count: 50</div>',
        action: 'replace'
      };

      const result = await client.applyUpdate(update);
      expect(result.success).toBe(true);

      // Verify content and attributes updated
      const updatedElement = document.getElementById('counter');
      expect(updatedElement?.textContent).toBe('Count: 50');
      expect(updatedElement?.getAttribute('class')).toBe('metric active');
      expect(updatedElement?.getAttribute('data-value')).toBe('50');
      expect(updatedElement?.getAttribute('data-max')).toBe('100');
    });

    it('should handle complex nested HTML structures', async () => {
      // Set up complex nested structure
      document.body.innerHTML = `
        <div id="app">
          <div id="product-list">
            <div class="header">
              <h2>Products</h2>
              <span class="count">0 items</span>
            </div>
            <div class="items"></div>
          </div>
        </div>
      `;

      // Add first product
      const addProductUpdate: RealtimeUpdate = {
        fragment_id: 'product-list',
        html: `
          <div class="product" data-id="1">
            <h3>Laptop</h3>
            <p class="price">$999</p>
            <div class="tags">
              <span class="tag">Electronics</span>
              <span class="tag">Computers</span>
            </div>
          </div>
        `,
        action: 'append'
      };

      const result1 = await client.applyUpdate(addProductUpdate);
      expect(result1.success).toBe(true);

      // Update product count in header
      const updateHeaderUpdate: RealtimeUpdate = {
        fragment_id: 'product-list',
        html: `
          <div id="product-list">
            <div class="header">
              <h2>Products</h2>
              <span class="count">1 items</span>
            </div>
            <div class="items">
              <div class="product" data-id="1">
                <h3>Laptop</h3>
                <p class="price">$999</p>
                <div class="tags">
                  <span class="tag">Electronics</span>
                  <span class="tag">Computers</span>
                </div>
              </div>
            </div>
          </div>
        `,
        action: 'replace'
      };

      const result2 = await client.applyUpdate(updateHeaderUpdate);
      expect(result2.success).toBe(true);

      // Verify final structure
      const productList = document.getElementById('product-list');
      const countElement = productList?.querySelector('.count');
      expect(countElement?.textContent).toBe('1 items');
      
      const productElement = productList?.querySelector('.product');
      expect(productElement?.getAttribute('data-id')).toBe('1');
      expect(productElement?.querySelector('h3')?.textContent).toBe('Laptop');
    });
  });

  describe('Error handling and edge cases', () => {
    it('should handle malformed HTML gracefully', async () => {
      document.body.innerHTML = '<div id="test">Original</div>';

      const malformedUpdate: RealtimeUpdate = {
        fragment_id: 'test',
        html: '<div><p>Unclosed paragraph',
        action: 'replace'
      };

      const result = await client.applyUpdate(malformedUpdate);
      expect(result.success).toBe(true); // Browser will auto-close tags
      
      const element = document.getElementById('test');
      expect(element?.innerHTML).toContain('Unclosed paragraph');
    });

    it('should handle updates with special characters', async () => {
      document.body.innerHTML = '<div id="test">Original</div>';

      const specialCharsUpdate: RealtimeUpdate = {
        fragment_id: 'test',
        html: '<div id="test">Special: &lt;&gt;&amp;"\'</div>',
        action: 'replace'
      };

      const result = await client.applyUpdate(specialCharsUpdate);
      expect(result.success).toBe(true);
      
      const element = document.getElementById('test');
      expect(element?.innerHTML).toContain('Special: &lt;&gt;&amp;"\'');
    });

    it('should handle empty HTML content appropriately', async () => {
      document.body.innerHTML = '<div id="test">Content</div>';

      const emptyUpdate: RealtimeUpdate = {
        fragment_id: 'test',
        html: '<div id="test"></div>',
        action: 'replace'
      };

      const result = await client.applyUpdate(emptyUpdate);
      expect(result.success).toBe(true);
      
      const element = document.getElementById('test');
      expect(element?.innerHTML).toBe('');
    });
  });
});
