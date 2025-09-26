/**
 * LiveTemplate TypeScript Client
 * 
 * Reconstructs HTML from tree-based updates using cached static structure,
 * following the Phoenix LiveView optimization approach.
 */

import morphdom from 'morphdom';

export interface TreeNode {
  [key: string]: any;
  s?: string[];  // Static HTML segments (sent once, cached client-side)
}

export interface UpdateResult {
  html: string;
  changed: boolean;
  dom?: Element;
}

export class LiveTemplateClient {
  private treeState: TreeNode = {};
  private lvtId: string | null = null;

  /**
   * Apply an update to the current state and reconstruct HTML
   * @param update - Tree update object from LiveTemplate server
   * @returns Reconstructed HTML and whether anything changed
   */
  applyUpdate(update: TreeNode): UpdateResult {
    let changed = false;

    // Merge the update into our tree state
    for (const [key, value] of Object.entries(update)) {
      if (JSON.stringify(this.treeState[key]) !== JSON.stringify(value)) {
        this.treeState[key] = value;
        changed = true;
      }
    }

    // Reconstruct HTML from the complete tree state
    const html = this.reconstructFromTree(this.treeState);
    
    return { html, changed };
  }

  /**
   * Reconstruct HTML from a tree structure
   * This is the core algorithm that matches the Go server implementation
   * Dynamic values are interleaved between static segments: static[0] + dynamic[0] + static[1] + ...
   * Invariant: len(statics) == len(dynamics) + 1
   */
  private reconstructFromTree(node: TreeNode): string {
    // If node has static segments, use them as the template
    if (node.s && Array.isArray(node.s)) {
      let html = '';
      
      // Interleave static segments with dynamic values
      // Pattern: static[0] + dynamic[0] + static[1] + dynamic[1] + ... + static[n]
      for (let i = 0; i < node.s.length; i++) {
        const staticSegment = node.s[i];
        
        // Add static segment
        html += staticSegment;
        
        // After adding the static segment, add the corresponding dynamic value if it exists
        // But only if we're not at the last static segment
        if (i < node.s.length - 1) {
          const dynamicKey = i.toString();
          if (node[dynamicKey] !== undefined) {
            html += this.renderValue(node[dynamicKey]);
          }
        }
      }
      
      // Remove the <root> wrapper that was added for parsing
      html = html.replace(/<root>/g, '').replace(/<\/root>/g, '');
      
      return html;
    }
    
    // If no static segments, just render the values
    return this.renderValue(node);
  }

  /**
   * Render a dynamic value (could be string, nested tree, or array)
   */
  private renderValue(value: any): string {
    if (value === null || value === undefined) {
      return '';
    }
    
    // Skip template control expressions
    if (typeof value === 'string' && value.startsWith('{{') && value.endsWith('}}')) {
      return ''; // Don't render template expressions
    }
    
    // If it's a nested tree structure
    if (typeof value === 'object' && !Array.isArray(value) && value.s) {
      return this.reconstructFromTree(value);
    }
    
    // If it's an array (from range iteration)
    if (Array.isArray(value)) {
      return value.map(item => {
        // Each item should be a tree node with its own static/dynamic structure
        if (typeof item === 'object' && item.s) {
          return this.reconstructFromTree(item);
        }
        return this.renderValue(item);
      }).join('');
    }
    
    // Simple string/number value
    return String(value);
  }

  /**
   * Apply updates to existing HTML using morphdom for efficient DOM updates
   * @param existingHTML - Current full HTML document
   * @param update - Tree update object from LiveTemplate server
   * @returns Updated HTML content
   */
  applyUpdateToHTML(existingHTML: string, update: TreeNode): string {
    // Apply the update to our internal state
    const result = this.applyUpdate(update);
    
    // Extract lvt-id from existing HTML if we don't have it
    if (!this.lvtId) {
      const match = existingHTML.match(/data-lvt-id="([^"]+)"/);
      if (match) {
        this.lvtId = match[1];
      }
    }
    
    // The new tree includes complete HTML structure, so we can reconstruct properly
    const innerContent = result.html;
    
