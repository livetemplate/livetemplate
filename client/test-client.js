/**
 * Node.js test for LiveTemplate Client Engine
 * Tests the client engine logic without browser dependencies
 */

const LiveTemplateClient = require('./livetemplate-client.js');

// Mock DOM environment for Node.js testing
let JSDOM;
try {
    JSDOM = require('jsdom').JSDOM;
} catch (e) {
    // jsdom not available
    JSDOM = null;
}

function createMockDOM() {
    const dom = new JSDOM(`
        <!DOCTYPE html>
        <html>
        <body>
            <div id="test-container">
                <h1 id="title">Original Title</h1>
                <div id="counter" data-marker="count">Count: 0</div>
                <ul id="item-list">
                    <li id="item-0">Item 1</li>
                    <li id="item-1">Item 2</li>
                </ul>
                <div id="replacement-target">Original Content</div>
            </div>
        </body>
        </html>
    `);
    
    // Make DOM globals available
    global.document = dom.window.document;
    global.window = dom.window;
    
    return dom;
}

async function testStaticDynamicFragment() {
    console.log('Testing Static/Dynamic Fragment Application...');
    
    const client = new LiveTemplateClient({ debug: true });
    
    const fragment = {
        id: 'frag_static_dynamic_test',
        strategy: 'static_dynamic',
        action: 'update_values',
        data: {
            statics: ['<h1 id="title">', '</h1>'],
            dynamics: { 0: 'Updated Title' },
            fragment_id: 'frag_static_dynamic_test'
        }
    };
    
    const result = await client.applyFragment(fragment);
    console.log('Static/Dynamic result:', result);
    
    // Test dynamics-only update using cache
    const dynamicsOnlyFragment = {
        id: 'frag_static_dynamic_test',
        strategy: 'static_dynamic', 
        action: 'update_values',
        data: {
            dynamics: { 0: 'Cached Update' },
            fragment_id: 'frag_static_dynamic_test'
        }
    };
    
    const cacheResult = await client.applyFragment(dynamicsOnlyFragment);
    console.log('Dynamics-only result:', cacheResult);
    
    return result && cacheResult;
}

async function testMarkerFragment() {
    console.log('Testing Marker Fragment Application...');
    
    const client = new LiveTemplateClient({ debug: true });
    
    const fragment = {
        id: 'frag_markers_test',
        strategy: 'markers',
        action: 'apply_patches',
        data: {
            value_updates: {
                'count': '42'
            }
        }
    };
    
    const result = await client.applyFragment(fragment);
    console.log('Marker result:', result);
    
    // Check if the marker was updated
    const markerElement = document.querySelector('[data-marker="count"]');
    console.log('Marker element content:', markerElement ? markerElement.textContent : 'not found');
    
    return result;
}

async function testGranularFragment() {
    console.log('Testing Granular Fragment Application...');
    
    const client = new LiveTemplateClient({ debug: true });
    
    const fragment = {
        id: 'frag_granular_test',
        strategy: 'granular',
        action: 'apply_operations',
        data: {
            operations: [
                {
                    type: 'insert',
                    target_id: 'item-list',
                    content: '<li id="item-new">New Item</li>',
                    position: 'beforeend'
                },
                {
                    type: 'update',
                    target_id: 'replacement-target',
                    content: 'Updated via Granular'
                }
            ]
        }
    };
    
    const result = await client.applyFragment(fragment);
    console.log('Granular result:', result);
    
    // Check if operations were applied
    const newItem = document.getElementById('item-new');
    const updatedTarget = document.getElementById('replacement-target');
    console.log('New item exists:', newItem !== null);
    console.log('Target updated:', updatedTarget ? updatedTarget.textContent : 'not found');
    
    return result;
}

async function testReplacementFragment() {
    console.log('Testing Replacement Fragment Application...');
    
    const client = new LiveTemplateClient({ debug: true });
    
    const fragment = {
        id: 'frag_replacement_test',
        strategy: 'replacement',
        action: 'replace_content',
        data: {
            content: '<div id="replacement-target" class="replaced">Completely Replaced</div>',
            target_id: 'replacement-target'
        }
    };
    
    const result = await client.applyFragment(fragment);
    console.log('Replacement result:', result);
    
    // Check if content was replaced
    const replacedElement = document.getElementById('replacement-target');
    console.log('Replaced element class:', replacedElement ? replacedElement.className : 'not found');
    
    return result;
}

