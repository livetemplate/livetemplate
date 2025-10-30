import type { TreeNode, UpdateResult } from "../types";
import type { Logger } from "../utils/logger";

interface RangeStateEntry {
  items: any[];
  statics: any[];
}

/**
 * Handles tree state management and HTML reconstruction logic for LiveTemplate.
 */
export class TreeRenderer {
  private treeState: TreeNode = {};
  private rangeState: Record<string, RangeStateEntry> = {};
  private rangeIdKeys: Record<string, string> = {};

  constructor(private readonly logger: Logger) {}

  applyUpdate(update: TreeNode): UpdateResult {
    let changed = false;

    for (const [key, value] of Object.entries(update)) {
      const isDifferentialOps =
        Array.isArray(value) &&
        value.length > 0 &&
        Array.isArray(value[0]) &&
        typeof value[0][0] === "string";

      if (isDifferentialOps) {
        this.treeState[key] = value;
        changed = true;
      } else {
        const oldValue = this.treeState[key];
        const newValue =
          typeof value === "object" && value !== null && !Array.isArray(value)
            ? this.deepMergeTreeNodes(oldValue, value)
            : value;

        if (JSON.stringify(oldValue) !== JSON.stringify(newValue)) {
          this.treeState[key] = newValue;
          changed = true;
        }
      }
    }

    const html = this.reconstructFromTree(this.treeState, "");
    return { html, changed };
  }

  reset(): void {
    this.treeState = {};
    this.rangeState = {};
    this.rangeIdKeys = {};
  }

  getTreeState(): TreeNode {
    return { ...this.treeState };
  }

  getStaticStructure(): string[] | null {
    return this.treeState.s || null;
  }

  private deepMergeTreeNodes(existing: any, update: any): any {
    if (
      typeof update !== "object" ||
      update === null ||
      Array.isArray(update)
    ) {
      return update;
    }

    if (
      typeof existing !== "object" ||
      existing === null ||
      Array.isArray(existing)
    ) {
      return update;
    }

    const merged: any = { ...existing };

    for (const [key, value] of Object.entries(update)) {
      if (
        typeof value === "object" &&
        value !== null &&
        !Array.isArray(value) &&
        typeof merged[key] === "object" &&
        merged[key] !== null &&
        !Array.isArray(merged[key])
      ) {
        merged[key] = this.deepMergeTreeNodes(merged[key], value);
      } else {
        merged[key] = value;
      }
    }

    return merged;
  }

  private reconstructFromTree(node: TreeNode, statePath: string): string {
    if (node.s && Array.isArray(node.s)) {
      let html = "";

      for (let i = 0; i < node.s.length; i++) {
        const staticSegment = node.s[i];
        html += staticSegment;

        if (i < node.s.length - 1) {
          const dynamicKey = i.toString();
          if (node[dynamicKey] !== undefined) {
            const newStatePath = statePath
              ? `${statePath}.${dynamicKey}`
              : dynamicKey;
            html += this.renderValue(
              node[dynamicKey],
              dynamicKey,
              newStatePath
            );
          }
        }
      }

      html = html.replace(/<root>/g, "").replace(/<\/root>/g, "");
      return html;
    }

    return this.renderValue(node, "", statePath);
  }

