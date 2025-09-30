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
  private rangeState: { [fieldKey: string]: any[] } = {}; // Track range items by field key
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
            html += this.renderValue(node[dynamicKey], dynamicKey);
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
  private renderValue(value: any, fieldKey?: string): string {
    if (value === null || value === undefined) {
      return '';
    }

    // Skip template control expressions
    if (typeof value === 'string' && value.startsWith('{{') && value.endsWith('}}')) {
      return ''; // Don't render template expressions
    }

    // Handle range structures with 'd' (dynamics) and 's' (statics) arrays
    if (typeof value === 'object' && !Array.isArray(value)) {
      // Check if this is a range structure with 'd' and 's'
      if (value.d && Array.isArray(value.d) && value.s && Array.isArray(value.s)) {
        // Store the range items in our state for differential operations
        if (fieldKey) {
          this.rangeState[fieldKey] = value.d;
        }
        return this.renderRangeStructure(value);
      }
      // Regular nested tree structure
      if (value.s) {
        return this.reconstructFromTree(value);
      }
    }

    // Handle differential operations array
    if (Array.isArray(value)) {
      // Check if this is a differential operations array
      if (value.length > 0 && Array.isArray(value[0]) && typeof value[0][0] === 'string') {
        return this.applyDifferentialOperations(value, fieldKey);
      }

      // Regular array (from range iteration)
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
   * Render a range structure with 'd' (dynamics) and 's' (statics) arrays
   */
  private renderRangeStructure(rangeNode: any): string {
    const { d: dynamics, s: statics } = rangeNode;

    if (!dynamics || !Array.isArray(dynamics)) {
      return '';
    }

    // For empty ranges
    if (dynamics.length === 0) {
      // Check if there's alternative content for empty ranges
      if (rangeNode['else']) {
        return this.renderValue(rangeNode['else']);
      }
      return '';
    }

    // If we have statics, use them as the template for each item
    if (statics && Array.isArray(statics)) {
      return dynamics.map((item: any) => {
        let html = '';

        for (let i = 0; i < statics.length; i++) {
          html += statics[i];

          // Add dynamic value if not at the last static segment
          if (i < statics.length - 1) {
            const fieldKey = i.toString();
            if (item[fieldKey] !== undefined) {
              html += this.renderValue(item[fieldKey]);
            }
          }
        }

        return html;
      }).join('');
    }

    // Fallback: render items as-is if no statics template
    return dynamics.map(item => this.renderValue(item)).join('');
  }

  /**
   * Apply differential operations to existing range items
   * Operations: ["r", key] for remove, ["u", key, changes] for update, ["a", key, data] for add
   */
  private applyDifferentialOperations(operations: any[], fieldKey?: string): string {
    if (!fieldKey || !this.rangeState[fieldKey]) {
      // If we don't have previous range state, we can't apply differential operations
      // This happens on the first load - just return empty for now
      return '';
    }

    const currentItems = [...this.rangeState[fieldKey]]; // Clone current items

    // Apply each operation
    for (const operation of operations) {
      if (!Array.isArray(operation) || operation.length < 2) {
        continue;
      }

      const [opType, key, data] = operation;
      const itemIndex = currentItems.findIndex((item: any) => item['0'] === key); // Field '0' contains the key

      switch (opType) {
        case 'r': // Remove
          if (itemIndex >= 0) {
            currentItems.splice(itemIndex, 1);
          }
          break;

        case 'u': // Update
          if (itemIndex >= 0 && data) {
            // Merge the changes into the existing item
            currentItems[itemIndex] = { ...currentItems[itemIndex], ...data };
          }
          break;

        case 'a': // Add (append) - support both single item and array
          if (data) {
            if (Array.isArray(data)) {
              currentItems.push(...data);
            } else {
              currentItems.push(data);
            }
          }
          break;

        case 'i': // Insert with position - support both single item and array
          const [, targetKey, position, insertData] = operation;
          if (insertData) {
            const itemsToInsert = Array.isArray(insertData) ? insertData : [insertData];

            if (targetKey === null) {
              if (position === "start") {
                currentItems.unshift(...itemsToInsert);
              } else { // "end"
                currentItems.push(...itemsToInsert);
              }
            } else {
              const targetIndex = currentItems.findIndex(item => item['0'] === targetKey);
              if (targetIndex >= 0) {
                const insertIndex = position === "before" ? targetIndex : targetIndex + 1;
                currentItems.splice(insertIndex, 0, ...itemsToInsert);
              }
            }
          }
          break;

        case 'o': // Order (reordering)
          // key contains the new order array
          const newOrder = key as string[];
          const reorderedItems: any[] = [];

          // Build a map of current items by key for efficient lookup
          const itemsByKey = new Map();
          for (const item of currentItems) {
            if (item['0']) {
              itemsByKey.set(item['0'], item);
            }
          }

          // Reorder items according to the new key order
          for (const orderedKey of newOrder) {
            const item = itemsByKey.get(orderedKey);
            if (item) {
              reorderedItems.push(item);
            }
          }

          // Replace currentItems with reordered items
          currentItems.length = 0;
          currentItems.push(...reorderedItems);
          break;
      }
    }

    // Update our range state
    this.rangeState[fieldKey] = currentItems;

    // Render using the current range structure template
    const rangeStructure = this.getCurrentRangeStructure(fieldKey);
    if (rangeStructure && rangeStructure.s) {
      return this.renderItemsWithStatics(currentItems, rangeStructure.s);
    }

    return currentItems.map(item => this.renderValue(item)).join('');
  }

  /**
   * Get the current range structure for a field
   */
  private getCurrentRangeStructure(fieldKey: string): any {
    const fieldValue = this.treeState[fieldKey];
    if (fieldValue && typeof fieldValue === 'object' && fieldValue.s) {
      return fieldValue;
    }
    return null;
  }

  /**
   * Render items using static template
   */
  private renderItemsWithStatics(items: any[], statics: string[]): string {
    return items.map((item: any) => {
      let html = '';

      for (let i = 0; i < statics.length; i++) {
        html += statics[i];

        // Add dynamic value if not at the last static segment
        if (i < statics.length - 1) {
          const fieldKey = i.toString();
          if (item[fieldKey] !== undefined) {
            html += this.renderValue(item[fieldKey]);
          }
        }
      }

      return html;
    }).join('');
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
   * @param element - DOM element containing the LiveTemplate content (the wrapper div)
   * @param update - Tree update object from LiveTemplate server
   */
  updateDOM(element: Element, update: TreeNode): void {
    // Apply update to internal state and get reconstructed HTML
    const result = this.applyUpdate(update);

    if (!result.changed && !update.s) {
      // No changes detected and no statics in update, skip morphdom
      return;
    }

    // Create temporary container with the reconstructed inner content
    const tempContainer = document.createElement('div');
    tempContainer.innerHTML = result.html;

    // Use morphdom to efficiently update only the children of the wrapper element
    // This preserves the wrapper div itself (with data-lvt-id) and only updates its contents
    morphdom(element, tempContainer, {
      childrenOnly: true,  // Only update children, preserve the wrapper element itself
      onBeforeElUpdated: (fromEl, toEl) => {
        // Allow all updates
        return true;
      }
    });
  }

  /**
   * Reset client state (useful for testing)
   */
  reset(): void {
    this.treeState = {};
    this.rangeState = {};
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