/**
 * JavaScript Integration Tester for LiveTemplate Tree-Based Optimization
 * 
 * Comprehensive test suite for validating JavaScript client functionality
 * with real Go library outputs and tree-based optimization patterns.
 * 
 * @version 1.0.0
 */
const fs = require('fs');
const path = require('path');

// Load the TreeFragmentClient from the new location
const TreeFragmentClient = require('../../../pkg/client/web/tree-fragment-client.js');

class JavaScriptIntegrationTester {
    constructor(options = {}) {
        this.client = new TreeFragmentClient({
            enableLogging: options.enableLogging || false,
            enableMetrics: true,
            ...options
        });
        
        this.results = [];
        this.totalTests = 0;
        this.passedTests = 0;
        this.startTime = Date.now();
        
        // Test configuration
        this.config = {
            generateReport: options.generateReport !== false,
            outputDir: options.outputDir || './test-results',
            ...options
        };
    }

    /**
     * Enhanced test data with more complex scenarios
     */
    getTestData() {
        return {
            simpleField: {
                template: '<p>Hello {{.Name}}!</p>',
                description: 'Single field text replacement',
                firstData: { "Name": "World" },
                firstResult: {"0":"World","s":["<p>Hello ","!</p>"]},
                updateData: { "Name": "Universe" },  
                updateResult: {"0":"Universe"},
                expectedHtml: "<p>Hello World!</p>",
                expectedUpdateHtml: "<p>Hello Universe!</p>",
                expectedSavingsMin: 50
            },
            multipleFields: {
                template: '<div>{{.Name}} has {{.Score}} points</div>',
                description: 'Multiple field updates in single template',
                firstData: { "Name": "Alice", "Score": 100 },
                firstResult: {"0":"Alice","1":"100","s":["<div>"," has "," points</div>"]},
                updateData: { "Name": "Bob", "Score": 250 },
                updateResult: {"0":"Bob","1":"250"},
                expectedHtml: "<div>Alice has 100 points</div>",
                expectedUpdateHtml: "<div>Bob has 250 points</div>",
                expectedSavingsMin: 40
            },
            conditionalTrue: {
                template: '<div>{{if .Show}}Welcome {{.Name}}!{{else}}Please log in{{end}}</div>',
                description: 'Conditional branch with nested dynamics',
                firstData: { "Show": true, "Name": "John" },
                firstResult: {"0":{"0":"John","s":["Welcome ","!"]},"s":["<div>","</div>"]},
                updateData: { "Show": true, "Name": "Jane" },
                updateResult: {"0":{"0":"Jane"}},
                expectedHtml: "<div>Welcome John!</div>",
                expectedUpdateHtml: "<div>Welcome Jane!</div>",
                expectedSavingsMin: 60
            },
            conditionalFalse: {
                template: '<div>{{if .Show}}Welcome {{.Name}}!{{else}}Please log in{{end}}</div>',
                description: 'Conditional branch with static content',
                firstData: { "Show": false, "Name": "John" },
                firstResult: {"0":{"s":["Please log in"]},"s":["<div>","</div>"]},
                updateData: { "Show": false, "Name": "Jane" },
                updateResult: {"0":{"s":["Please log in"]}},
                expectedHtml: "<div>Please log in</div>",
                expectedUpdateHtml: "<div>Please log in</div>",
                expectedSavingsMin: 20
            },
            rangeItems: {
                template: '<ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul>',
                description: 'Range loop with item addition',
                firstData: { "Items": ["Apple", "Banana"] },
                firstResult: {"0":[{"0":"Apple","s":["<li>","</li>"]},{"0":"Banana","s":["<li>","</li>"]}],"s":["<ul>","</ul>"]},
                updateData: { "Items": ["Apple", "Banana", "Cherry"] },
                updateResult: {"0":[{"0":"Apple","s":["<li>","</li>"]},{"0":"Banana","s":["<li>","</li>"]},{"0":"Cherry","s":["<li>","</li>"]}]},
                expectedHtml: "<ul><li>Apple</li><li>Banana</li></ul>",
                expectedUpdateHtml: "<ul><li>Apple</li><li>Banana</li><li>Cherry</li></ul>",
                expectedSavingsMin: -50 // Negative expected for additions
            },
            emptyRange: {
                template: '<ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul>',
                description: 'Range loop empty to populated transition',
                firstData: { "Items": [] },
                firstResult: {"0":{"s":[""]},"s":["<ul>","</ul>"]},
                updateData: { "Items": ["Apple"] },
                updateResult: {"0":{"0":"Apple","s":["<li>","</li>"]}},
                expectedHtml: "<ul></ul>",
                expectedUpdateHtml: "<ul><li>Apple</li></ul>",
                expectedSavingsMin: -20 // Negative expected for population
            },
            nestedStructures: {
                template: '<div>{{.User.Profile.Name}} - {{.User.Profile.Role}}</div>',
                description: 'Deeply nested data structures',
                firstData: { "User": { "Profile": { "Name": "Admin", "Role": "Administrator" } } },
                firstResult: {"0":"Admin","1":"Administrator","s":["<div>"," - ","</div>"]},
                updateData: { "User": { "Profile": { "Name": "User", "Role": "Member" } } },
                updateResult: {"0":"User","1":"Member"},
                expectedHtml: "<div>Admin - Administrator</div>",
                expectedUpdateHtml: "<div>User - Member</div>",
                expectedSavingsMin: 50
            },
            complexNesting: {
                template: '<div>{{if .User.Active}}{{.User.Name}} ({{.User.Level}}){{else}}Inactive{{end}}</div>',
                description: 'Complex nested conditionals with multiple fields',
                firstData: { "User": { "Active": true, "Name": "John", "Level": "Gold" } },
                firstResult: {"0":{"0":"John","1":"Gold","s":[""," (",")"]},"s":["<div>","</div>"]},
                updateData: { "User": { "Active": true, "Name": "Jane", "Level": "Platinum" } },
                updateResult: {"0":{"0":"Jane","1":"Platinum"}},
                expectedHtml: "<div>John (Gold)</div>",
                expectedUpdateHtml: "<div>Jane (Platinum)</div>",
                expectedSavingsMin: 40
            }
        };
    }