  private renderValue(
    value: any,
    fieldKey?: string,
    statePath?: string
  ): string {
    if (value === null || value === undefined) {
      return "";
    }

    if (
      typeof value === "string" &&
      value.startsWith("{{") &&
      value.endsWith("}}")
    ) {
      return "";
    }

    if (typeof value === "object" && !Array.isArray(value)) {
      if (
        value.d &&
        Array.isArray(value.d) &&
        value.s &&
        Array.isArray(value.s)
      ) {
        const stateKey = statePath || fieldKey || "";
        if (stateKey) {
          this.rangeState[stateKey] = {
            items: value.d,
            statics: value.s,
          };
          if (
            value.m &&
            typeof value.m === "object" &&
            typeof value.m.idKey === "string"
          ) {
            this.rangeIdKeys[stateKey] = value.m.idKey;
          }
        }
        return this.renderRangeStructure(value, fieldKey, statePath);
      }

      if ("s" in value && Array.isArray((value as TreeNode).s)) {
        return this.reconstructFromTree(value as TreeNode, statePath || "");
      }
    }

    if (Array.isArray(value)) {
      if (
        value.length > 0 &&
        Array.isArray(value[0]) &&
        typeof value[0][0] === "string"
      ) {
        return this.applyDifferentialOperations(value, statePath);
      }

      return value
        .map((item, idx) => {
          const itemKey = idx.toString();
          const itemStatePath = statePath ? `${statePath}.${itemKey}` : itemKey;
          if (typeof item === "object" && item && (item as TreeNode).s) {
            return this.reconstructFromTree(item as TreeNode, itemStatePath);
          }
          return this.renderValue(item, itemKey, itemStatePath);
        })
        .join("");
    }

    if (typeof value === "object") {
      this.logger.error(
        "Object value reached string conversion; this will render as [object Object]."
      );

      if (this.logger.isDebugEnabled()) {
        this.logger.debug("Value diagnostics:", {
          valueType: typeof value,
          isArray: Array.isArray(value),
          keys: Object.keys(value as Record<string, unknown>),
          hasStatics: Boolean((value as any).s),
          hasDynamics: Boolean((value as any).d),
          value,
        });
      }
    }

    return String(value);
  }

  private renderRangeStructure(
    rangeNode: any,
    fieldKey?: string,
    statePath?: string
  ): string {
    const { d: dynamics, s: statics } = rangeNode;

    if (!dynamics || !Array.isArray(dynamics)) {
      return "";
    }

    if (dynamics.length === 0) {
      if (rangeNode["else"]) {
        const elseKey = "else";
        const elseStatePath = statePath ? `${statePath}.else` : "else";
        return this.renderValue(rangeNode["else"], elseKey, elseStatePath);
      }
      return "";
    }

    if (statics && Array.isArray(statics)) {
      return dynamics
        .map((item: any, itemIdx: number) => {
          let html = "";

          for (let i = 0; i < statics.length; i++) {
            html += statics[i];

            if (i < statics.length - 1) {
              const localKey = i.toString();
              if (item[localKey] !== undefined) {
                const itemStatePath = statePath
                  ? `${statePath}.${itemIdx}.${localKey}`
                  : `${itemIdx}.${localKey}`;
                html += this.renderValue(
                  item[localKey],
                  localKey,
                  itemStatePath
                );
              }
            }
          }

          return html;
        })
        .join("");
    }

    return dynamics
      .map((item: any, idx: number) => {
        const itemKey = idx.toString();
        const itemStatePath = statePath ? `${statePath}.${itemKey}` : itemKey;
        return this.renderValue(item, itemKey, itemStatePath);
      })
      .join("");
  }