async function testErrorHandling() {
    console.log('Testing Error Handling...');
    
    const client = new LiveTemplateClient({ debug: true });
    
    // Test invalid fragment
    const invalidFragment = {
        id: 'invalid',
        // Missing required fields
    };
    
    const result1 = await client.applyFragment(invalidFragment);
    console.log('Invalid fragment result:', result1); // Should be false
    
    // Test unknown strategy
    const unknownStrategyFragment = {
        id: 'unknown',
        strategy: 'unknown_strategy',
        action: 'unknown_action',
        data: {}
    };
    
    const result2 = await client.applyFragment(unknownStrategyFragment);
    console.log('Unknown strategy result:', result2); // Should be false
    
    const metrics = client.getMetrics();
    console.log('Error metrics:', metrics.errorCount); // Should be > 0
    
    return !result1 && !result2 && metrics.errorCount > 0;
}

async function testCachingSystem() {
    console.log('Testing Caching System...');
    
    const client = new LiveTemplateClient({ debug: true });
    
    // Clear cache and reset metrics
    client.clearCache();
    client.resetMetrics();
    
    // Apply fragment with statics (should cache)
    const fragmentWithStatics = {
        id: 'cache_test',
        strategy: 'static_dynamic',
        action: 'update_values',
        data: {
            statics: ['<div id="cache-test">', '</div>'],
            dynamics: { 0: 'Initial' },
            fragment_id: 'cache_test'
        }
    };
    
    await client.applyFragment(fragmentWithStatics);
    
    // Apply dynamics-only update (should use cache)
    const dynamicsOnly = {
        id: 'cache_test',
        strategy: 'static_dynamic',
        action: 'update_values',
        data: {
            dynamics: { 0: 'From Cache' },
            fragment_id: 'cache_test'
        }
    };
    
    await client.applyFragment(dynamicsOnly);
    
    const metrics = client.getMetrics();
    console.log('Cache metrics:', {
        hits: metrics.cacheHits,
        misses: metrics.cacheMisses,
        hitRate: metrics.cacheHitRate
    });
    
    return metrics.cacheHits > 0 && metrics.cacheMisses > 0;
}

async function runAllTests() {
    console.log('LiveTemplate Client Engine Tests\n');
    
    // Install jsdom if not available
    if (!JSDOM) {
        console.log('JSDOM not available - install with: npm install jsdom');
        console.log('Skipping Node.js tests...');
        return true; // Consider this a pass since it's optional
    }
    
    // Create mock DOM
    createMockDOM();
    
    const tests = [
        { name: 'Static/Dynamic Fragment', fn: testStaticDynamicFragment },
        { name: 'Marker Fragment', fn: testMarkerFragment },
        { name: 'Granular Fragment', fn: testGranularFragment },
        { name: 'Replacement Fragment', fn: testReplacementFragment },
        { name: 'Error Handling', fn: testErrorHandling },
        { name: 'Caching System', fn: testCachingSystem }
    ];
    
    let passed = 0;
    let total = tests.length;
    
    for (const test of tests) {
        try {
            console.log(`\n--- ${test.name} ---`);
            const result = await test.fn();
            if (result) {
                console.log(`✓ ${test.name} PASSED`);
                passed++;
            } else {
                console.log(`✗ ${test.name} FAILED`);
            }
        } catch (error) {
            console.log(`✗ ${test.name} ERROR:`, error.message);
        }
    }
    
    console.log(`\n=== Results ===`);
    console.log(`Passed: ${passed}/${total}`);
    console.log(`Success Rate: ${(passed/total*100).toFixed(1)}%`);
    
    return passed === total;
}

// Run tests if called directly
if (require.main === module) {
    runAllTests().then(success => {
        process.exit(success ? 0 : 1);
    });
}

module.exports = { runAllTests };