    /**
     * Run individual test with comprehensive validation
     */
    runTest(testName, testCase) {
        console.log(`\n=== Running Test: ${testName} ===`);
        console.log(`Description: ${testCase.description}`);
        
        const startTime = Date.now();
        
        try {
            // Test 1: Initial render
            const initialFragment = { id: testName, data: testCase.firstResult };
            const initialHtml = this.client.processFragment(initialFragment, true);
            
            console.log(`Template: ${testCase.template}`);
            console.log(`Expected: ${testCase.expectedHtml}`);
            console.log(`Actual:   ${initialHtml}`);
            const initialMatch = initialHtml === testCase.expectedHtml;
            console.log(`Match:    ${initialMatch ? '✓' : '✗'}`);

            // Test 2: Incremental update 
            const updateFragment = { id: testName, data: testCase.updateResult };
            const updateHtml = this.client.processFragment(updateFragment, false);
            
            console.log(`Update Expected: ${testCase.expectedUpdateHtml}`);
            console.log(`Update Actual:   ${updateHtml}`);  
            const updateMatch = updateHtml === testCase.expectedUpdateHtml;
            console.log(`Update Match:    ${updateMatch ? '✓' : '✗'}`);

            // Test 3: Bandwidth calculations
            const savings = this.client.calculateSavings(testCase.firstResult, testCase.updateResult);
            console.log(`Bandwidth Savings: ${savings.savingsFormatted}`);

            // Test 4: Cache functionality
            const cacheStats = this.client.getCacheStats();
            const hasCachedStructure = cacheStats.fragmentIds.includes(testName);
            console.log(`Cache Working: ${hasCachedStructure ? 'Yes' : 'No'}`);

            // Test 5: Performance metrics
            const processingTime = Date.now() - startTime;
            console.log(`Processing Time: ${processingTime}ms`);

            // Comprehensive validation
            const passed = this.validateTest(testCase, initialHtml, updateHtml, savings, processingTime);
            
            const result = {
                name: testName,
                description: testCase.description,
                template: testCase.template,
                initialMatch,
                updateMatch,
                savings,
                hasCachedStructure,
                processingTime,
                passed,
                timestamp: new Date().toISOString()
            };

            this.results.push(result);
            this.totalTests++;
            
            if (passed) {
                this.passedTests++;
                console.log(`✓ ${testName} PASSED`);
            } else {
                console.log(`✗ ${testName} FAILED`);
            }

            return result;

        } catch (error) {
            console.error(`✗ ${testName} ERROR: ${error.message}`);
            const errorResult = {
                name: testName,
                description: testCase.description,
                error: error.message,
                stack: error.stack,
                passed: false,
                timestamp: new Date().toISOString()
            };
            this.results.push(errorResult);
            this.totalTests++;
            return errorResult;
        }
    }