    // Find where to insert the reconstructed content
    const bodyMatch = existingHTML.match(/<body>([\s\S]*?)<\/body>/);
    if (!bodyMatch) {
      return existingHTML;
    }
    
    // Replace the body content with our reconstructed HTML
    // We need to preserve the wrapper div with data-lvt-id
    const wrapperStart = `<div data-lvt-id="${this.lvtId || 'lvt-unknown'}">`;
    const wrapperEnd = '</div>';
    const newBodyContent = wrapperStart + innerContent + wrapperEnd;
    
    return existingHTML.replace(/<body>[\s\S]*?<\/body>/, `<body>${newBodyContent}</body>`);
  }

  /**
   * Update a live DOM element with new tree data
   * @param element - DOM element containing the LiveTemplate content
   * @param update - Tree update object from LiveTemplate server
   */
  updateDOM(element: Element, update: TreeNode): void {
    const currentHTML = element.outerHTML;
    const updatedHTML = this.applyUpdateToHTML(currentHTML, update);
    
    // Create temporary container for updated HTML
    const tempContainer = document.createElement('div');
    tempContainer.innerHTML = updatedHTML;
    const newElement = tempContainer.firstElementChild;
    
    if (newElement) {
      // Use morphdom to efficiently update the DOM
      morphdom(element, newElement, {
        onBeforeElUpdated: (fromEl, toEl) => {
          // Preserve lvt-id attribute
          if (fromEl.hasAttribute('data-lvt-id') && !toEl.hasAttribute('data-lvt-id')) {
            toEl.setAttribute('data-lvt-id', fromEl.getAttribute('data-lvt-id')!);
          }
          return true;
        }
      });
    }
  }

  /**
   * Reset client state (useful for testing)
   */
  reset(): void {
    this.treeState = {};
    this.lvtId = null;
  }

  /**
   * Get current tree state (for debugging)
   */
  getTreeState(): TreeNode {
    return { ...this.treeState };
  }

  /**
   * Get the static structure if available
   */
  getStaticStructure(): string[] | null {
    return this.treeState.s || null;
  }
}

/**
 * Utility function to load and apply updates from JSON files
 */
export async function loadAndApplyUpdate(
  client: LiveTemplateClient, 
  updatePath: string
): Promise<UpdateResult> {
  try {
    // In Node.js environment
    if (typeof require !== 'undefined') {
      const fs = require('fs');
      const updateData = JSON.parse(fs.readFileSync(updatePath, 'utf8'));
      return client.applyUpdate(updateData);
    }
    
    // In browser environment
    const response = await fetch(updatePath);
    const updateData = await response.json();
    return client.applyUpdate(updateData);
  } catch (error) {
    throw new Error(`Failed to load update from ${updatePath}: ${error}`);
  }
}

/**
 * Compare two HTML strings, ignoring whitespace differences
 */
export function compareHTML(expected: string, actual: string): {
  match: boolean;
  differences: string[];
} {
  const differences: string[] = [];
  
  // Normalize whitespace for comparison
  const normalizeHTML = (html: string) => {
    return html
      .replace(/\s+/g, ' ')           // Collapse multiple spaces
      .replace(/>\s+</g, '><')        // Remove spaces between tags
      .trim();
  };
  
  const normalizedExpected = normalizeHTML(expected);
  const normalizedActual = normalizeHTML(actual);
  
  if (normalizedExpected === normalizedActual) {
    return { match: true, differences: [] };
  }
  
  // Find specific differences
  const expectedLines = normalizedExpected.split('\n');
  const actualLines = normalizedActual.split('\n');
  
  const maxLines = Math.max(expectedLines.length, actualLines.length);
  for (let i = 0; i < maxLines; i++) {
    const expectedLine = expectedLines[i] || '';
    const actualLine = actualLines[i] || '';
    
    if (expectedLine !== actualLine) {
      differences.push(`Line ${i + 1}:`);
      differences.push(`  Expected: ${expectedLine}`);
      differences.push(`  Actual:   ${actualLine}`);
    }
  }
  
  return { match: false, differences };
}