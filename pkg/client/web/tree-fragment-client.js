/**
 * TreeFragmentClient - JavaScript client for LiveTemplate tree-based optimization
 * Handles tree structures from Go template optimization library
 *
 * @version 1.0.0
 * @author LiveTemplate Team
 * @license MIT
 */
class TreeFragmentClient {
  constructor(options = {}) {
    this.options = {
      enableLogging: options.enableLogging || false,
      enableMetrics: options.enableMetrics || true,
      maxCacheSize: options.maxCacheSize || 1000,
      autoCleanupInterval: options.autoCleanupInterval || 300000,
      showErrors: options.showErrors !== false,
      ...options,
    };

    this.cachedStructures = new Map();
    this.renderedElements = new Map();
    this.metrics = {
      totalFragments: 0,
      cacheHits: 0,
      cacheMisses: 0,
      totalProcessingTime: 0,
      bandwidthSaved: 0,
    };

    if (this.options.autoCleanupInterval > 0) {
      setInterval(() => this.cleanup(), this.options.autoCleanupInterval);
    }
  }

  processFragment(fragment, isInitial = false) {
    if (this.options.enableLogging) {
      console.log(
        `[TreeFragmentClient] Processing fragment ${fragment.id}`,
        fragment.data
      );
    }

    const { id, data } = fragment;

    if (isInitial) {
      this.cachedStructures.set(id, this.deepClone(data));
      this.metrics.totalFragments++;
      return this.renderTree(data);
    } else {
      const cachedStructure = this.cachedStructures.get(id);
      if (!cachedStructure) {
        if (this.options.enableLogging) {
          console.warn(
            `[TreeFragmentClient] No cached structure for fragment ${id}, treating as initial`
          );
        }
        this.cachedStructures.set(id, this.deepClone(data));
        this.metrics.totalFragments++;
        return this.renderTree(data);
      }

      const mergedStructure = this.mergeUpdate(cachedStructure, data);
      this.cachedStructures.set(id, this.deepClone(mergedStructure));

      return this.renderTree(mergedStructure);
    }
  }

  renderTree(tree) {
    const startTime = performance.now();

    if (typeof tree === "string") {
      return tree;
    }

    if (Array.isArray(tree)) {
      return tree.map((item) => this.renderTree(item)).join("");
    }

    if (!tree || typeof tree !== "object") {
      return String(tree || "");
    }

    const { s: statics = [] } = tree;
    let result = "";

    for (let i = 0; i < Math.max(statics.length, 10); i++) {
      if (i < statics.length) {
        result += statics[i] || "";
      }

      const dynamicKey = String(i);
      if (tree.hasOwnProperty(dynamicKey)) {
        result += this.renderTree(tree[dynamicKey]);
      }
    }

    const endTime = performance.now();
    if (this.options.enableMetrics) {
      this.metrics.totalProcessingTime += endTime - startTime;
    }

    return result;
  }

  mergeUpdate(cached, update) {
    const merged = this.deepClone(cached);

    for (const key in update) {
      if (key !== "s") {
        merged[key] = this.deepMerge(merged[key], update[key]);
      }
    }

    return merged;
  }

  deepMerge(cached, update) {
    if (update == null) return cached;
    if (cached == null) return update;

    if (
      typeof cached === "object" &&
      typeof update === "object" &&
      !Array.isArray(cached) &&
      !Array.isArray(update)
    ) {
      const merged = this.deepClone(cached);
      for (const key in update) {
        merged[key] = this.deepMerge(merged[key], update[key]);
      }
      return merged;
    }

    return update;
  }

  deepClone(obj) {
    if (obj === null || typeof obj !== "object") {
      return obj;
    }

    if (Array.isArray(obj)) {
      return obj.map((item) => this.deepClone(item));
    }

    const cloned = {};
    for (const key in obj) {
      if (obj.hasOwnProperty(key)) {
        cloned[key] = this.deepClone(obj[key]);
      }
    }

    return cloned;
  }

  updateElement(fragmentId, fragmentData, targetElement, isInitial = false) {
    const html = this.processFragment(
      { id: fragmentId, data: fragmentData },
      isInitial
    );

    if (targetElement) {
      targetElement.innerHTML = html;
      this.renderedElements.set(fragmentId, targetElement);
    }

    return html;
  }

  clearCache(fragmentId) {
    this.cachedStructures.delete(fragmentId);
    this.renderedElements.delete(fragmentId);
  }

  clearAllCaches() {
    this.cachedStructures.clear();
    this.renderedElements.clear();
    this.metrics = {
      totalFragments: 0,
      cacheHits: 0,
      cacheMisses: 0,
      totalProcessingTime: 0,
      bandwidthSaved: 0,
    };
  }

  getCacheStats() {
    return {
      cachedStructures: this.cachedStructures.size,
      renderedElements: this.renderedElements.size,
      fragmentIds: Array.from(this.cachedStructures.keys()),
      metrics: this.metrics,
    };
  }

  getMetrics() {
    const avgProcessingTime =
      this.metrics.totalProcessingTime /
      Math.max(this.metrics.totalFragments, 1);
    const cacheHitRate =
      this.metrics.totalFragments > 0
        ? (this.metrics.cacheHits / this.metrics.totalFragments) * 100
        : 0;

    return {
      totalFragments: this.metrics.totalFragments,
      cacheHits: this.metrics.cacheHits,
      cacheMisses: this.metrics.cacheMisses,
      cacheHitRate: parseFloat(cacheHitRate.toFixed(2)),
      avgProcessingTime: parseFloat(avgProcessingTime.toFixed(2)),
      bandwidthSaved: this.metrics.bandwidthSaved,
    };
  }

  calculateSavings(initialData, updateData) {
    const initialSize = JSON.stringify(initialData).length;
    const updateSize = JSON.stringify(updateData).length;
    const savings = (((initialSize - updateSize) / initialSize) * 100).toFixed(
      1
    );

    return {
      initialSize,
      updateSize,
      savings: parseFloat(savings),
      savedBytes: initialSize - updateSize,
    };
  }

  cleanup() {
    if (this.cachedStructures.size > this.options.maxCacheSize) {
      const toRemove = this.cachedStructures.size - this.options.maxCacheSize;
      const keys = Array.from(this.cachedStructures.keys()).slice(0, toRemove);
      keys.forEach((key) => this.clearCache(key));
    }
  }
}

// Export for both Node.js and browser environments
if (typeof module !== "undefined" && module.exports) {
  module.exports = TreeFragmentClient;
} else if (typeof window !== "undefined") {
  window.TreeFragmentClient = TreeFragmentClient;
}