    /**
     * Enhanced test validation with flexible criteria
     */
    validateTest(testCase, initialHtml, updateHtml, savings, processingTime) {
        // Basic validation
        if (!initialHtml || initialHtml.trim() === '') {
            console.log('  ✗ Initial HTML is empty');
            return false;
        }
        if (!updateHtml || updateHtml.trim() === '') {
            console.log('  ✗ Update HTML is empty');
            return false;
        }
        
        // HTML content validation
        if (initialHtml !== testCase.expectedHtml) {
            console.log('  ✗ Initial HTML mismatch');
            return false;
        }
        if (updateHtml !== testCase.expectedUpdateHtml) {
            console.log('  ✗ Update HTML mismatch');
            return false;
        }
        
        // Performance validation
        if (processingTime > 100) {
            console.log(`  ⚠️  Processing time high: ${processingTime}ms`);
        }
        
        // Bandwidth validation with flexible thresholds
        const expectedMinSavings = testCase.expectedSavingsMin || -100;
        if (savings.savings < expectedMinSavings) {
            console.log(`  ✗ Bandwidth savings below threshold: ${savings.savings}% < ${expectedMinSavings}%`);
            return false;
        }

        // Success logging
        console.log('  ✓ All validation checks passed');
        return true;
    }

    /**
     * Run all tests with comprehensive reporting
     */
    runAllTests() {
        console.log('🚀 Starting JavaScript Integration Tests...\n');
        console.log('Testing TreeFragmentClient with real Go library outputs\n');
        console.log('Tree-based optimization with 92%+ bandwidth savings target\n');
        
        const testData = this.getTestData();
        
        Object.keys(testData).forEach(testName => {
            this.runTest(testName, testData[testName]);
        });

        this.runEdgeCaseTests();
        this.runPerformanceTests();
        this.displaySummary();
        
        if (this.config.generateReport) {
            this.generateReport();
        }
        
        return this.passedTests === this.totalTests;
    }

    /**
     * Enhanced edge case testing
     */
    runEdgeCaseTests() {
        console.log('\n🔍 Testing Edge Cases...\n');
        
        const edgeCases = [
            {
                name: 'Empty Fragment',
                test: () => {
                    const result = this.client.processFragment({ id: 'empty', data: {} }, true);
                    return result === '';
                }
            },
            {
                name: 'Null Data',
                test: () => {
                    const result = this.client.processFragment({ id: 'null', data: null }, true);
                    return result === '';
                }
            },
            {
                name: 'Large Data',
                test: () => {
                    const largeData = {
                        "s": ["<div>", "</div>"],
                        "0": "x".repeat(10000)
                    };
                    const result = this.client.processFragment({ id: 'large', data: largeData }, true);
                    return result.includes('xxx') && result.length > 9000;
                }
            },
            {
                name: 'Cache Clearing',
                test: () => {
                    this.client.processFragment({ id: 'clear-test', data: { "s": ["test"] } }, true);
                    const cleared = this.client.clearCache('clear-test');
                    const stats = this.client.getCacheStats();
                    return cleared && !stats.fragmentIds.includes('clear-test');
                }
            },
            {
                name: 'Deep Nesting',
                test: () => {
                    const deepData = {
                        "s": ["<div>", "</div>"],
                        "0": {
                            "s": ["<span>", "</span>"],
                            "0": {
                                "s": ["<em>", "</em>"],
                                "0": "Deep content"
                            }
                        }
                    };
                    const result = this.client.processFragment({ id: 'deep', data: deepData }, true);
                    return result === "<div><span><em>Deep content</em></span></div>";
                }
            }
        ];
        
        edgeCases.forEach(({ name, test }) => {
            try {
                const passed = test();
                console.log(`${name}: ${passed ? '✓' : '✗'}`);
                this.totalTests++;
                if (passed) this.passedTests++;
            } catch (error) {
                console.log(`${name}: ✗ (${error.message})`);
                this.totalTests++;
            }
        });
    }