  private applyDifferentialOperations(
    operations: any[],
    statePath?: string
  ): string {
    if (!statePath || !this.rangeState[statePath]) {
      return "";
    }

    const rangeData = this.rangeState[statePath];
    const currentItems = [...rangeData.items];
    const statics = rangeData.statics;

    for (const operation of operations) {
      if (!Array.isArray(operation) || operation.length < 2) {
        continue;
      }

      const opType = operation[0];

      switch (opType) {
        case "r": {
          const removeIndex = this.findItemIndexByKey(
            currentItems,
            operation[1],
            statics,
            statePath
          );
          if (removeIndex >= 0) {
            currentItems.splice(removeIndex, 1);
          }
          break;
        }
        case "u": {
          const updateIndex = this.findItemIndexByKey(
            currentItems,
            operation[1],
            statics,
            statePath
          );
          const changes = operation[2];
          if (updateIndex >= 0 && changes) {
            currentItems[updateIndex] = {
              ...currentItems[updateIndex],
              ...changes,
            };
          }
          break;
        }
        case "a": {
          this.addItemsToRange(
            currentItems,
            operation[1],
            operation[2],
            rangeData,
            false
          );
          if (
            operation[3] &&
            typeof operation[3] === "object" &&
            operation[3].idKey
          ) {
            this.rangeIdKeys[statePath || ""] = operation[3].idKey;
          }
          break;
        }
        case "p": {
          this.addItemsToRange(
            currentItems,
            operation[1],
            operation[2],
            rangeData,
            true
          );
          break;
        }
        case "i": {
          const targetIndex = this.findItemIndexByKey(
            currentItems,
            operation[1],
            statics,
            statePath
          );
          if (targetIndex >= 0) {
            const itemsToInsert = Array.isArray(operation[2])
              ? operation[2]
              : [operation[2]];
            currentItems.splice(targetIndex + 1, 0, ...itemsToInsert);
          }
          break;
        }
        case "o": {
          const newOrder = operation[1] as string[];
          const reorderedItems: any[] = [];
          const itemsByKey = new Map<string, any>();

          for (const item of currentItems) {
            const itemKey = this.getItemKey(item, statics, statePath);
            if (itemKey) {
              itemsByKey.set(itemKey, item);
            }
          }

          for (const orderedKey of newOrder) {
            const item = itemsByKey.get(orderedKey);
            if (item) {
              reorderedItems.push(item);
            }
          }

          currentItems.length = 0;
          currentItems.push(...reorderedItems);
          break;
        }
        default:
          break;
      }
    }

    this.rangeState[statePath] = {
      items: currentItems,
      statics: rangeData.statics,
    };

    this.treeState[statePath] = {
      d: currentItems,
      s: rangeData.statics,
    };

    const rangeStructure = this.getCurrentRangeStructure(statePath);
    if (rangeStructure && rangeStructure.s) {
      return this.renderItemsWithStatics(currentItems, rangeStructure.s);
    }

    return currentItems.map((item) => this.renderValue(item)).join("");
  }

  private getCurrentRangeStructure(stateKey: string): any {
    if (this.rangeState[stateKey]) {
      return {
        d: this.rangeState[stateKey].items,
        s: this.rangeState[stateKey].statics,
      };
    }

    const fieldValue = this.treeState[stateKey];
    if (
      fieldValue &&
      typeof fieldValue === "object" &&
      (fieldValue as TreeNode).s
    ) {
      return fieldValue;
    }

    return null;
  }

  private renderItemsWithStatics(items: any[], statics: string[]): string {
    const result = items
      .map((item: any) => {
        let html = "";

        for (let i = 0; i < statics.length; i++) {
          html += statics[i];

          if (i < statics.length - 1) {
            const fieldKey = i.toString();
            if (item[fieldKey] !== undefined) {
              html += this.renderValue(item[fieldKey]);
            }
          }
        }

        return html;
      })
      .join("");

    if (this.logger.isDebugEnabled()) {
      this.logger.debug("[renderItemsWithStatics] statics:", statics);
      this.logger.debug("[renderItemsWithStatics] items count:", items.length);
      this.logger.debug(
        "[renderItemsWithStatics] result snippet:",
        result.substring(0, 200)
      );
    }

    return result;
  }

  private addItemsToRange(
    currentItems: any[],
    items: any,
    statics: any[] | undefined,
    rangeData: RangeStateEntry,
    prepend: boolean
  ): void {
    if (statics) {
      rangeData.statics = statics;
    }

    if (!items) return;

    const itemsArray = Array.isArray(items) ? items : [items];
    if (prepend) {
      currentItems.unshift(...itemsArray);
    } else {
      currentItems.push(...itemsArray);
    }
  }

  private getItemKey(
    item: any,
    statics: any[],
    statePath?: string
  ): string | null {
    if (!statePath || !this.rangeIdKeys[statePath]) {
      return null;
    }

    const keyPosStr = this.rangeIdKeys[statePath];
    return item[keyPosStr] || null;
  }

  private findItemIndexByKey(
    items: any[],
    key: string,
    statics: any[],
    statePath?: string
  ): number {
    return items.findIndex(
      (item: any) => this.getItemKey(item, statics, statePath) === key
    );
  }
}
