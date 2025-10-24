/**
 * Test Conditional Rendering - Debug [object Object] Issue
 *
 * This test debugs the conditional rendering bug where conditional nodes
 * render as [object Object] instead of HTML.
 */

import { LiveTemplateClient } from '../livetemplate-client';

describe('Conditional Rendering Debug Tests', () => {
  let client: LiveTemplateClient;

  beforeEach(() => {
    client = new LiveTemplateClient();
  });

  test('Simple conditional: false -> true transition', () => {
    console.log('\n=== TEST: Simple Conditional False -> True ===\n');

    // Step 1: Initial state with conditional = false
    const initialTree = {
      "0": "",
      "s": [
        "\n<div>\n<h1>Title</h1>\n",
        "\n</div>\n"
      ]
    };

    console.log('Step 1: Apply initial tree (conditional = false)');
    console.log('Initial tree:', JSON.stringify(initialTree, null, 2));

    const result1 = client.applyUpdate(initialTree);

    console.log('Result HTML:', result1.html);
    console.log('Result changed:', result1.changed);
    console.log('Client treeState:', JSON.stringify(client.getTreeState(), null, 2));

    expect(result1.html).toContain('<h1>Title</h1>');
    expect(result1.html).not.toContain('Message is shown');
    expect(result1.html).not.toContain('[object Object]');

    // Step 2: Update to show conditional (false -> true)
    const update = {
      "0": {
        "s": ["\n<p>Message is shown</p>\n"]
      }
    };

    console.log('\nStep 2: Apply update (conditional = true)');
    console.log('Update:', JSON.stringify(update, null, 2));

    const result2 = client.applyUpdate(update);

    console.log('Result HTML:', result2.html);
    console.log('Result changed:', result2.changed);
    console.log('Client treeState after update:', JSON.stringify(client.getTreeState(), null, 2));

    // THIS IS THE BUG: The HTML should contain the message, not [object Object]
    console.log('\n=== CHECKING FOR BUG ===');
    if (result2.html.includes('[object Object]')) {
      console.error('❌ BUG DETECTED: HTML contains [object Object]');
      console.error('Full HTML:', result2.html);
    } else if (result2.html.includes('Message is shown')) {
      console.log('✅ SUCCESS: Conditional rendered correctly');
    } else {
      console.error('❌ UNEXPECTED: Message not found and no [object Object]');
      console.error('Full HTML:', result2.html);
    }

    expect(result2.html).toContain('Message is shown');
    expect(result2.html).not.toContain('[object Object]');
  });

  test('Conditional with search results (mimics Search_Functionality)', () => {
    console.log('\n=== TEST: Search Results Conditional ===\n');

    // Initial state: list with items (send as range structure, not operations)
    const initialTree = {
      "0": {
        "d": [
          {"0": "todo-1", "1": "First Todo"},
          {"0": "todo-2", "1": "Second Todo"}
        ],
        "s": ["<tr data-key=\"", "\"><td>", "</td></tr>"]
      },
      "1": "",
      "s": ["<table><tbody>", "</tbody></table>\n", "\n"]
    };

    console.log('Step 1: Apply initial tree (has items)');
    const result1 = client.applyUpdate(initialTree);
    console.log('Result HTML:', result1.html);

    expect(result1.html).toContain('First Todo');
    expect(result1.html).toContain('Second Todo');

    // Update: empty results with "no results" conditional (ACTUAL SERVER OUTPUT)
    const update = {
      "0": [
        ["r", "todo-1"],
        ["r", "todo-2"]
      ],
      "1": {
        "0": {
          "0": "NonExistent",
          "s": ["\n<small>No todos found matching \"", "\"</small>\n"]
        },
        "1": "",
        "s": ["\n<p>\n", "\n", "\n</p>\n"]
      }
    };

    console.log('\nStep 2: Apply update (empty results + conditional message)');
    console.log('Update:', JSON.stringify(update, null, 2));

    const result2 = client.applyUpdate(update);

    console.log('Result HTML:', result2.html);
    console.log('Client treeState:', JSON.stringify(client.getTreeState(), null, 2));

    // THIS IS THE BUG IN Search_Functionality
    console.log('\n=== CHECKING FOR BUG ===');
    if (result2.html.includes('[object Object]')) {
      console.error('❌ BUG DETECTED: HTML contains [object Object]');
      console.error('Full HTML:', result2.html);
      console.error('This is the Search_Functionality bug!');
    } else if (result2.html.includes('No todos found')) {
      console.log('✅ SUCCESS: Conditional rendered correctly');
    } else {
      console.error('❌ UNEXPECTED: Message not found and no [object Object]');
      console.error('Full HTML:', result2.html);
    }

    expect(result2.html).not.toContain('[object Object]');
    expect(result2.html).toContain('No todos found');
  });

  test('Deep dive: Inspect renderValue with conditional node', () => {
    console.log('\n=== TEST: Deep Dive - renderValue Behavior ===\n');

    // Create a conditional node
    const conditionalNode = {
      "s": ["\n<p>Message is shown</p>\n"]
    };

    console.log('Testing renderValue with conditional node:');
    console.log('Input:', JSON.stringify(conditionalNode, null, 2));

    // Call renderValue directly (via applyUpdate which calls reconstructFromTree which calls renderValue)
    const testTree = {
      "0": conditionalNode,
      "s": ["<div>", "</div>"]
    };

    const result = client.applyUpdate(testTree);
    console.log('Output HTML:', result.html);

    // Check if the conditional node was processed correctly
    const hasObjectString = result.html.includes('[object Object]');
    const hasMessage = result.html.includes('Message is shown');

    console.log('Has [object Object]:', hasObjectString);
    console.log('Has message:', hasMessage);

    if (hasObjectString) {
      console.error('\n❌ PROBLEM IDENTIFIED:');
      console.error('The conditional node {"s": [...]} is being converted to [object Object]');
      console.error('This means renderValue is not detecting it as a tree node');
      console.error('Check the condition at livetemplate-client.ts:1431');
    }

    expect(result.html).not.toContain('[object Object]');
    expect(result.html).toContain('Message is shown');
  });

  test('Verify tree node detection logic', () => {
    console.log('\n=== TEST: Tree Node Detection Logic ===\n');

    const testCases = [
      {
        name: 'Conditional node (only s)',
        value: { "s": ["<p>Test</p>"] },
        shouldBeTree: true,
        expectedHTML: '<p>Test</p>'
      },
      {
        name: 'Tree node with dynamics',
        value: { "0": "dynamic", "s": ["<p>", "</p>"] },
        shouldBeTree: true,
        expectedHTML: '<p>dynamic</p>'
      },
      {
        name: 'Range node',
        value: { "d": [{"0": "item1"}], "s": ["<li>", "</li>"] },
        shouldBeTree: false, // Should be detected as range
        expectedHTML: '<li>item1</li>'
      },
      {
        name: 'Empty object',
        value: {},
        shouldBeTree: false,
        expectedHTML: '[object Object]' // Will fail, but documents behavior
      }
    ];

    testCases.forEach(testCase => {
      console.log(`\nTest case: ${testCase.name}`);
      console.log('Value:', JSON.stringify(testCase.value, null, 2));

      const tree = { "0": testCase.value, "s": ["<div>", "</div>"] };
      const result = client.applyUpdate(tree);

      console.log('Result HTML:', result.html);
      console.log('Contains expected:', result.html.includes(testCase.expectedHTML.replace(/<[^>]*>/g, m => m)));

      // Reset client for next test
      client = new LiveTemplateClient();
    });
  });
});