    /**
     * Performance testing suite
     */
    runPerformanceTests() {
        console.log('\n⚡ Performance Testing...\n');
        
        const iterations = 1000;
        const testData = { "s": ["<div>", "</div>"], "0": "test content" };
        
        // Warm up
        for (let i = 0; i < 10; i++) {
            this.client.processFragment({ id: 'warmup', data: testData }, i === 0);
        }
        
        // Performance test
        const startTime = Date.now();
        for (let i = 0; i < iterations; i++) {
            this.client.processFragment({ id: 'perf-test', data: { ...testData, "0": `test ${i}` } }, false);
        }
        const endTime = Date.now();
        
        const totalTime = endTime - startTime;
        const avgTime = totalTime / iterations;
        
        console.log(`Performance Test Results:`);
        console.log(`  Iterations: ${iterations}`);
        console.log(`  Total Time: ${totalTime}ms`);
        console.log(`  Average Time: ${avgTime.toFixed(2)}ms per fragment`);
        console.log(`  Throughput: ${(iterations / (totalTime / 1000)).toFixed(0)} fragments/sec`);
        
        const performancePassed = avgTime < 1; // Less than 1ms average
        console.log(`  Performance: ${performancePassed ? '✓' : '✗'} (${avgTime < 1 ? 'Good' : 'Needs improvement'})`);
        
        this.totalTests++;
        if (performancePassed) this.passedTests++;
    }

    /**
     * Enhanced summary display
     */
    displaySummary() {
        const totalTime = Date.now() - this.startTime;
        
        console.log('\n' + '='.repeat(60));
        console.log('📊 COMPREHENSIVE TEST SUMMARY');
        console.log('='.repeat(60));
        console.log(`Total Tests: ${this.totalTests}`);
        console.log(`Passed: ${this.passedTests}`);
        console.log(`Failed: ${this.totalTests - this.passedTests}`);
        console.log(`Success Rate: ${((this.passedTests / this.totalTests) * 100).toFixed(1)}%`);
        console.log(`Total Test Time: ${totalTime}ms`);

        // Client metrics
        const metrics = this.client.getMetrics();
        console.log(`\n📈 Client Metrics:`);
        console.log(`  Cache Hit Rate: ${metrics.cacheHitRate}%`);
        console.log(`  Average Processing Time: ${metrics.averageProcessingTime}ms`);
        console.log(`  Total Bandwidth Saved: ${metrics.bandwidthSaved} bytes`);
        console.log(`  Error Rate: ${metrics.errorRate}%`);

        // Cache statistics
        const cacheStats = this.client.getCacheStats();
        console.log(`\n🗄️ Cache Statistics:`);
        console.log(`  Cached Structures: ${cacheStats.cachedStructures}`);
        console.log(`  Cache Usage: ${cacheStats.cacheUsagePercent}%`);

        // Calculate average bandwidth savings
        const validResults = this.results.filter(r => !r.error && r.savings);
        if (validResults.length > 0) {
            const avgSavings = validResults.reduce((sum, r) => sum + r.savings.savings, 0) / validResults.length;
            console.log(`\n💾 Bandwidth Analysis:`);
            console.log(`  Average Bandwidth Savings: ${avgSavings.toFixed(1)}%`);
            console.log(`  Tree-based Optimization Target: 92%+ (for simple templates)`);
            
            // Show savings by test type
            validResults.forEach(result => {
                console.log(`    ${result.name}: ${result.savings.savingsFormatted}`);
            });
        }

        // List failed tests
        const failed = this.results.filter(r => !r.passed);
        if (failed.length > 0) {
            console.log('\n❌ Failed Tests:');
            failed.forEach(test => {
                console.log(`  - ${test.name}${test.error ? ` (${test.error})` : ''}`);
            });
        }

        console.log('='.repeat(60));
    }

    /**
     * Generate comprehensive test report
     */
    generateReport() {
        const report = {
            summary: {
                timestamp: new Date().toISOString(),
                totalTests: this.totalTests,
                passedTests: this.passedTests,
                failedTests: this.totalTests - this.passedTests,
                successRate: ((this.passedTests / this.totalTests) * 100).toFixed(1),
                testDuration: Date.now() - this.startTime
            },
            clientInfo: this.client.getClientInfo(),
            metrics: this.client.getMetrics(),
            cacheStats: this.client.getCacheStats(),
            results: this.results,
            environment: {
                nodeVersion: process.version,
                platform: process.platform,
                arch: process.arch
            }
        };

        // Ensure output directory exists
        if (!fs.existsSync(this.config.outputDir)) {
            fs.mkdirSync(this.config.outputDir, { recursive: true });
        }

        // Write JSON report
        const reportPath = path.join(this.config.outputDir, 'js-integration-test-report.json');
        fs.writeFileSync(reportPath, JSON.stringify(report, null, 2));
        
        // Write markdown report
        const mdReportPath = path.join(this.config.outputDir, 'js-integration-test-report.md');
        fs.writeFileSync(mdReportPath, this.generateMarkdownReport(report));
        
        console.log(`\n📄 Reports generated:`);
        console.log(`  JSON: ${reportPath}`);
        console.log(`  Markdown: ${mdReportPath}`);
    }

    /**
     * Generate markdown report
     */
    generateMarkdownReport(report) {
        return `# JavaScript Integration Test Report

**Test Date**: ${new Date(report.summary.timestamp).toLocaleString()}  
**Client Version**: ${report.clientInfo.version}  
**Test Environment**: Node.js ${report.environment.nodeVersion}  

## Executive Summary

${report.summary.passedTests === report.summary.totalTests ? '✅' : '❌'} **Tests ${report.summary.successRate}% Passed** (${report.summary.passedTests}/${report.summary.totalTests})  
⚡ **Performance**: ${report.metrics.averageProcessingTime}ms average processing time  
💾 **Efficiency**: ${report.metrics.bandwidthSaved} bytes total bandwidth saved  
🗄️ **Cache**: ${report.metrics.cacheHitRate}% hit rate  

## Test Results

| Test Name | Status | Processing Time | Bandwidth Savings | Description |
|-----------|--------|----------------|-------------------|-------------|
${report.results.map(r => `| ${r.name} | ${r.passed ? '✅ PASS' : '❌ FAIL'} | ${r.processingTime || 'N/A'}ms | ${r.savings ? r.savings.savingsFormatted : 'N/A'} | ${r.description || ''} |`).join('\n')}

## Performance Metrics

- **Total Fragments Processed**: ${report.metrics.totalFragments}
- **Cache Hit Rate**: ${report.metrics.cacheHitRate}%
- **Average Processing Time**: ${report.metrics.averageProcessingTime}ms
- **Total Bandwidth Saved**: ${report.metrics.bandwidthSaved} bytes
- **Error Rate**: ${report.metrics.errorRate}%

## Client Configuration

- **Version**: ${report.clientInfo.version}
- **Features**: ${report.clientInfo.features.join(', ')}
- **Max Cache Size**: ${report.clientInfo.options.maxCacheSize}
- **Auto Cleanup**: ${report.clientInfo.options.autoCleanupInterval}ms

## Conclusion

${report.summary.passedTests === report.summary.totalTests 
    ? 'All tests passed successfully. The JavaScript client is ready for production use.' 
    : `${report.summary.failedTests} test(s) failed. Review the failed tests and address issues before deployment.`}
`;
    }
}

// Export for use in other modules
module.exports = JavaScriptIntegrationTester;

// Run tests if this file is executed directly
if (require.main === module) {
    const tester = new JavaScriptIntegrationTester({
        enableLogging: false,
        generateReport: true,
        outputDir: './test-results'
    });
    
    const success = tester.runAllTests();
    process.exit(success ? 0 : 1);
}